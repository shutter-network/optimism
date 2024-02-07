package writer

import (
	"context"
	"database/sql"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	syncclient "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/number"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"gorm.io/gorm"
)

var _ service.Service = &DBWriter{}

func NewDBWriter(url string, logger log.Logger, db *database.Database, opts ...Option) *DBWriter {
	return &DBWriter{
		options:   opts,
		log:       logger,
		url:       url,
		database:  db,
		eventChan: make(chan any),
	}
}

type DBWriter struct {
	options []Option

	// we need this to finish work
	// even after the start context is closed
	cancelDB context.CancelCauseFunc

	log      log.Logger
	url      string
	database *database.Database
	db       *gorm.DB
	client   *syncclient.Client

	eventChan chan any
}

func (w *DBWriter) Session(ctx context.Context, logger log.Logger) *gorm.DB {
	return w.database.Session(ctx, logger)
}

// Init reads in the options provided in the constructor,
// queries the database and creates the sync-client
// for the L2.
// If the DBWriter is run in a routine by means of
// the medley.Service inerface / the Start() method,
// Init() should not be called manually.
func (w *DBWriter) Init(ctx context.Context) error {
	w.db = w.database.Session(ctx, w.log)
	opts := defaultOptions()
	if err := opts.apply(w.options...); err != nil {
		return err
	}
	var dbLastSyncedBlock *uint64 = nil
	err := w.db.Transaction(func(tx *gorm.DB) error {
		latestState := &models.State{}
		result := tx.Order("block").Limit(1).Find(latestState)
		if result.Error != nil {
			// TODO: are there errors that we can catch?
			return result.Error
		}
		if result.RowsAffected > 0 {
			// initial block
			// FIXME: +1 or not?
			lb := uint64(latestState.Block) + 1
			dbLastSyncedBlock = &lb
		}
		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelDefault,
		ReadOnly:  true,
	})
	if err != nil {
		return err
	}

	// don't connect to the rollup when we're unit testing,
	// and also don't actually start the loop, since
	// we will be processing events synchronously
	if opts.unitTesting {
		return nil
	}

	c, err := syncclient.NewClient(
		ctx,
		syncclient.WithClientURL(w.url),
		syncclient.WithLogger(w.log),
		// NOTE: this means we get the block-events for the time we missed,
		// but we don't get epoch keys!
		// First: Beware of that, Second: make it so that the db-writer itself
		// can handle missing epochs and does not make their existance
		/// conditional to anything:
		syncclient.WithSyncStartBlock(number.NewBlockNumber(dbLastSyncedBlock)),
		syncclient.WithSyncNewEonKey(w.HandleEonKey),
		syncclient.WithSyncNewShutterState(w.HandleShutterState),
		syncclient.WithSyncNewBlock(w.HandleLatestBlock),
		syncclient.WithSyncNewKeyperSet(w.HandleKeyperSet),
	)
	if err != nil {
		return err
	}
	w.client = c
	return err
}

func (w *DBWriter) forwardEvent(ctx context.Context, ev any) error {
	select {
	case w.eventChan <- ev:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (w *DBWriter) HandleLatestBlock(ctx context.Context, lb *syncevent.LatestBlock) error {
	return w.forwardEvent(ctx, lb)
}

func (w *DBWriter) HandleKeyperSet(ctx context.Context, ks *syncevent.KeyperSet) error {
	return w.forwardEvent(ctx, ks)
}

func (w *DBWriter) HandleEonKey(ctx context.Context, epk *syncevent.EonPublicKey) error {
	return w.forwardEvent(ctx, epk)
}

func (w *DBWriter) HandleShutterState(ctx context.Context, ss *syncevent.ShutterState) error {
	return w.forwardEvent(ctx, ss)
}

func (w *DBWriter) HandleNewEpoch(ctx context.Context, epc *models.Epoch) error {
	return w.forwardEvent(ctx, epc)
}
