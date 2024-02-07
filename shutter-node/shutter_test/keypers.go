package shutter_test

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/number"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/identitypreimage"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/testkeygen"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"gotest.tools/assert"
)

var dummyID = identitypreimage.BigToIdentityPreimage(common.Big0)

var rng = rand.New(rand.NewSource(42))

func NewKeypers(t *testing.T, eon, numKeyper, threshold, activationBlock uint) *Keypers {
	tkg := testkeygen.NewTestKeyGenerator(t, uint64(numKeyper), uint64(threshold), true)
	keypers := []common.Address{}
	for i := 0; i < int(numKeyper); i++ {
		keypers = append(keypers, common.BigToAddress(big.NewInt(rng.Int63())))
	}
	return &Keypers{
		t:               t,
		addrs:           keypers,
		activationBlock: activationBlock,
		threshold:       uint(threshold),
		eon:             eon,
		tkg:             tkg,
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

func (k *Keypers) KeyperSet(atBlock uint) *syncevent.KeyperSet {
	return &syncevent.KeyperSet{
		ActivationBlock: uint64(k.activationBlock),
		Members:         k.addrs,
		Threshold:       uint64(k.threshold),
		Eon:             uint64(k.eon),
		AtBlockNumber:   number.BigToBlockNumber(big.NewInt(int64(atBlock))),
	}
}

func (k *Keypers) EonPubkey(atBlock uint) *syncevent.EonPublicKey {
	eonpubkey, err := k.tkg.EonPublicKey(dummyID).GobEncode()
	assert.NilError(k.t, err)
	return &syncevent.EonPublicKey{
		Eon:           uint64(k.eon),
		Key:           eonpubkey,
		AtBlockNumber: number.BigToBlockNumber(big.NewInt(int64(atBlock))),
	}
}

func (k *Keypers) EpochKey(blockNum uint, wrongKey bool) *p2pmsg.DecryptionKeys {
	idt := identitypreimage.Uint64ToIdentityPreimage(uint64(blockNum))
	keygenIdt := idt
	if wrongKey {
		keygenIdt = identitypreimage.Uint64ToIdentityPreimage(uint64(blockNum + 1))
	}
	epochSk, err := k.tkg.EpochSecretKey(keygenIdt).GobEncode()
	assert.NilError(k.t, err)
	key := &p2pmsg.Key{
		Identity: idt.Bytes(),
		Key:      epochSk,
	}
	return &p2pmsg.DecryptionKeys{
		InstanceID: InstanceID,
		Eon:        uint64(k.eon),
		Keys:       []*p2pmsg.Key{key},
	}
}
