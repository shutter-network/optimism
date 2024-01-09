package models

import (
	"github.com/ethereum-optimism/optimism/shutter-node/keys/identity"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
)

type Epoch struct {
	gorm.Model

	Eon       Eon `gorm:"foreignKey:EonIndex"`
	EonIndex  uint
	Identity  identity.Preimage        `gorm:"type:bytes;serializer:gob"`
	SecretKey *shcrypto.EpochSecretKey `gorm:"type:bytes;serializer:gob"`

	ConfirmedBatch *uint
}

func (k *Epoch) ModelVersion() uint {
	return 1
}
