package writer

import (
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/database/query"
	"github.com/pkg/errors"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"gorm.io/gorm"
)

func LatestBlockEventToModel(lb *syncevent.LatestBlock) (*models.State, error) {
	num, err := lb.Number.ToUInt64()
	if err != nil {
		return nil, errors.Wrap(err, "convert block-number")
	}
	return &models.State{
		Metadata: models.Metadata{
			InsertBlock: uint(num),
		},
		Block: uint(num),
	}, nil
}

func (w *DBWriter) handleLatestBlock(lb *syncevent.LatestBlock) error {
	w.log.Info("handle latest block")
	newState, err := LatestBlockEventToModel(lb)
	if err != nil {
		return errors.Wrap(err, "convert event")
	}
	err = w.db.Transaction(func(tx *gorm.DB) error {
		// this is just the onchain event emitted from the
		// inbox contract.
		// This does not conclude fully wether shutter is "actice"
		// at the given time
		active, err := query.GetActiveState(tx, newState.Block)
		if err != nil {
			return errors.Wrap(err, "query active state")
		}
		// this searches for the updates "block"
		if active == nil {
			// we don't have an active event up until this block
			// XXX: what to do... we can create a "virtual" active entity
			// e.g. for block==0 ?
			active.Active = false
			active.Block = 0
			active.InsertBlock = newState.Block
		}
		w.log.Info("handle-latest block query active", "active", active)
		newState.Active = active.Active

		activeUpdate, err := query.GetActiveUpdate(tx, newState.Block)
		if err != nil {
			return errors.Wrap(err, "query active update")
		}
		if activeUpdate != nil {
			newState.ActiveUpdate = *active
			newState.ActiveUpdateID = active.Metadata.ID
		}

		eon := &models.Eon{}
		result := tx.Scopes(query.ScopeEonAtBlock(newState.Block)).Find(eon)
		if result.Error != nil {
			return errors.Wrap(result.Error, "get active eon")
		}
		if result.RowsAffected > 0 {
			newState.Eon = eon
			newState.EonID = &eon.Metadata.ID
		}
		w.log.Info("handle-latest block eon", "eon", eon)

		// FIXME: within the STF, IsShutterEnabled first checks that
		// an Eon key exists for the active keyper set, and then wether
		// the Inbox contract is active.
		// This means that a missing Eon-key for the current keyper-set also means
		// that shutter is not active.
		// This is not reflected in the State.Active field, is this
		// is only concerned with the shutter inbox activation events.

		// We don't check that a Publickey exists for the eon,
		// since this is strictly only necessary for receiving epoch-secret-keys
		// and it can happen that keypers don't have a key ready for their activation
		result = tx.Create(newState)
		if result.Error != nil {
			return errors.Wrap(result.Error, "insert new block")
		}
		return nil
	})

	if err == nil {
		w.log.Info(
			"latest head has been inserted to db",
			"block-number", newState.Block,
			"state", newState,
		)
	}
	return err
}
