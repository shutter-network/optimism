package client

import (
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type DecryptionKeyResult struct {
	Block     uint
	Active    bool
	SecretKey *shcrypto.EpochSecretKey
}
