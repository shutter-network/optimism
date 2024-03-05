package client

import (
	"github.com/ethereum-optimism/optimism/shutter-node/grpc/v1"
	"github.com/pkg/errors"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type DecryptionKeyResult struct {
	Block     uint
	Active    bool
	SecretKey *shcrypto.EpochSecretKey

	Error error
}

func ToResult(in *grpc.DecryptionKey, err error) DecryptionKeyResult {
	k := &DecryptionKeyResult{Error: err}
	if in == nil && err == nil {
		// XXX: error message
		k.Error = errors.New("no values returned")
	}
	if in == nil {
		return *k
	}
	k.Block = uint(in.Block)
	k.Active = in.Active

	key := &shcrypto.EpochSecretKey{}
	if err := key.Unmarshal(in.Key); err != nil {
		k.Error = errors.Wrap(err, "marshal error")
		return *k
	}
	k.SecretKey = key
	return *k
}
