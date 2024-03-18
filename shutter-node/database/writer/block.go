package writer

import (
	"fmt"

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

func (w *DBWriter) deleteAbove(db *gorm.DB, blockNum uint) error {
	w.log.Info("delete database entries", "where", fmt.Sprintf("insert_block > %d", blockNum))
	// Unscoped because soft delete violates the unique constraint
	res := db.Unscoped().Delete(&models.State{}, "insert_block > ?", blockNum)
	if res.Error != nil {
		return res.Error
	}
	res = db.Unscoped().Delete(&models.Eon{}, "insert_block > ?", blockNum)
	if res.Error != nil {
		return res.Error
	}
	res = db.Unscoped().Delete(&models.Keyper{}, "insert_block > ?", blockNum)
	if res.Error != nil {
		return res.Error
	}
	// NOTE: We don't delete the epoch keys we previously received,
	// since they keypers currently don't rebroadcast them upon a reorg,
	// and there would be a race condition even if they would do so.
	// FIXME: Handle the edgecase, where during reorg the keyperset
	// for that eon-index changes and the old, now invalid key is still
	// present in the database. The new key currently would not be inserted into the
	// db because of the "on-conflict do nothing" policy.
	return nil
}

func (w *DBWriter) handleLatestBlock(lb *syncevent.LatestBlock) error {
	newState, err := LatestBlockEventToModel(lb)
	if err != nil {
		return errors.Wrap(err, "convert event")
	}
	w.log.Info("handle new l2 unsafe head", "block-number", newState.Block)
	err = w.db.Transaction(func(tx *gorm.DB) error {
		latest, err := query.GetLatestBlock(tx)
		if err != nil {
			return errors.Wrap(err, "query latest block")
		}
		if latest != nil && newState.Block <= *latest {
			w.log.Warn("reorg detected", "block", newState.Block+1)
			// if a reorg happens, then the chainsyncer will
			// emit the latest-head before the newly reorged
			// head in order to signal that a reorg is incoming.
			// this means we only wind back changes ABOVE
			// this block number.
			if err := w.deleteAbove(tx, newState.Block); err != nil {
				return errors.Wrap(err, "handle reorg in database")
			}
			// don't apply any further state-changes, since
			// we don't want to alter the parent of the newly
			// re-orged head.
			// the new events and the latest head event will follow
			// downstream
			return nil
		}

		// this is just the onchain event emitted from the
		// inbox contract.
		// keypersetmanager contract.
		// This does not conclude fully wether shutter is "active"
		// at the given time, since this also depends on the
		// eon key being broadcast by the keypers.
		active, err := query.GetActiveState(tx, newState.Block)
		if err != nil {
			return errors.Wrap(err, "query active state")
		}
		// this searches for the updates "block"
		if active == nil {
			// we don't have an active event up until this block
			// create a virtual active-update entity, but don't persist to db
			active = &models.ActiveUpdate{
				Metadata: models.Metadata{
					InsertBlock: newState.Block,
				},
				Block:  0,
				Active: true,
			}
		}
		newState.Active = active.Active

		// query the database wether we received a paused/unpaused
		// state update this block, taking effect the next block
		activeUpdate, err := query.GetActiveUpdate(tx, newState.Block)
		if err != nil {
			return errors.Wrap(err, "query active update")
		}
		if activeUpdate != nil {
			newState.ActiveUpdate = activeUpdate
			newState.ActiveUpdateID = &activeUpdate.Metadata.ID
		}

		// query the database for the active eon at the block.
		// this can be an older eon, or one for the current block that was
		// just inserted into the db
		eon := &models.Eon{}
		result := tx.Scopes(query.ScopeEonAtBlock(newState.Block)).Find(eon)
		if result.Error != nil {
			return errors.Wrap(result.Error, "get active eon")
		}
		if result.RowsAffected > 0 {
			newState.Eon = eon
			newState.EonID = &eon.Metadata.ID
		}

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
		var eonIndex *uint
		if newState.Eon != nil {
			eonIndex = &newState.Eon.EonIndex
		}
		w.log.Info(
			"new l2 unsafe head state has been inserted to db",
			"eon-index", eonIndex,
			"block-number", newState.Block,
			"shutter-active", newState.Active,
		)
	}
	return err
}
