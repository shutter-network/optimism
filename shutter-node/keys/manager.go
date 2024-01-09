package keys

import (
	"context"

	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
)

type Manager interface {
	service.Service

	GetPublicKey(eon uint64) *shcrypto.EonPublicKey
	IsKeyperInEon(eon uint64, address common.Address) bool

	GetChannelNewBlock() chan<- *models.Block
	GetChannelNewEpoch() chan<- *models.Epoch
	GetChannelNewEon() chan<- *models.Eon

	RequestDecryptionKey(context.Context, uint64) <-chan *KeyRequestResult
}

// XXX: nocheckin, this is just there to instantiate for now
func New(db *database.Database, logger log.Logger) (Manager, error) {
	return &manager{
		newEpoch:                 make(chan *models.Epoch, 1),
		newBlockReceiveFinalized: make(chan *models.Block, 1),
		newEon:                   make(chan *models.Eon, 1),
		db:                       db,
		log:                      logger,
	}, nil
}

type KeyRequestResult struct {
	Batch     uint64
	SecretKey *shcrypto.EpochSecretKey
	Error     error
}

type manager struct {
	db  *database.Database
	log log.Logger

	newEpoch                 chan *models.Epoch
	newEon                   chan *models.Eon
	newBlockReceiveFinalized chan *models.Block
}

var (
	ErrNoEonForBatch  = errors.New("no eon found for batch")
	ErrRequestAborted = errors.New("request was aborted")
)

func (m *manager) processDecryptionKeyRequest(ctx context.Context, batch uint64) <-chan *KeyRequestResult {
	return nil
}

// TODO: maybe return error when eon doesn't exist
func (m *manager) GetPublicKey(eon uint64) *shcrypto.EonPublicKey {
	// FIXME: query db
	return nil
}

// TODO: maybe return error when eon doesn't exist
func (m *manager) IsKeyperInEon(eon uint64, address common.Address) bool {
	// FIXME: query db
	return false
}

func (m *manager) GetChannelNewBlock() chan<- *models.Block {
	return m.newBlockReceiveFinalized
}

func (m *manager) GetChannelNewEpoch() chan<- *models.Epoch {
	return m.newEpoch
}

func (m *manager) GetChannelNewEon() chan<- *models.Eon {
	return m.newEon
}

// RequestDecryptionKey does not actively request the decryption at the
// keypers, but it will initiate an internal subscription that
// fulfils as soon as the decryption key was received from the keypers.
func (m *manager) RequestDecryptionKey(ctx context.Context, batch uint64) <-chan *KeyRequestResult {
	// TODO: pass on to the key-request handler
	return nil
}

func (m *manager) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		return m.eventLoop(ctx)
	},
	)
	return nil
}

func (m *manager) handleNewEon(ctx context.Context, eon *models.Eon, db *gorm.DB) error {
	m.log.Info("handle new eon called")
	// this is written to the db in the syncer ...
	return nil
}

func (m *manager) handleNewEpoch(ctx context.Context, epoch *models.Epoch, db *gorm.DB) error {
	m.log.Info("handle new epoch called")
	err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.FirstOrCreate(&epoch)
		if res.Error != nil {
			return res.Error
		}
		res.Save(&epoch)
		return nil
	})
	// TODO: the epoch is only partially filled with data.
	// we will query for the eon an insert it together with the link to the eon into the db
	// TODO:
	// we also want to verify that the correct key was indeed belonging to the at that time
	// active keyperset. and that shutter was enabled!
	// (write separately in db / keyperset)
	return err
}

func (m *manager) handleNewKeyRequest(ctx context.Context, req *keyRequest) error {
	ok := true
	// TODO: Pre-check
	if !ok {
		// XXX: internal server error..
		errorPromise(req, ErrNoEonForBatch)
		return nil
	}
	// FIXME: implement
	return nil
}

func (m *manager) eventLoop(ctx context.Context) error {
	db := m.db.Session(ctx, m.log)
	stop := make(chan error, 1)
	// FIXME: set initial block?
	var latestBlock uint
	for {
		select {
		case err := <-stop:
			close(stop)
			return err
		case <-ctx.Done():
			stop <- ctx.Err()
		case e, ok := <-m.newBlockReceiveFinalized:
			// NOTE: when this block arrives here,
			// it is assumed that all events for this block
			// have already been processed and the effects have
			// been included in the database
			if !ok {
				stop <- nil
				continue
			}
			// TODO: we need this for better processing?
			latestBlock = e.Number
			m.log.Info("received new block", "block", latestBlock)
		case e, ok := <-m.newEpoch:
			if !ok {
				stop <- nil
				continue
			}
			err := m.handleNewEpoch(ctx, e, db)
			if err != nil {
				stop <- err
			}
			// TODO: pass on to the key-request handler if we got a new key
		case e, ok := <-m.newEon:
			if !ok {
				stop <- nil
				continue
			}
			err := m.handleNewEon(ctx, e, db)
			if err != nil {
				stop <- err
			}
		}
	}
}
