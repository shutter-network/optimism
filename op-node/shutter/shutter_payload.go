package shutter

import (
	"context"
	"errors"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/shutter-node/grpc/v1/client"
)

type stateAt struct {
	hash   common.Hash
	number uint
	active bool
}

func NewEngine(shutter *client.Client) *Engine {
	logger := log.New()
	return &Engine{
		shutter: shutter,
		log:     logger,
		states:  map[common.Hash]stateAt{},
	}
}

type Engine struct {
	shutter *client.Client
	log     log.Logger
	states  map[common.Hash]stateAt
}

func (sh *Engine) getStateAt(hash common.Hash) *stateAt {
	state, ok := sh.states[hash]
	if !ok {
		return nil
	}
	return &state
}

func (sh *Engine) clearStateAt(hash common.Hash) {
	delete(sh.states, hash)
}

func (sh *Engine) setStateAt(state stateAt) bool {
	_, ok := sh.states[state.hash]
	sh.states[state.hash] = state
	return ok
}

var ErrInvalidShutterState = errors.New("shutter state invalid")

func (sh *Engine) RegisterPayloadResult(
	errType derive.BlockInsertionErrType,
	err error,
	l2Parent eth.L2BlockRef,
	attrs *eth.PayloadAttributes,
) {
	// we did save this state just before
	state := sh.getStateAt(l2Parent.Hash)
	keyString := ""
	if attrs.DecryptionKey != nil {
		keyString = attrs.DecryptionKey.String()
	}
	if err != nil {
		if errType == derive.BlockShutterStateInvalidErr {
			sh.log.Error("engine API returned invalid shutter state",
				"block", l2Parent.Number+1,
				"payload-key", keyString,
			)
			// toggle the state
			state.active = !state.active
			sh.setStateAt(*state)
		} else {
			sh.log.Error("engine API start error",
				"block", l2Parent.Number+1,
				"payload-key", keyString,
				"error-type", errType,
				"error", err,
			)
		}
	} else {
		sh.log.Info(
			"engine API start success",
			"block", l2Parent.Number+1,
			"payload-key", keyString,
		)
	}
}

// TODO: add the active deactivation of shutter
func (sh *Engine) PreparePayloadAttributes(
	ctx context.Context,
	attrs *eth.PayloadAttributes,
	l2Parent eth.L2BlockRef,
	epoch eth.BlockID,
) (*eth.PayloadAttributes, error) {
	state := sh.getStateAt(l2Parent.Hash)
	if state == nil {
		// first time we called for this ref,
		state = &stateAt{
			hash:   l2Parent.Hash,
			number: uint(l2Parent.Number),
			active: false,
		}
		// check the state before that
		parentState := sh.getStateAt(l2Parent.ParentHash)
		if parentState != nil {
			state.active = parentState.active
		}
		// if parent state was nil, we just assume shutter is inactive,
		// and will get corrected by a payload error if this is not the case
		sh.setStateAt(*state)
		// cleanup the map, we won't need this anymore
		sh.clearStateAt(l2Parent.ParentHash)
	}
	if !state.active {
		return attrs, nil
	}
	keyPromise := sh.shutter.GetKey(ctx, uint(l2Parent.Number+1))
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case key := <-keyPromise:
		sh.log.Info("shutter: received key from shutter-node", "key", key)
		if key.Error != nil {
			sh.log.Info("shutter: key has error", "error", key.Error)
		} else {
			hexKey := hexutil.Bytes(key.SecretKey.Marshal())
			attrs.DecryptionKey = &hexKey
			sh.log.Info("shutter: got valid key",
				"key", hexKey,
				"active", key.Active,
				"block", key.Block,
			)
		}
	}
	return attrs, nil
}
