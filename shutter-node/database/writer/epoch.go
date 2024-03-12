package writer

import (
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (w *DBWriter) handleNewEpoch(epoch *models.Epoch) error {
	var duplicate bool
	err := w.db.Transaction(func(tx *gorm.DB) error {
		epochResult := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&epoch)
		if epochResult.Error != nil {
			return errors.Wrap(epochResult.Error, "create epoch")
		}
		if epochResult.RowsAffected == 0 {
			duplicate = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !duplicate {
		w.log.Info("decryption-key inserted into db",
			"reveal-block", epoch.Block,
			"eon-index", epoch.EonIndex,
		)
	} else {
		w.log.Info("handled duplicate decryption-key, not inserted into db",
			"reveal-block", epoch.Block,
			"eon-index", epoch.EonIndex,
		)
	}
	// TODO: notify the key-request fulfillment service that there is a new epoch
	// but use a non-blocking send, since the service is also polling the db
	return nil
}
