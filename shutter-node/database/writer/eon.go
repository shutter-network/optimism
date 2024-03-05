package writer

import (
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/pkg/errors"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func KeyperSetEventToModel(ks *syncevent.KeyperSet) (*models.Eon, error) {
	insertBlock, err := ks.AtBlockNumber.ToUInt64()
	if err != nil {
		return nil, errors.Wrap(err, "convert block-number")
	}
	keypers := []*models.Keyper{}
	for _, addr := range ks.Members {
		k := &models.Keyper{
			Address: addr,
		}
		k.Metadata.InsertBlock = uint(insertBlock)
		keypers = append(keypers, k)
	}

	eon := &models.Eon{
		EonIndex:        uint(ks.Eon),
		IsFinalized:     true,
		ActivationBlock: ks.ActivationBlock,
		Threshold:       ks.Threshold,
		Keypers:         keypers,
	}
	eon.Metadata.InsertBlock = uint(insertBlock)
	return eon, nil
}

func (w *DBWriter) handleKeyperSet(ks *syncevent.KeyperSet) error {
	w.log.Info("called handleKeyperSet in syncer")
	eon, err := KeyperSetEventToModel(ks)
	if err != nil {
		return errors.Wrap(err, "convert event")
	}
	rowsAffected := int64(0)
	err = w.db.Transaction(func(tx *gorm.DB) error {
		// In some cases the keyper-set has no keypers.
		// (We currently use an empty keyperset as
		// a hack to increase the first "real" eon
		// index to 1).
		if len(eon.Keypers) != 0 {
			// can exist already, keypers can be member of multiple keypersets
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&eon.Keypers)
			if res.Error != nil {
				return errors.Wrap(res.Error, "create keypers")
			}
		}
		// XXX: are the keyper IDs (primary keys) filled on a create-conflict-do-nothing?
		res := tx.Create(eon)
		if res.Error != nil {
			return errors.Wrap(res.Error, "update eon")
		}
		w.log.Info("save eon keyper set", "eon", eon)
		rowsAffected = res.RowsAffected
		return nil
	})
	if err == nil && rowsAffected != 0 {
		w.log.Info("successfully upserted keyper set", "eon", ks.Eon)
	}
	return err
}

func PublicKeyEventToModel(epk *syncevent.EonPublicKey) (*models.PublicKey, error) {
	atBlock, err := epk.AtBlockNumber.ToUInt64()
	if err != nil {
		return nil, errors.Wrap(err, "convert block-number")
	}

	pub := new(shcrypto.EonPublicKey)
	err = pub.GobDecode(epk.Key)
	if err != nil {
		return nil, errors.Wrap(err, "decode key")
	}

	pk := &models.PublicKey{
		Key:      pub,
		EonIndex: uint(epk.Eon),
	}
	pk.Metadata.InsertBlock = uint(atBlock)
	return pk, nil
}

func (w *DBWriter) handleEonKey(epk *syncevent.EonPublicKey) error {
	w.log.Info("called handleEonKey in syncer")
	pk, err := PublicKeyEventToModel(epk)
	if err != nil {
		return errors.Wrap(err, "convert event")
	}

	err = w.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Save(pk)
		if result.Error != nil {
			return result.Error
		}
		return err
	})
	if err == nil {
		w.log.Info("successfully upserted pubkey", "event-eon", epk.Eon, "db-eon-index", pk.EonIndex)
	}
	return err
}
