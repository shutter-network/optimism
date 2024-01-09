package rollup

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"

	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	shopclient "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/optimism/sync"
	shopevent "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/optimism/sync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ service.Service = &Syncer{}

func NewL2Syncer(url string, log log.Logger, m keys.Manager, db *database.Database) *Syncer {
	return &Syncer{
		log:      log,
		url:      url,
		database: db,
	}
}

type Syncer struct {
	log      log.Logger
	url      string
	database *database.Database
	db       *gorm.DB
	client   *shopclient.ShutterL2Client
}

func (s *Syncer) Init(ctx context.Context) error {
	s.db = s.database.Session(ctx, s.log)
	return nil
}

func (s *Syncer) Start(ctx context.Context, runner service.Runner) error {
	err := s.Init(ctx)
	if err != nil {
		return err
	}
	// XXX: maybe use the instrumented client here
	// iclient := client.NewInstrumentedClient(jsonRPC, n.metrics)
	// shopclient.WithClient(iclient)
	// TODO: retrieve the last synced block from the db and
	// set as sync start block
	c, err := shopclient.NewShutterL2Client(
		ctx,
		shopclient.WithClientURL(s.url),
		shopclient.WithLogger(s.log),
		shopclient.WithSyncStartBlock(nil),
		shopclient.WithSyncNewEonKey(s.HandleEonKey),
		shopclient.WithSyncNewShutterState(s.HandleShutterState),
		shopclient.WithSyncNewKeyperSet(s.HandleKeyperSet),
	)
	if err != nil {
		return err
	}
	s.client = c
	err = runner.StartService(s.client)
	if err != nil {
		return err
	}
	return nil
}

// When this is called, we assume that all relevant events for the new latest-block
// have been processed and the results are represented in the DB.
func (s *Syncer) finalizeLatestState(ctx context.Context, db *gorm.DB, oldState, newState *models.State) error {
	// TODO: think about locking
	return s.db.Transaction(func(tx *gorm.DB) error {
		if oldState == nil {
			oldState := &models.State{IsLatest: true, IsPending: false}
			result := tx.First(oldState)
			if result.Error != nil {
				return errors.Wrap(result.Error, "query latest state")
			}
		}
		if oldState.BlockNumber > newState.BlockNumber {
			return errors.New("block number not increasing")
		}
		eon := &models.Eon{}
		result := tx.Scopes(eon.NextEons(newState.BlockNumber)).First(eon)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return errors.Wrap(result.Error, "retrieve latest active eon")
		}
		var activeEon *uint
		activeEon = nil
		if result.RowsAffected != 0 {
			activeEon = &eon.Index
		}
		oldState.IsLatest = false
		newState.IsLatest = true
		newState.EonIndex = activeEon
		// If those values haven't b
		if newState.ShutterActive == nil {
			newState.ShutterActive = oldState.ShutterActive
		}
		if newState.EonIndex == nil {
			newState.ShutterActive = oldState.ShutterActive
		}
		tx.Save(newState)
		tx.Delete(oldState)
		return nil
	})
	// TODO: Now we can trigger the manager service and signal that a new block has been processed
}

// XXX: maybe this should always get triggered when a new event for the latest
// block gets processed?
func (s *Syncer) HandleLatestBlock(ctx context.Context, lb *shopevent.LatestBlock) error {
	num, err := lb.Number.ToUInt64()
	if err != nil {
		return err
	}
	newLatestBlockNum := uint(num)
	// TODO: think about locking?
	return s.db.Transaction(func(tx *gorm.DB) error {
		newState := &models.State{
			BlockNumber: newLatestBlockNum,
			IsLatest:    false,
			IsPending:   true,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(newState)
		// TODO: pass rows-affected 0
		if result.Error != nil {
			return result.Error
		}

		s.finalizeLatestState(ctx, tx, latestState, newState)
		return nil
	})
}

func (s *Syncer) HandleKeyperSet(ctx context.Context, ks *shopevent.KeyperSet) error {
	s.log.Info("called handleKeyperSet in syncer")
	atBlock, err := ks.AtBlockNumber.ToUInt64()
	_ = atBlock
	if err != nil {
		return errors.Wrap(err, "convert block-number")
	}
	// TODO: think about locking?
	err = s.db.Transaction(func(tx *gorm.DB) error {
		keypers := []*models.Keyper{}
		for _, addr := range ks.Members {
			keypers = append(keypers, &models.Keyper{
				Address: addr,
			})
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&keypers)
		if res.Error != nil {
			return errors.Wrap(res.Error, "create keypers")
		}
		eon := &models.Eon{
			UpdatedAtBlock:  atBlock,
			Index:           uint(ks.Eon),
			IsFinalized:     true,
			ActivationBlock: ks.ActivationBlock,
			Threshold:       ks.Threshold,
			Keypers:         keypers,
		}
		// FIXME: public-key nil value causes this to hang.
		// (if here the PublicKey is not omitted for example)
		// Probably the serializer?
		res = tx.Omit("PublicKey").Save(eon)
		if res.Error != nil {
			return errors.Wrap(res.Error, "update eon")
		}
		return nil
	})
	if err == nil {
		s.log.Info("successfully upserted keyper set", "eon", ks.Eon)
	}
	return err
}

func (s *Syncer) HandleShutterState(ctx context.Context, epk *shopevent.ShutterState) error {
	s.log.Info("called HandleShutterState in syncer")
	atBlock, err := epk.AtBlockNumber.ToUInt64()
	if err != nil {
		return errors.Wrap(err, "convert block-number")
	}
	// TODO: think about locking?
	return s.db.Transaction(func(tx *gorm.DB) error {
		state := &models.State{BlockNumber: uint(atBlock), IsLatest: false, IsPending: true}
		// XXX: will the assign work that only the shutteractive will get updated?
		result := tx.Assign(models.State{ShutterActive: &epk.Active}).FirstOrCreate(state)
		if result.Error != nil {
			return errors.Wrap(result.Error, "upsert shutter state")
		}
		return nil
	})
}

func (s *Syncer) HandleEonKey(ctx context.Context, epk *shopevent.EonPublicKey) error {
	s.log.Info("called handleEonKey in syncer")
	// TODO: think about locking?
	err := s.db.Transaction(func(tx *gorm.DB) error {
		pub := new(shcrypto.EonPublicKey)
		err := pub.GobDecode(epk.Key)
		if err != nil {
			return errors.Wrap(err, "decode key")
		}
		block, err := epk.AtBlockNumber.ToUInt64()
		if err != nil {
			return errors.Wrap(err, "convert block-number")
		}
		eon := &models.Eon{
			Index:          uint(epk.Eon),
			PublicKey:      pub,
			UpdatedAtBlock: block,
		}

		res := tx.Select("PublicKey", "UpdatedAtBlock").Save(eon)
		if res.Error != nil {
			return errors.Wrap(res.Error, "update eon public key")
		}
		return nil
	})
	return err
}
