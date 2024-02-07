package query

import (
	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetActiveUpdate(db *gorm.DB, block uint) (*models.ActiveUpdate, error) {
	return getObjByColumn(db, new(models.ActiveUpdate), "insert_block", block)
}

func GetActiveState(db *gorm.DB, block uint) (*models.ActiveUpdate, error) {
	update := new(models.ActiveUpdate)
	res := db.Scopes(ScopeMostRecentForBlock(block)).Take(update)
	return CheckGetUniqueObject(update, res)
}

func GetLatestBlock(db *gorm.DB) (uint, error) {
	state := new(models.State)
	db = db.Order("block DESC").Limit(1).Take(state)
	state, err := CheckGetUniqueObject(state, db)
	if err != nil {
		return 0, err
	}
	return state.Block, nil
}

// This returns the latest COMITTED state.
// This means that this is the state of a
// latest head event including the state updates
// for all events of that block.
func GetLatestState(db *gorm.DB) (*models.State, error) {
	state := new(models.State)
	db = db.Preload(clause.Associations).Preload("Eon.Keypers")
	db = db.Order("block DESC").Limit(1).Take(state)
	return CheckGetUniqueObject(state, db)
}

func GetState(db *gorm.DB, block uint) (*models.State, error) {
	db = db.Preload(clause.Associations).Preload("Eon.Keypers")
	return getObjByColumn(db, new(models.State), "block", block)
}

func GetPubKey(db *gorm.DB, index uint) (*models.PublicKey, error) {
	return getObjByColumn(db, new(models.PublicKey), "eon_index", index)
}

func GetEonByIndex(db *gorm.DB, index uint) (*models.Eon, error) {
	db = db.Preload(clause.Associations)
	return getObjByColumn(db, new(models.Eon), "eon_index", index)
}

func GetEonForBlock(db *gorm.DB, blockNumber uint) (*models.Eon, error) {
	eon := new(models.Eon)
	db = db.Preload(clause.Associations)
	res := db.Scopes(ScopeEonAtBlock(blockNumber)).Take(eon)
	return CheckGetUniqueObject(eon, res)
}

// GetEpoch retrieves the epoch that is relevant for inclusion
// in block 'atBlock', and thus the next block after the
// "DecryptionBlock" of this epoch.
func GetEpochForInclusion(db *gorm.DB, atBlock uint, index uint) (*models.Epoch, error) {
	db = db.Where("eon_index = ?", index)
	return getObjByColumn(db, new(models.Epoch), "block", atBlock)
}
