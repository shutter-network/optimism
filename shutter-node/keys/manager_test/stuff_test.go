package manager_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	shopevent "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/optimism/sync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/number"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gorm.io/gorm"
	"gotest.tools/assert"
)

// *shopevent.KeyperSet:
// *shopevent.EonPublicKey:
// *shopevent.LatestBlock:
func Block(num uint) *shopevent.LatestBlock {
	bn := big.NewInt(int64(num))
	return &shopevent.LatestBlock{
		Number:    number.BigToBlockNumber(bn),
		BlockHash: common.BigToHash(bn),
	}
}

func ShutterActive(atBlock uint) *shopevent.ShutterState {
	n := uint64(atBlock)
	return &shopevent.ShutterState{
		Active:        true,
		AtBlockNumber: number.NewBlockNumber(&n),
	}
}

func ShutterInactive(atBlock uint) *shopevent.ShutterState {
	n := uint64(atBlock)
	return &shopevent.ShutterState{
		Active:        false,
		AtBlockNumber: number.NewBlockNumber(&n),
	}
}

func KeyRequestExpectResult(ctx context.Context, key []byte, batch uint, err error) CheckFunction {
	return func(db *gorm.DB, ev *TestEvent) error {
		res, err := ev.WaitResult(ctx)
		if err != nil {
			return err
		}
		krr, ok := res.(*keys.KeyRequestResult)
		if !ok {
			return err
		}
		exKey := &shcrypto.EpochSecretKey{}
		err = exKey.GobDecode(key)
		if err != nil {
			return err
		}
		expected := &keys.KeyRequestResult{
			Batch:     uint64(batch),
			SecretKey: exKey,
			Error:     err,
		}

		if cmp.Equal(krr, expected) {
			return nil
		}
		// TODO: compare expected
		cmp.Diff(krr, expected)
		return errors.New("not equal")
	}
}

func TestSimple(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	kpr := NewKeypers(t, 0, 4, 3, 3)
	tt := Setup(ctx, t)
	tt.Events(
		NewTestEvent(
			ShutterActive(1),
		),
		NewTestEvent(
			Block(1),
		),
		NewTestEvent(
			kpr.KeyperSet(1),
		),
		NewTestEvent(
			Block(2),
		),
		NewTestEvent(
			kpr.EonPubkey(2),
		),
		NewTestEvent(
			DecryptionKeyRequest(3),
			WithFinalCheck(
				KeyRequestExpectResult(ctx, []byte{}, 3, nil))),
		NewTestEvent(
			kpr.EpochKey(3),
		),
		NewTestEvent(
			Block(3),
		),
	)
	err := service.Run(ctx, tt)
	assert.NilError(t, err)
}
