package manager_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/p2p"
	"github.com/ethereum-optimism/optimism/shutter-node/rollup"
	"github.com/ethereum/go-ethereum/log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	shopevent "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/optimism/sync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"gorm.io/gorm"
	"gotest.tools/assert"
)

type Tester struct {
	t           *testing.T
	manager     keys.Manager
	syncer      *rollup.Syncer
	decrHandler *p2p.DecryptionKeyHandler
	db          *gorm.DB
	events      []*TestEvent
}

const InstanceID = 42

func Setup(ctx context.Context, t *testing.T) *Tester {
	t.Helper()

	db := &database.Database{}
	f, err := os.CreateTemp("/tmp", "test-shutter-node-db.")
	assert.NilError(t, err)

	// close and remove the temporary file at the end of the program
	t.Cleanup(func() {
		f.Close()
		os.Remove(f.Name())
	})

	err = db.Connect(f.Name())
	assert.NilError(t, err)

	logger := log.New()
	logger.SetHandler(log.StdoutHandler)
	m, err := keys.New(db, logger)
	assert.NilError(t, err)
	syncer := rollup.NewL2Syncer("dummyurl", logger, m, db)
	err = syncer.Init(ctx)
	assert.NilError(t, err)
	return &Tester{
		t:           t,
		manager:     m,
		syncer:      syncer,
		decrHandler: p2p.NewDecryptionKeyHandler(InstanceID, m, logger),
	}
}

func (tst *Tester) Events(ev ...*TestEvent) {
	tst.events = append(tst.events, ev...)
}

func (tst *Tester) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		tst.Process(ctx)
		return nil
	},
	)
	return nil
}

type (
	DecryptionKeyRequest uint64
)

func (tst *Tester) Process(ctx context.Context) {
	for _, ev := range tst.events {
		assert.NilError(tst.t, ev.PreCheck(tst.db))
		switch evTyped := ev.Value.(type) {
		case *shopevent.KeyperSet:
			err := tst.syncer.HandleKeyperSet(ctx, evTyped)
			assert.NilError(tst.t, err)
		case *shopevent.EonPublicKey:
			err := tst.syncer.HandleEonKey(ctx, evTyped)
			assert.NilError(tst.t, err)
		case *shopevent.LatestBlock:
			err := tst.syncer.HandleLatestBlock(ctx, evTyped)
			assert.NilError(tst.t, err)
		case *shopevent.ShutterState:
			err := tst.syncer.HandleShutterState(ctx, evTyped)
			assert.NilError(tst.t, err)
		case *p2pmsg.DecryptionKey:
			validate, err := tst.decrHandler.ValidateMessage(ctx, evTyped)
			if validate != pubsub.ValidationAccept {
				// TODO:
				_ = err
				// FIXME: will this break the switch?
				break
			}
			msgs, err := tst.decrHandler.HandleMessage(ctx, evTyped)
			assert.NilError(tst.t, err)
			assert.Check(tst.t, msgs == nil)
		case DecryptionKeyRequest:
			res := tst.manager.RequestDecryptionKey(ctx, uint64(evTyped))
			go func() {
				select {
				case <-ctx.Done():
					return
				case krRes := <-res:
					ev.SetResult(ctx, krRes)
					return
				}
			}()
		default:
			err := errors.New("event type not supported in tester")
			assert.NilError(tst.t, err)
		}
		assert.NilError(tst.t, ev.Wait(ctx, tst.db))
		assert.NilError(tst.t, ev.PostCheck(tst.db))
	}

	for _, ev := range tst.events {
		assert.NilError(tst.t, ev.FinalCheck(tst.db))
	}
}
