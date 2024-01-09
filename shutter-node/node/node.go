package node

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/errgroup"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/ethereum-optimism/optimism/shutter-node/config"
	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/p2p"
	"github.com/ethereum-optimism/optimism/shutter-node/rollup"
	service "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type ShutterNode struct {
	log        log.Logger
	appVersion string
	metrics    *metrics.Metrics

	metricsSrv *httputil.HTTPServer

	keyHandler *p2p.DecryptionKeyHandler
	keyManager keys.Manager
	syncer     *rollup.Syncer
	db         *database.Database

	p2p    shp2p.Messaging
	errgrp *errgroup.Group

	// some resources cannot be stopped directly, like the p2p gossipsub router (not our design),
	// and depend on this ctx to be closed.
	resourcesCtx   context.Context
	resourcesClose context.CancelFunc

	closed atomic.Bool

	// cancels execution prematurely, e.g. to halt. This may be nil.
	cancel context.CancelCauseFunc
	halted atomic.Bool
}

// New creates a new ShutterNode instance.
// The provided ctx argument is for the span of initialization only;
// the node will immediately Stop(ctx) before finishing initialization if the context is canceled during initialization.
func New(ctx context.Context, cfg *config.Config, log log.Logger, appVersion string) (*ShutterNode, error) {
	if err := cfg.Check(); err != nil {
		return nil, err
	}

	// TODO: those are rollup metrics.
	// for now just pass them in to avoid nil-derefs
	// during initialistation.
	// Later we can write or own metrics definitions.
	m := metrics.NewMetrics("shutter")

	n := &ShutterNode{
		metrics:    m,
		log:        log,
		appVersion: appVersion,
		closed:     atomic.Bool{},
		cancel:     cfg.Cancel,
		halted:     atomic.Bool{},
	}
	// not a context leak, gossipsub is closed with a context.
	n.resourcesCtx, n.resourcesClose = context.WithCancel(context.Background())

	err := n.init(ctx, cfg)
	if err != nil {
		log.Error("Error initializing the shutter node", "err", err)
		// ensure we always close the node resources if we fail to initialize the node.
		if closeErr := n.Stop(ctx); closeErr != nil {
			return nil, multierror.Append(err, closeErr)
		}
		return nil, err
	}
	n.log.Info("initialised shutter node", "config", cfg)
	return n, nil
}

func (n *ShutterNode) init(ctx context.Context, cfg *config.Config) error {
	var err error
	n.log.Info("Initializing shutter node", "version", n.appVersion)
	if err := n.initDatabase(cfg); err != nil {
		return fmt.Errorf("failed to init the database: %w", err)
	}

	n.keyManager, err = keys.New(n.db, n.log)
	if err != nil {
		return err
	}
	n.syncer = rollup.NewL2Syncer(cfg.L2Sync.L2NodeAddr, n.log, n.keyManager, n.db)
	if err := n.initP2P(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init the P2P stack: %w", err)
	}
	if err := n.initMetricsServer(cfg); err != nil {
		return fmt.Errorf("failed to init the metrics server: %w", err)
	}
	n.metrics.RecordInfo(n.appVersion)
	n.metrics.RecordUp()
	return nil
}

func (n *ShutterNode) initDatabase(cfg *config.Config) error {
	db := &database.Database{}
	// TODO: make file configurable
	if err := db.Connect("test.db"); err != nil {
		return err
	}
	n.db = db
	return nil
}

func (n *ShutterNode) initMetricsServer(cfg *config.Config) error {
	if !cfg.Metrics.Enabled {
		n.log.Info("metrics disabled")
		return nil
	}
	n.log.Debug("starting metrics server", "addr", cfg.Metrics.ListenAddr, "port", cfg.Metrics.ListenPort)
	// FIXME: start in a service?
	metricsSrv, err := n.metrics.StartServer(cfg.Metrics.ListenAddr, cfg.Metrics.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	n.log.Info("started metrics server", "addr", metricsSrv.Addr())
	n.metricsSrv = metricsSrv
	return nil
}

func (n *ShutterNode) initP2P(ctx context.Context, cfg *config.Config) error {
	n.log.Info("got p2p config", "p2p-config", *cfg.P2P)
	mss, err := shp2p.New(cfg.P2P)
	if err != nil {
		return err
	}
	n.p2p = mss
	n.keyHandler = p2p.NewDecryptionKeyHandler(cfg.InstanceID, n.keyManager, n.log)
	n.p2p.AddMessageHandler(n.keyHandler)
	return nil
}

func (n *ShutterNode) Start(ctx context.Context) error {
	n.errgrp = service.RunBackground(ctx, n.keyManager, n.syncer)
	// XXX: the appCtx didn't seem to work for e.g. libp2p.
	p2perrgrp := service.RunBackground(n.resourcesCtx, n.p2p)
	n.log.Info("Rollup node started")
	go func() {
		err := n.errgrp.Wait()
		n.log.Error("errgroup wait returned", "error", err)
		if err != nil {
			n.cancel(err)
		}
	}()

	go func() {
		err := p2perrgrp.Wait()
		n.log.Error("p2p errgroup wait returned", "error", err)
		if err != nil {
			n.cancel(err)
		}
	}()
	return nil
}

// unixTimeStale returns true if the unix timestamp is before the current time minus the supplied duration.
func unixTimeStale(timestamp uint64, duration time.Duration) bool {
	return time.Unix(int64(timestamp), 0).Before(time.Now().Add(-1 * duration))
}

// Stop stops the node and closes all resources.
// If the provided ctx is expired, the node will accelerate the stop where possible, but still fully close.
func (n *ShutterNode) Stop(ctx context.Context) error {
	if n.closed.Load() {
		return errors.New("node is already closed")
	}

	var result *multierror.Error
	if n.resourcesClose != nil {
		// p2p context
		n.resourcesClose()
	}

	if result == nil { // mark as closed if we successfully fully closed
		n.closed.Store(true)
	}

	if n.halted.Load() {
		// if we had a halt upon initialization, idle for a while, with open metrics, to prevent a rapid restart-loop
		tim := time.NewTimer(time.Minute * 5)
		n.log.Warn("halted, idling to avoid immediate shutdown repeats")
		defer tim.Stop()
		select {
		case <-tim.C:
		case <-ctx.Done():
		}
	}

	// Close metrics and pprof only after we are done idling
	if n.metricsSrv != nil {
		if err := n.metricsSrv.Stop(ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to close metrics server: %w", err))
		}
	}
	// FIXME: how to wait properly for this?
	log.Info("will wait for errgroup")
	err := n.errgrp.Wait()
	if err != nil {
		log.Error("errgroup errorered", "error", err)
	}

	log.Info("shutting down")
	return result.ErrorOrNil()
}

func (n *ShutterNode) Stopped() bool {
	return n.closed.Load()
}

func (n *ShutterNode) HTTPEndpoint() string {
	// if n.server == nil {
	// 	return ""
	// }
	// return fmt.Sprintf("http://%s", n.server.Addr().String())
	return ""
}
