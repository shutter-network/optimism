package keys

import (
	"context"
	"database/sql"
	"time"

	"github.com/ethereum-optimism/optimism/shutter-node/database"
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/database/query"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
)

type (
	CancelRequest        func(error)
	RequestDecryptionKey func(context.Context, uint) (<-chan *KeyRequestResult, CancelRequest)

	Manager interface {
		service.Service

		GetChannelNewState() chan<- *models.State
		RequestDecryptionKey(context.Context, uint) (<-chan *KeyRequestResult, CancelRequest)
	}
)

func New(db *database.Database, logger log.Logger) (Manager, error) {
	return &manager{
		db:            db,
		log:           logger,
		newState:      make(chan *models.State, 10),
		newKeyRequest: make(chan *keyRequest, 10),
		newEpoch:      make(chan *models.Epoch, 10),
	}, nil
}

type KeyRequestResult struct {
	Block     uint
	SecretKey *shcrypto.EpochSecretKey
	Error     error
}

type manager struct {
	db  *database.Database
	log log.Logger

	newKeyRequest chan *keyRequest
	newState      chan *models.State
	newEpoch      chan *models.Epoch
}

var (
	ErrNoEonForBlock     = errors.New("no eon found for block")
	ErrNoEpochForBlock   = errors.New("no epoch found for block")
	ErrPastBlockNotKnown = errors.New("no block state found, too far in past")
	ErrNoBlock           = errors.New("no block state found")
	ErrNotActive         = errors.New("shutter not active")
	ErrRequestAborted    = errors.New("request was aborted")
)

func (m *manager) queryEpochForBlock(db *gorm.DB, block uint) (*models.Epoch, error) {
	var epoch *models.Epoch
	err := db.Transaction(func(tx *gorm.DB) error {
		// TODO: this can likely be optimised with a tailored query.
		// And this SHOULD be optimised because we will poll this method regularly
		var err error
		active, err := query.GetActiveState(db, block)
		if err != nil {
			return errors.Wrapf(err, "get active state for block: %d", block)
		}
		// shutter is not active for that block
		if !active.Active {
			return ErrNotActive
		}
		eon, err := query.GetEonForBlock(db, block)
		if err != nil {
			return errors.Wrapf(err, "get eon for block: %d", block)
		}
		if eon == nil {
			return ErrNoEonForBlock
		}
		epk, err := query.GetPubKey(db, eon.EonIndex)
		if err != nil {
			return err
		}
		if epk == nil {
			// This is an additional check in the EVM -
			// if the eon has no public key, then
			// shutter is considered inactive too
			return ErrNotActive
		}

		epoch, err = query.GetEpochForInclusion(db, block, eon.EonIndex)
		if err != nil {
			return errors.Wrap(err, "retrieve epoch from database")
		}
		if epoch == nil {
			return ErrNoEpochForBlock
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if errors.Is(err, ErrNoEpochForBlock) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return epoch, nil
}

func (m *manager) GetChannelNewState() chan<- *models.State {
	return m.newState
}

func (m *manager) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		return m.eventLoop(ctx)
	},
	)
	runner.Defer(
		func() {
			m.cleanup(ctx)
		})
	return nil
}

type requestsMap map[uint][]*keyRequest

func (m *manager) checkRequestResult(reqs requestsMap, db *gorm.DB, latestState *models.State, latestEpoch *models.Epoch) error {
	if latestState == nil {
		// this function always gets fed the latest known state from the outside
		return errors.New("no latest state")
	}
	filled := []uint{}
	for block, requests := range reqs {
		var epoch *models.Epoch
		var err error

		// FIXME:
		// if block < earliestKnownState.Block {
		// return too far in past, no state known
		// }
		//

		// only fill epoch requests for up to the next
		// block after the known latest state
		if block > latestState.Block+1 {
			for _, request := range requests {
				request.touch()
			}
			continue
		}

		if latestEpoch != nil && block == latestEpoch.Block {
			epoch = latestEpoch
		}
		if epoch == nil {
			epoch, err = m.queryEpochForBlock(db, block)
		}
		// if errors.Is(err, ErrNoBlock) {
		// 	// TODO:
		// }
		if err != nil {
			// TODO: don't fill promise on internal errors that might
			// go away in another iteration
			for _, request := range requests {
				request.errorPromise(err)
				m.log.Info("filled key request promise",
					"block", epoch.Block, "success", false, "error", err)
			}
			filled = append(filled, block)
			continue
		} else {
			if epoch != nil {
				for _, request := range requests {
					request.success(epoch.SecretKey)
					m.log.Info("filled key request promise",
						"block", epoch.Block, "success", true, "error", nil)
				}
			} else {
				for _, request := range requests {
					request.touch()
				}
				continue
			}
		}
		filled = append(filled, block)
		continue
	}
	for _, filledBlock := range filled {
		delete(reqs, filledBlock)
	}
	return nil
}

func (m *manager) eventLoop(ctx context.Context) error {
	m.log.Debug("manager starting event loop")
	db := m.db.Session(ctx, m.log)
	requests := make(requestsMap)
	var latestState *models.State
	// HACK: for now just poll here, since we
	// currently dont send on the channels from
	// the other side
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	cleanupTimer := time.NewTicker(10 * time.Second)
	defer cleanupTimer.Stop()

evLoop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-cleanupTimer.C:
			// cleanup the queue so that only unprocessed
			// events remain in the queue
			m.log.Info("cleaning up filled key requests", "num-blocks", len(requests))
			for block, reqs := range requests {
				n := []*keyRequest{}
				for _, r := range reqs {
					if !r.processed() {
						n = append(n, r)
					}
				}
				if len(n) == 0 {
					delete(requests, block)
				} else {
					requests[block] = n
				}
			}
		case <-t.C:
			if len(requests) == 0 {
				// no work to do, the queue is empty
				continue
			}
			// HACK: for now just poll the db
			state, err := query.GetLatestState(db)
			if err != nil || state == nil {
				m.log.Error("couldn't poll latest state", "error", err, "state", state)
				continue evLoop
			}
			latestState = state
			err = m.checkRequestResult(requests, db, latestState, nil)
			if err != nil {
				m.log.Error("error checking request result", "error", err)
				// return on unrecoverable errors
				return err
			}
		case r, ok := <-m.newKeyRequest:
			if !ok {
				// XXX: what to do?
				return errors.New("key request closed")
			}
			m.log.Info("scheduling request", "request", r)

			reqs, ok := requests[r.block]
			if !ok {
				requests[r.block] = []*keyRequest{r}
			} else {
				reqs = append(reqs, r)
				requests[r.block] = reqs
			}

		// TODO:
		// the db writer does not notify those channels yet,
		// so this code is not used yet/
		// we want to make sure the we don't block the DB writer
		// when it is writing to those channels and the key manager
		// is doing something else.
		// it's fine to use a non-blocking send, since the notification
		// is an optimisation. as long as we are regularly polling the
		// db for updates on the requests we still don't miss anything.
		case e, ok := <-m.newEpoch:
			if !ok {
				// XXX: what to do?
				return errors.New("epoch receive closed")
			}
			if latestState == nil {
				m.log.Error("received new epoch, but latest state not set. wait for next poll.")
				continue evLoop
			}
			err := m.checkRequestResult(requests, db, latestState, e)
			if err != nil {
				// return on unrecoverable errors
				return err
			}
		case s, ok := <-m.newState:
			if !ok {
				// XXX: what to do?
				return errors.New("block receive closed")
			}
			latestState = s
			m.log.Info("received new latest state", "block", latestState.Block)
		}
	}
}

func (m *manager) cleanup(ctx context.Context) error {
	// FIXME: the sender should close the channels!
	// so the db-writer and the http-api putting values
	// on the newKeyRequest chan
	close(m.newKeyRequest)
	close(m.newEpoch)
	close(m.newState)
	return nil
}
