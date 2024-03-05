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

	"github.com/ethereum-optimism/optimism/shutter-node/config"
	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/database/writer"
	"github.com/ethereum-optimism/optimism/shutter-node/grpc/v1/server"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/p2p"
	service "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	shp2p "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type ShutterNode struct {
	log        log.Logger
	appVersion string

	keyHandler *p2p.DecryptionKeyHandler
	keyManager keys.Manager
	writer     *writer.DBWriter
	db         *database.Database
	grpc       *server.Server

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

	n := &ShutterNode{
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
	n.writer = writer.NewDBWriter(cfg.L2Sync.L2NodeAddr, n.log, n.db)
	if err := n.initP2P(ctx, cfg); err != nil {
		return fmt.Errorf("failed to init the P2P stack: %w", err)
	}
	if err := n.initGRPCServer(cfg, n.log, n.keyManager.RequestDecryptionKey); err != nil {
		return fmt.Errorf("failed to open grpc server: %w", err)
	}
	return nil
}

func (n *ShutterNode) initDatabase(cfg *config.Config) error {
	db := &database.Database{}
	if err := db.Connect(cfg.Database.FilePath); err != nil {
		return err
	}
	n.db = db
	return nil
}

func (n *ShutterNode) initGRPCServer(
	cfg *config.Config,
	log log.Logger,
	dkFn keys.RequestDecryptionKey,
) error {
	grpc, err := server.NewServer(
		dkFn,
		server.WithLogger(log),
		server.WithListenAddress(cfg.GRPC.ListenNetwork, cfg.GRPC.ListenAddress),
	)
	if err != nil {
		return err
	}
	n.grpc = grpc
	return nil
}

func (n *ShutterNode) initP2P(ctx context.Context, cfg *config.Config) error {
	n.log.Info("got p2p config", "p2p-config", *cfg.P2P)
	mss, err := shp2p.New(cfg.P2P)
	if err != nil {
		return err
	}
	n.p2p = mss
	n.keyHandler = p2p.NewDecryptionKeyHandler(cfg.InstanceID, n.writer, n.log)
	n.p2p.AddMessageHandler(n.keyHandler)
	return nil
}

func (n *ShutterNode) Start(ctx context.Context) error {
	errgrp, teardown := service.RunBackground(ctx, n.grpc)
	n.errgrp = errgrp
	go func() {
		defer teardown()
		err := n.errgrp.Wait()
		n.log.Error("errgroup wait returned", "error", err)
		if err != nil {
			n.cancel(err)
		}
	}()

	p2perrgrp, p2pTeardown := service.RunBackground(n.resourcesCtx, n.keyManager, n.writer, n.p2p)
	go func() {
		defer p2pTeardown()
		err := p2perrgrp.Wait()
		n.log.Error("p2p errgroup wait returned", "error", err)
		if err != nil {
			n.cancel(err)
		}
	}()
	n.log.Info("Rollup node started")
	return nil
}

// Stop stops the node and closes all resources.
// If the provided ctx is expired, the node will accelerate the stop where possible, but still fully close.
func (n *ShutterNode) Stop(ctx context.Context) error {
	if n.closed.Load() {
		return errors.New("node is already closed")
	}
	// First wait for the gRPC shutdown
	err := n.errgrp.Wait()
	if err != nil {
		log.Error("errgroup errorered", "error", err)
	}
	// TODO: wait for next decryption-key (eventually with timeout)
	// For this we need to save what the "next" / last expected
	// decryption key should be.
	// HACK: for now just wait 10 seconds and hope key arrives
	// (gRPC is closed by now)
	n.log.Info("gRPC server closed, waiting before syncer shutdown")
	time.Sleep(10 * time.Second)

	var result *multierror.Error
	if n.resourcesClose != nil {
		// p2p, dbwriter and key-manager context
		n.resourcesClose()
	}

	if result == nil { // mark as closed if we successfully fully closed
		n.closed.Store(true)
	}

	if n.halted.Load() {
		// if we had a halt upon initialization, idle for a while,
		// to prevent a rapid restart-loop
		tim := time.NewTimer(time.Minute * 5)
		n.log.Warn("halted, idling to avoid immediate shutdown repeats")
		defer tim.Stop()
		select {
		case <-tim.C:
		case <-ctx.Done():
		}
	}

	log.Info("shutting down")
	return result.ErrorOrNil()
}

func (n *ShutterNode) Stopped() bool {
	return n.closed.Load()
}
