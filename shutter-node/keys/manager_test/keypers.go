package manager_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	shopevent "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/optimism/sync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/number"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/identitypreimage"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/testkeygen"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"gotest.tools/assert"
)

var dummyID = identitypreimage.BigToIdentityPreimage(common.Big0)

func NewKeypers(t *testing.T, eon, numKeyper, threshold, activationBlock uint) *Keypers {
	// TODO: sanity check threshold
	tkg := testkeygen.NewTestKeyGenerator(t, uint64(numKeyper), uint64(threshold), true)
	// TODO: randomize
	keypers1 := []common.Address{
		common.BigToAddress(big.NewInt(1)),
		common.BigToAddress(big.NewInt(2)),
		common.BigToAddress(big.NewInt(3)),
		common.BigToAddress(big.NewInt(4)),
	}
	return &Keypers{
		t:         t,
		addrs:     keypers1,
		threshold: uint(threshold),
		eon:       eon,
		tkg:       tkg,
	}
}

type Keypers struct {
	t               *testing.T
	addrs           []common.Address
	threshold       uint
	eon             uint
	activationBlock uint
	tkg             *testkeygen.TestKeyGenerator
}

func (k *Keypers) KeyperSet(atBlock uint) *shopevent.KeyperSet {
	return &shopevent.KeyperSet{
		ActivationBlock: uint64(k.activationBlock),
		Members:         k.addrs,
		Threshold:       uint64(k.threshold),
		Eon:             uint64(k.eon),
		AtBlockNumber:   number.BigToBlockNumber(big.NewInt(int64(atBlock))),
	}
}

func (k *Keypers) EonPubkey(atBlock uint) *shopevent.EonPublicKey {
	eonpubkey, err := k.tkg.EonPublicKey(dummyID).GobEncode()
	assert.NilError(k.t, err)
	return &shopevent.EonPublicKey{
		Eon:           uint64(k.eon),
		Key:           eonpubkey,
		AtBlockNumber: number.BigToBlockNumber(big.NewInt(int64(atBlock))),
	}
}

func (k *Keypers) EpochKey(blockNum uint) *p2pmsg.DecryptionKey {
	idt := identitypreimage.Uint64ToIdentityPreimage(uint64(blockNum))
	epoch, err := k.tkg.EpochSecretKey(idt).GobEncode()
	assert.NilError(k.t, err)
	return &p2pmsg.DecryptionKey{
		InstanceID: InstanceID,
		Eon:        uint64(k.eon),
		EpochID:    idt.Bytes(),
		Key:        epoch,
	}
}
