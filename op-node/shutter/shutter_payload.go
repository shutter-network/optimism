package shutter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/shutter-node/grpc/v1/client"
)

var ErrShutterFetchKeyTimeout = errors.New("shutter gRPC decryption-key API did not fetch key in time")

func init() {
	k := make([]byte, 128)
	for i := 0; i < len(k); i++ {
		k[i] = math.MaxUint8
	}
	DeactivationDecryptionKey = hexutil.Bytes(k)
}

var (
	nullTime                  time.Time
	ShutterDeadline           = 10 * time.Minute
	DeactivationDecryptionKey hexutil.Bytes
)

type updateEntity uint8

const (
	updateEntityCreated updateEntity = iota
	updateEntityExecutionClient
	updateEntityShutterNode
)

type stateAt struct {
	hash                 common.Hash
	number               uint
	active               bool
	created              time.Time
	touchedByExecClient  time.Time
	touchedByShutterNode time.Time
	lastTouched          updateEntity
}

func (s *stateAt) isTouchedBy(who updateEntity) bool {
	var t time.Time
	switch who {
	case updateEntityCreated:
		t = s.created
	case updateEntityShutterNode:
		t = s.touchedByShutterNode
	case updateEntityExecutionClient:
		t = s.touchedByExecClient
	}
	return !t.Equal(nullTime)
}

func (s *stateAt) touch(who updateEntity) {
	t := time.Now()
	switch who {
	case updateEntityCreated:
		s.created = t
	case updateEntityShutterNode:
		s.touchedByShutterNode = t
	case updateEntityExecutionClient:
		s.touchedByExecClient = t
	}
	s.lastTouched = who
}

func NewEngine(shutter *client.Client) *Engine {
	logger := log.New()
	return &Engine{
		shutter: shutter,
		log:     logger,
		states:  map[common.Hash]*stateAt{},
	}
}

type Engine struct {
	shutter *client.Client
	log     log.Logger
	states  map[common.Hash]*stateAt
}

func (sh *Engine) getStateAt(hash common.Hash) *stateAt {
	state, ok := sh.states[hash]
	if !ok {
		return nil
	}
	return state
}

func (sh *Engine) clearStateAt(hash common.Hash) {
	delete(sh.states, hash)
}

func (sh *Engine) setStateAt(state *stateAt) bool {
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
		} else {
			sh.log.Error("engine API start error",
				"block", l2Parent.Number+1,
				"payload-key", keyString,
				"error-type", errType,
				"error", err,
			)
			// other error, has nothing to do with
			// shutter state
			return
		}
	} else {
		sh.log.Info(
			"engine API start success",
			"block", l2Parent.Number+1,
			"payload-key", keyString,
		)
	}
	state.touch(updateEntityExecutionClient)
}

func (sh *Engine) decideError(state *stateAt, attrs *eth.PayloadAttributes, err error) (*eth.PayloadAttributes, error) {
	// don't ask the engine api again, but try polling the shutter API again
	// on the next action cycle
	if state.isTouchedBy(updateEntityExecutionClient) {
		return nil, derive.NewTemporaryError(
			fmt.Errorf("%s: %s", ErrShutterFetchKeyTimeout, err),
		)
	}
	sh.log.Warn("shutter - got error, but first asking execution client", "error", err)
	// Now if we never before asked the engine-api
	// wether the state is right, we don't want to error.
	// For now assume that shutter might be inactive,
	// but let the engineapi correct us if not
	attrs.DecryptionKey = nil
	state.active = false
	return attrs, nil
}

func (sh *Engine) PreparePayloadAttributes(
	ctx context.Context,
	attrs *eth.PayloadAttributes,
	l2Parent eth.L2BlockRef,
) (*eth.PayloadAttributes, error) {
	state := sh.getStateAt(l2Parent.Hash)
	if state == nil {
		// first time we called for this ref,
		state = &stateAt{
			hash:   l2Parent.Hash,
			number: uint(l2Parent.Number),
			active: false,
		}
		state.touch(updateEntityCreated)
		// check the state before that
		parentState := sh.getStateAt(l2Parent.ParentHash)
		if parentState != nil {
			state.active = parentState.active
		}
		// if parent state was nil, we just assume shutter is inactive,
		// and will get corrected by a payload error if this is not the case
		sh.setStateAt(state)
		// cleanup the map, we won't need this anymore
		sh.clearStateAt(l2Parent.ParentHash)
	}
	if !state.active {
		// assume this is correct and try to post this
		// to the engineAPI
		return attrs, nil
	}

	if state.created.Add(ShutterDeadline).Before(time.Now()) {
		sh.log.Warn("shutter - shutter key-query deadline exceeded, deactivating shutter")
		attrs.DecryptionKey = &DeactivationDecryptionKey
		state.active = false
		return attrs, nil
	}

	// havent been corrected by engine-api, we think the key is still active
	// but the shutter-api is down

	// The shutter API will usually return quickly when it
	// thinks the state is inactive.
	// If it thinks shutter is active, it will block until a key
	// is received.
	// Other reasons for blocking long is an undesired connectivity.
	key, ok := <-sh.shutter.GetKey(ctx, uint(l2Parent.Number+1))
	if !ok {
		// This means either the context timed out,
		// or the connection closed
		err := errors.New("shutter - key promise closed without value")
		return sh.decideError(state, attrs, err)
	}
	// The shutter-node api is not down and returned.
	sh.log.Info("shutter - received key from shutter-node", "key", key)
	if key.Error != nil {
		err := fmt.Errorf("get-key returned with error: %s", key.Error)
		return sh.decideError(state, attrs, err)
	} else {
		hexKey := hexutil.Bytes(key.SecretKey.Marshal())
		if !key.Active {
			if state.isTouchedBy(updateEntityExecutionClient) {
				// if we already got confirmed by the
				// engine API before, we have a mismatch with
				// the shutter-node.
				// We can disable shutter now already,
				// because the API doesn't recover from this.
				sh.log.Warn("shutter - shutter API mismatch, deactivating shutter")
				attrs.DecryptionKey = &DeactivationDecryptionKey
				return attrs, nil
			}
			attrs.DecryptionKey = nil
			state.active = false
		} else {
			// as expected, we are active and we got
			// a key within the timeout
			attrs.DecryptionKey = &hexKey
			sh.log.Info("shutter - got valid key",
				"key", hexKey,
				"active", key.Active,
				"block", key.Block,
			)
		}
		state.touch(updateEntityShutterNode)
	}
	return attrs, nil
}
