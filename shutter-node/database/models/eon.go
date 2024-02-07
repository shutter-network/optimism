package models

import (
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type PublicKey struct {
	Metadata
	EonIndex uint `gorm:"uniqueIndex"`

	// FIXME: the gob serializer does not deal well with nil values
	//  it seems to block forever
	Key *shcrypto.EonPublicKey `gorm:"type:bytes;serializer:gob"`
}

type Eon struct {
	Metadata
	EonIndex uint `gorm:"uniqueIndex"`

	IsFinalized     bool
	ActivationBlock uint64
	Threshold       uint64

	Keypers []*Keyper `gorm:"many2many:eon_keypers;"`
}

func (k *Eon) ModelVersion() uint {
	return 1
}
