package writer

import (
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/pkg/errors"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"gorm.io/gorm"
)

func ShutterStateToActive(s *syncevent.ShutterState) (*models.ActiveUpdate, error) {
	atBlock, err := s.AtBlockNumber.ToUInt64()
	if err != nil {
		return nil, errors.Wrap(err, "convert block-number")
	}
	return &models.ActiveUpdate{
		Metadata: models.Metadata{
			InsertBlock: uint(atBlock),
		},
		// although the event was emitted in block atBlock,
		// the effect only applies one block after
		Block:  uint(atBlock) + 1,
		Active: s.Active,
	}, nil
}

func (w *DBWriter) handleShutterActive(epk *syncevent.ShutterState) error {
	active, err := ShutterStateToActive(epk)
	if err != nil {
		return errors.Wrap(err, "convert event")
	}
	w.log.Info("handle shutter paused/unpaused event",
		"unpaused", active.Active, "block", active.Block)
	return w.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Create(active)
		if result.Error != nil {
			return errors.Wrap(result.Error, "create new shutter state")
		}
		return nil
	})
}
