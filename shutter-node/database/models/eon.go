package models

import (
	"time"

	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
)

// FIXME:  what if the events are received out of order,
// and updated at block will decrease?
type Eon struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	UpdatedAtBlock uint64 // latest blocknumber that caused an update
	// Candidate      bool

	// this is the "eon" value:
	Index           uint `gorm:"primarykey"`
	IsFinalized     bool
	ActivationBlock uint64
	Threshold       uint64
	PublicKey       *shcrypto.EonPublicKey `gorm:"type:bytes;serializer:gob"`

	Keypers []*Keyper `gorm:"many2many:eon_keypers;"`
}

func (k *Eon) ModelVersion() uint {
	return 1
}

func (k Eon) NextEons(blockNumber uint) DBScope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("activation_block <= ?", blockNumber).Order("activation_block DESC")
	}
}
