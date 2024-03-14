package shutter_test

import (
	"context"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/database/writer"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/p2p"
	"github.com/ethereum/go-ethereum/log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gotest.tools/assert"
)

type Tester struct {
	log         log.Logger
	manager     keys.Manager
	writer      *writer.DBWriter
	decrHandler *p2p.DecryptionKeyHandler
	db          *gorm.DB
	events      []*TestEvent
}

const InstanceID = 42

func Setup(ctx context.Context, t *testing.T) *Tester {
	t.Helper()
	db := &database.Database{}

	path, err := os.MkdirTemp("", "test-shutter-node-db-*")
	assert.NilError(t, err)
	// close and remove the temporary file at the end of the program
	t.Cleanup(func() {
		os.RemoveAll(path)
	})

	err = db.Connect(path + "/db")
	assert.NilError(t, err)

	logger := log.New()
	logger.SetHandler(log.StdoutHandler)
	m, err := keys.New(db, logger)
	assert.NilError(t, err)

	t.Cleanup(func() {
		err := db.Close()
		assert.NilError(t, err)
	})

	// The UnitTesting option will not connect to the RPC, so the URL doesn't have any effect
	w := writer.NewDBWriter("http://localhost:8454", logger, db, writer.UnitTesting())

	return &Tester{
		log:         logger,
		manager:     m,
		writer:      w,
		db:          db.Session(ctx, logger),
		decrHandler: p2p.NewDecryptionKeyHandler(InstanceID, w, logger),
	}
}

func (tst *Tester) Events(ev ...*TestEvent) {
	tst.events = append(tst.events, ev...)
}

func (tst *Tester) Start(ctx context.Context, runner service.Runner) error {
	cclCtx, cancel := context.WithCancel(ctx)
	grp, mgrDefer := service.RunBackground(cclCtx, tst.manager)
	runner.Defer(mgrDefer)

	runner.Go(
		func() error {
			if err := tst.writer.Init(ctx); err != nil {
				return err
			}

			if err := tst.Process(ctx); err != nil {
				return err
			}
			cancel()

			err := grp.Wait()
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		})
	return nil
}

type (
	DecryptionKeyRequest uint
)

var ErrCloseTester = errors.New("close tester signal received")

func (tst *Tester) schedulePushEvent(ctx, thisEventCtx context.Context, grp *errgroup.Group, ev *TestEvent) {
	grp.Go(func() error {
		switch evTyped := ev.Value.(type) {
		case *syncevent.KeyperSet:
			if err := tst.writer.HandleKeyperSet(thisEventCtx, evTyped); err != nil {
				return errors.Wrap(err, "handle keyper-set")
			}
		case *syncevent.EonPublicKey:
			if err := tst.writer.HandleEonKey(thisEventCtx, evTyped); err != nil {
				return errors.Wrap(err, "handle eon-public-key")
			}
		case *syncevent.LatestBlock:
			if err := tst.writer.HandleLatestBlock(thisEventCtx, evTyped); err != nil {
				return errors.Wrap(err, "handle latest-block")
			}
		case *syncevent.ShutterState:
			if err := tst.writer.HandleShutterState(thisEventCtx, evTyped); err != nil {
				return errors.Wrap(err, "handle shutter-state")
			}
		case *p2pmsg.DecryptionKeys:
			validate, err := tst.decrHandler.ValidateMessage(thisEventCtx, evTyped)
			if validate != pubsub.ValidationAccept {
				// TODO:
				_ = err
				// FIXME: will this break the switch?
				tst.log.Error("decryption key validation failed", "error", err)
				break
			}

			// FIXME: this calls the async method internally
			msgs, err := tst.decrHandler.HandleMessage(thisEventCtx, evTyped)
			if err != nil {
				return errors.Wrap(err, "handle message")
			}
			if msgs != nil {
				return errors.New("handle message returned non-nil messages")
			}
		case DecryptionKeyRequest:
			res, cancel := tst.manager.RequestDecryptionKey(thisEventCtx, uint(evTyped))
			go func(ctx context.Context, evnt *TestEvent, result <-chan *keys.KeyRequestResult) {
				select {
				case <-ctx.Done():
					cancel(ctx.Err())
					return
				case krRes := <-result:
					tst.log.Info("set result DecryptionKeyRequest", "result", krRes)
					evnt.SetResult(ctx, krRes, nil)
					return
				}
			}(ctx, ev, res)
		case final:
			tst.log.Info("received 'Close' signal, stop test-event handling")
			return ErrCloseTester
		default:
			return errors.New("event type not supported in tester")
		}
		return nil
	})
}

func (tst *Tester) scheduleProcessEvent(ctx, thisEventCtx context.Context, grp *errgroup.Group, ev *TestEvent) {
	grp.Go(
		func() error {
			// Schedule the event processing of 1 event
			switch ev.Value.(type) {
			case *syncevent.KeyperSet,
				*syncevent.EonPublicKey,
				*syncevent.LatestBlock,
				*syncevent.ShutterState,
				*p2pmsg.DecryptionKeys:

				err := tst.writer.ProcessNextEvent(thisEventCtx)
				if err != nil {
					return errors.Wrapf(err, "error while processing test-event: '%s' ", ev.String())
				}
			case DecryptionKeyRequest, final:
			// don't process, because those events also
			// don't put anything on the writers process channel
			// and thus would hang forever
			default:
				return errors.New("event type not supported in tester")
			}
			return nil
		})
}

func (tst *Tester) Process(ctx context.Context) error {
	for _, ev := range tst.events {
		if err := ev.PreCheck(tst.db); err != nil {
			return errors.Wrapf(err, "pre-check failed for test-event: '%s' ", ev.String())
		}

		// Schedule the event dispatching of 1 event
		grp, grpCtx := errgroup.WithContext(ctx)

		tst.scheduleProcessEvent(ctx, grpCtx, grp, ev)
		tst.schedulePushEvent(ctx, grpCtx, grp, ev)
		err := grp.Wait()
		if errors.Is(err, ErrCloseTester) {
			break
		}
		if err != nil {
			return err
		}
		if err := ev.PostCheck(tst.db); err != nil {
			return errors.Wrapf(err, "post-check failed for test-event: '%s' ", ev.String())
		}
	}

	tst.log.Info("doing final checks")
	for _, ev := range tst.events {
		if err := ev.FinalCheck(tst.db); err != nil {
			return errors.Wrap(err, "final-check failed")
		}
	}
	tst.log.Info("finished processing")
	return nil
}
