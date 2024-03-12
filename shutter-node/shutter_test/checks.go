package shutter_test

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/database/query"
	"github.com/ethereum-optimism/optimism/shutter-node/database/writer"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	syncevent "github.com/shutter-network/rolling-shutter/rolling-shutter/medley/chainsync/event"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/number"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"gorm.io/gorm"
)

var (
	ErrObjNotInDB            = errors.New("object not in DB")
	ErrTestEventNotSupported = errors.New("test-event not supported by check function")
)

func Block(num uint) *syncevent.LatestBlock {
	bn := big.NewInt(int64(num))
	return &syncevent.LatestBlock{
		Number:    number.BigToBlockNumber(bn),
		BlockHash: common.BigToHash(bn),
	}
}

func ShutterActive(atBlock uint) *syncevent.ShutterState {
	n := uint64(atBlock)
	return &syncevent.ShutterState{
		Active:        true,
		AtBlockNumber: number.NewBlockNumber(&n),
	}
}

func ShutterInactive(atBlock uint) *syncevent.ShutterState {
	n := uint64(atBlock)
	return &syncevent.ShutterState{
		Active:        false,
		AtBlockNumber: number.NewBlockNumber(&n),
	}
}

func KeyRequestExpectResult(ctx context.Context, p2pKeys *p2pmsg.DecryptionKeys, block uint, err error) CheckFunction {
	return func(db *gorm.DB, ev *TestEvent) error {
		fmt.Printf("expect result, event:%v", ev)
		// FIXME: the insert epoch test waits undefinetely here
		// TODO: timeout here!
		// Mainly this is used as a final test, where no events will
		// get processed anymore anyways. The only thing running here
		// is the key-manager loop polling the db.
		ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()

		res, err := ev.WaitResult(ctx)
		if err != nil {
			return err
		}
		krr, ok := res.(*keys.KeyRequestResult)
		if !ok {
			return errors.New("result type not expected")
		}
		k := p2pKeys.GetKeys()
		if len(k) != 1 {
			return errors.New("expected p2p decryption-keys message malformed")
		}
		sk, err := k[0].GetEpochSecretKey()
		if err != nil {
			return err
		}
		expected := &keys.KeyRequestResult{
			Block:     block,
			SecretKey: sk,
			Error:     err,
		}

		err = IsEqual(krr, expected)
		if err != nil {
			return err
		}
		return nil
	}
}

func DefaultCmpOpts() []cmp.Option {
	// NOTE: this is susceptible to field / type renames!
	return []cmp.Option{
		cmpopts.IgnoreUnexported(),
		cmpopts.IgnoreFields(models.State{}, "EonID", "ActiveUpdateID"),
		cmpopts.IgnoreFields(models.Metadata{}, "ID", "CreatedAt", "UpdatedAt", "DeletedAt"),
		cmpopts.IgnoreFields(models.Keyper{}, "Eons"),
		CompareAddress(),
	}
}

func CompareAddress() cmp.Option {
	return cmp.Comparer(
		func(a, b common.Address) bool {
			return a.Cmp(b) == 0
		})
}

func LatestState(state *models.State) CheckFunction {
	return func(db *gorm.DB, _ *TestEvent) error {
		queried, err := query.GetLatestState(db)
		if err != nil {
			return errors.Wrap(err, "query state")
		}
		if queried == nil {
			return errors.New("state not found")
		}
		if err := IsEqual(state, queried); err != nil {
			return errors.Wrap(err, ErrObjNotInDB.Error())
		}
		return nil
	}
}

func IsEqual(x, y any) error {
	diff := cmp.Diff(
		x,
		y,
		DefaultCmpOpts()...,
	)
	if diff == "" {
		return nil
	}
	return errors.Errorf("objects not equal: %s", diff)
}

func ExpectError(fn CheckFunction) CheckFunction {
	return func(db *gorm.DB, ev *TestEvent) error {
		err := fn(db, ev)
		if err == nil {
			return errors.New("error in check-function expected")
		}
		return nil
	}
}

func IsKeyperSetActive(ks *syncevent.KeyperSet) CheckFunction {
	return func(db *gorm.DB, _ *TestEvent) error {
		// search by nearest activation block
		//
		latestBlock, err := query.GetLatestBlock(db)
		if err != nil || latestBlock == nil {
			return errors.Wrap(err, "retrieve latest block")
		}
		// pending: latestBlock + 1
		queried, err := query.GetEonForBlock(db, *latestBlock+1)
		if err != nil {
			return err
		}
		if queried == nil {
			return ErrObjNotInDB
		}
		expected, err := writer.KeyperSetEventToModel(ks)
		if err != nil {
			return errors.Wrap(err, "derive expected model")
		}
		if err := IsEqual(expected, queried); err != nil {
			return errors.Wrap(err, ErrObjNotInDB.Error())
		}
		return nil
	}
}

func ExpectEventDB() CheckFunction {
	return func(db *gorm.DB, ev *TestEvent) error {
		switch t := ev.Value.(type) {
		case *syncevent.ShutterState:
			block, err := t.AtBlockNumber.ToUInt64()
			if err != nil {
				return errors.Wrap(err, "convert block")
			}
			queried, err := query.GetActiveUpdate(db, uint(block))
			if err != nil {
				return err
			}
			if queried == nil {
				return ErrObjNotInDB
			}
			expected, err := writer.ShutterStateToActive(t)
			if err != nil {
				return errors.Wrap(err, "derive expected model")
			}
			if err := IsEqual(expected, queried); err != nil {
				return errors.Wrap(err, ErrObjNotInDB.Error())
			}

		case *syncevent.LatestBlock:
			block, err := t.Number.ToUInt64()
			if err != nil {
				return errors.Wrap(err, "convert block")
			}
			queried, err := query.GetState(db, uint(block))
			if err != nil {
				return err
			}
			if queried == nil {
				return ErrObjNotInDB
			}
			expected, err := writer.LatestBlockEventToModel(t)
			if err != nil {
				return errors.Wrap(err, "derive expected model")
			}
			// here we only want to assert that there is a block.
			// state comparisons have to be made explicit
			if expected.Block != queried.Block {
				return errors.Wrapf(ErrObjNotInDB, "expected block %d, got block %d as latest state", expected.Block, queried.Block)
			}
		case *syncevent.EonPublicKey:
			queried, err := query.GetPubKey(db, uint(t.Eon))
			if err != nil {
				return err
			}
			if queried == nil {
				return ErrObjNotInDB
			}
			expected, err := writer.PublicKeyEventToModel(t)
			if err != nil {
				return errors.Wrap(err, "derive expected model")
			}
			if err := IsEqual(expected, queried); err != nil {
				return errors.Wrap(err, ErrObjNotInDB.Error())
			}
		case *syncevent.KeyperSet:
			queried, err := query.GetEonByIndex(db, uint(t.Eon))
			if err != nil {
				return err
			}
			if queried == nil {
				return ErrObjNotInDB
			}
			expected, err := writer.KeyperSetEventToModel(t)
			if err != nil {
				return errors.Wrap(err, "derive expected model")
			}
			if err := IsEqual(expected, queried); err != nil {
				return errors.Wrap(err, ErrObjNotInDB.Error())
			}
		case *p2pmsg.DecryptionKeys:
			expected, err := p2p.DecryptionKeysEventToModel(t)
			if err != nil {
				return errors.Wrap(err, "derive expected model")
			}
			// XXX:
			queried, err := query.GetEpochForInclusion(db, expected.Block, uint(t.Eon))
			if err != nil {
				return err
			}
			if queried == nil {
				return ErrObjNotInDB
			}
			if err := IsEqual(expected, queried); err != nil {
				return errors.Wrap(err, ErrObjNotInDB.Error())
			}
		default:
			return ErrTestEventNotSupported
		}
		return nil
	}
}
