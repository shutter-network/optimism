package keys

import (
	"context"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type TestManager struct {
	logger     log.Logger
	newSKChan  chan *NewSecretKey
	newEonChan chan *NewEon
}

func NewTestManager(logger log.Logger) *TestManager {
	return &TestManager{
		newSKChan:  make(chan *NewSecretKey),
		newEonChan: make(chan *NewEon),
	}
}

func (t *TestManager) GetPublicKey(eon uint64) *shcrypto.EonPublicKey {
	return nil
}

func (t *TestManager) IsKeyperInEon(eon uint64, address common.Address) bool {
	return false
}

func (t *TestManager) GetChannelNewSecretKey() chan<- *NewSecretKey {
	return t.newSKChan
}

func (t *TestManager) GetChannelNewEon() chan<- *NewEon {
	return t.newEonChan
}

func (t *TestManager) RequestDecryptionKey(eon uint64,
	batch uint64,
) error {
	return nil
}

func (t *TestManager) pollNewSk(ctx context.Context) error {
	for {
		select {
		case sk, ok := <-t.newSKChan:
			if !ok {
				return nil
			}
			keyBytes, err := sk.SecretKey.GobEncode()
			if err != nil {
				t.logger.Error("couldn't encode secret key")
				continue
			}
			t.logger.Info("got new DecryptionKey", "key", hex.EncodeToString(keyBytes), "epoch", sk.Epoch.String())

		case <-ctx.Done():
			return ctx.Err()

		}
	}
}

func (t *TestManager) teardown() {
	close(t.newSKChan)
	close(t.newEonChan)
}

func (t *TestManager) Start(
	ctx context.Context,
	runner service.Runner,
) error {
	runner.Go(func() error {
		return t.pollNewSk(ctx)
	})
	runner.Defer(t.teardown)
	return nil
}
