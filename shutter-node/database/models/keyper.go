package models

import (
	"github.com/ethereum/go-ethereum/common"
)

type Keyper struct {
	Metadata

	Address common.Address `gorm:"type:bytes;serializer:gob;index:,unique"`
	Eons    []*Eon         `gorm:"many2many:eon_keypers;"`
}

func (k *Keyper) ModelVersion() uint {
	return 1
}
