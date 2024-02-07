package query

import (
	"gorm.io/gorm"
)

type DBScope func(*gorm.DB) *gorm.DB

func ScopeSortedForBlock(blockNumber uint) DBScope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("block <= ?", blockNumber).Order("block DESC")
	}
}

func ScopeMostRecentForBlock(blockNumber uint) DBScope {
	return func(db *gorm.DB) *gorm.DB {
		return ScopeSortedForBlock(blockNumber)(db).Limit(1)
	}
}

func scopeNextEons(blockNumber uint) DBScope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("activation_block <= ?", blockNumber).Order("activation_block DESC")
	}
}

// ScopeEonAtBlock finds the most up to date eon for blockNumber at the
// chain-state of blockNumber.
func ScopeEonAtBlock(blockNumber uint) DBScope {
	return func(db *gorm.DB) *gorm.DB {
		return scopeNextEons(blockNumber)(db).Limit(1)
	}
}
