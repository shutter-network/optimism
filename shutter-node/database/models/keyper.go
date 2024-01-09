package models

import (
	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
)

type Keyper struct {
	gorm.Model
	// ID        uint `gorm:"primarykey"`
	// CreatedAt time.Time
	// UpdatedAt time.Time
	// DeletedAt gorm.DeletedAt `gorm:"index"`

	Address common.Address `gorm:"type:bytes;serializer:gob;index:,unique"`
	Eons    []*Eon         `gorm:"many2many:eon_keypers;"`
}

func (k *Keyper) ModelVersion() uint {
	return 1
}
