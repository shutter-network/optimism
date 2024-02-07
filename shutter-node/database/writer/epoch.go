package writer

import (
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (w *DBWriter) handleNewEpoch(epoch *models.Epoch) error {
	w.log.Info("handle new epoch called", "epoch", epoch)
	err := w.db.Transaction(func(tx *gorm.DB) error {
		// XXX: should we set the Metadata.InsertBlock?
		//  We could poll for the latest state here and use that
		//  block number.
		// Howeber this might be ambigouus, since the epoch arrival
		// is not directly synced with the latest-head events,
		// so it could be either inserted at
		// "latest-head" or "latest-head + 1".
		epochResult := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&epoch)
		if epochResult.Error != nil {
			return errors.Wrap(epochResult.Error, "create epoch")
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.log.Info("inserted epoch in db", "decrypt-block", epoch.Block)
	// TODO: notify the key-request fulfillment service that there is a new epoch
	// but use a non-blocking send, since the service is also polling the db
	return nil
}
