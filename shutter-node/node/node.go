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
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/shutter-node/config"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/p2p"
	shclient "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/optimism/sync"
	service "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type ShutterNode struct {
	log        log.Logger
	appVersion string
	metrics    *metrics.Metrics

	metricsSrv *httputil.HTTPServer

	l2Client   *sources.ShutterL2Client
	keyHandler *p2p.DecryptionKeyHandler
	keyManager keys.Manager

	rpcClient *shclient.ShutterL2Client
	p2p       shp2p.Messaging
	errgrp    *errgroup.Group

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
	n.log.Info("initialised shutter node", "config", cfg)
	if err != nil {
		log.Error("Error initializing the shutter node", "err", err)
		// ensure we always close the node resources if we fail to initialize the node.
		if closeErr := n.Stop(ctx); closeErr != nil {
			return nil, multierror.Append(err, closeErr)
		}
		return nil, err
	}
	return n, nil
}

func (n *ShutterNode) init(ctx context.Context, cfg *config.Config) error {
	n.log.Info("Initializing shutter node", "version", n.appVersion)
	n.keyManager = keys.NewTestManager(n.log)
	if err := n.initShutterL2Client(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init L2 RPC sync: %w", err)
	}
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

func (n *ShutterNode) initShutterL2Client(ctx context.Context, cfg *config.Config) error {
	// FIXME: this does not seem to be the typical JSON-RPC client.
	// rpcSyncClient, rpcCfg, err := cfg.L2Sync.Setup(ctx, n.log, &cfg.Rollup)
	// if err != nil {
	// 	return fmt.Errorf("failed to setup L2 execution-engine RPC client for RPC sync: %w", err)
	// }
	// if rpcSyncClient == nil { // if no RPC client is configured to sync from, then don't add the RPC sync client
	// 	return nil
	// }

	// XXX: instead use a json-rpc client
	jsonRPC, err := rpc.DialContext(ctx, cfg.L2Sync.L2NodeAddr)
	if err != nil {
		return err
	}
	iclient := client.NewInstrumentedClient(jsonRPC, n.metrics)
	shutterClient, err := shclient.NewShutterL2Client(
		ctx,
		shclient.WithClient(iclient),
		shclient.WithLogger(n.log),
	)
	if err != nil {
		return err
	}
	n.rpcClient = shutterClient
	return nil
}

func (n *ShutterNode) initMetricsServer(cfg *config.Config) error {
	if !cfg.Metrics.Enabled {
		n.log.Info("metrics disabled")
		return nil
	}
	n.log.Debug("starting metrics server", "addr", cfg.Metrics.ListenAddr, "port", cfg.Metrics.ListenPort)
	metricsSrv, err := n.metrics.StartServer(cfg.Metrics.ListenAddr, cfg.Metrics.ListenPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	n.log.Info("started metrics server", "addr", metricsSrv.Addr())
	n.metricsSrv = metricsSrv
	return nil
}

func (n *ShutterNode) initP2P(ctx context.Context, cfg *config.Config) error {
	mss, err := shp2p.New(cfg.P2P)
	if err != nil {
		return err
	}
	n.p2p = mss
	// FIXME: remove hardcoded instance ID
	n.keyHandler = p2p.NewDecryptionKeyHandler(442, n.keyManager)
	return nil
}

func (n *ShutterNode) Start(ctx context.Context) error {
	// If the rpc unsafe sync client is enabled, start its event loop
	if n.rpcClient != nil {
		// TODO: start syncing the shutter state and the latest "unsafe" block
		// from the op-geth json RPC l2 client

		// FIXME: comply to the service interface
		// if err := n.rpcClient.Start(ctx); err != nil {
		// 	n.log.Error("Could not start the RPC L2 client", "err", err)
		// 	return err
		// }
		// n.log.Info("Started L2-RPC client service")
	}
	n.errgrp = service.RunBackground(n.resourcesCtx, n.p2p, n.keyManager)
	log.Info("Rollup node started")
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
	if n.rpcClient != nil {
		// FIXME: comply to the service interdface
		// if err := n.rpcClient.Close(); err != nil {
		// 	result = multierror.Append(result, fmt.Errorf("failed to close RPC server: %w", err))
		// }
	}

	// if n.server != nil {
	// 	if err := n.server.Stop(ctx); err != nil {
	// 		result = multierror.Append(result, fmt.Errorf("failed to close RPC server: %w", err))
	// 	}
	// }
	// if n.p2pSigner != nil {
	// 	if err := n.p2pSigner.Close(); err != nil {
	// 		result = multierror.Append(result, fmt.Errorf("failed to close p2p signer: %w", err))
	// 	}
	// }
	if n.resourcesClose != nil {
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
		log.Error("errgroup errorered", err)
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
