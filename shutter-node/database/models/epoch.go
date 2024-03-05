package models

import (
	"github.com/ethereum-optimism/optimism/shutter-node/keys/identity"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type EpochSecretKey struct {
	shcrypto.EpochSecretKey
}

func (sk *EpochSecretKey) GobEncode() ([]byte, error) {
	return sk.Marshal(), nil
}

func (sk *EpochSecretKey) GobDecode(data []byte) error {
	return sk.Unmarshal(data)
}

type Epoch struct {
	Metadata

	EonIndex  uint                     `gorm:"index:,unique,composite:eonblock"`
	Identity  *identity.Preimage       `gorm:"type:bytes;serializer:gob"`
	SecretKey *shcrypto.EpochSecretKey `gorm:"type:bytes;serializer:gob"`

	// This is the block the epoch references,
	// so at block-height 'Block', this epoch
	// is required to be included as reveal-tx
	// when shutter is active
	Block uint `gorm:"index:,unique,composite:eonblock"`
}

func (k *Epoch) ModelVersion() uint {
	return 1
}
