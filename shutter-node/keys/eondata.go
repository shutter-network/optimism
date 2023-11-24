package keys

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

var ErrServerClosedKeyRequest error = errors.New("server closed the key request")

type NewSecretKey struct {
	Eon       uint64
	Epoch     EpochID
	SecretKey *shcrypto.EpochSecretKey
}

type keyRequest struct {
	eon         uint64
	batch       uint64
	epoch       EpochID
	requested   time.Time
	lastChecked time.Time
	promise     chan *KeyRequestResult
}

func newEonData(keypers []common.Address, publicKey *shcrypto.EonPublicKey) *eonData {
	keyperSet := map[common.Address]int{}
	for i, kp := range keypers {
		keyperSet[kp] = i
	}
	return &eonData{
		keyperSet:      keyperSet,
		publicKey:      publicKey,
		decryptionKeys: map[EpochID]*shcrypto.EpochSecretKey{},
		newKey:         make(chan *NewSecretKey, 1),
		requestKey:     make(chan *keyRequest, 1),
	}
}

type eonData struct {
	keyperSet      map[common.Address]int
	publicKey      *shcrypto.EonPublicKey
	decryptionKeys map[EpochID]*shcrypto.EpochSecretKey

	newKey     chan *NewSecretKey
	requestKey chan *keyRequest
}

func errorPromis(req *keyRequest, err error) chan *KeyRequestResult {
	req.promise <- &KeyRequestResult{
		Batch:     req.batch,
		SecretKey: nil,
		Error:     err,
	}
	close(req.promise)
	return req.promise
}

func (e *eonData) processWaitingRequest(req *keyRequest) bool {
	key, ok := e.decryptionKeys[req.epoch]
	if !ok {
		req.lastChecked = time.Now()
		return false
	}
	req.promise <- &KeyRequestResult{
		Batch:     req.batch,
		SecretKey: key,
		Error:     nil,
	}
	close(req.promise)
	return true
}

func (e *eonData) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		return e.eventLoop(ctx)
	},
	)
	return nil
}

func (e *eonData) eventLoop(ctx context.Context) error {
	waitingRequests := []*keyRequest{}
	stop := make(chan error, 1)
	for {
		select {
		case err := <-stop:
			for _, req := range waitingRequests {
				errorPromis(req, ErrServerClosedKeyRequest)
			}
			close(stop)
			return err
		case <-ctx.Done():
			stop <- ctx.Err()
		case req := <-e.requestKey:
			if !e.processWaitingRequest(req) {
				waitingRequests = append(waitingRequests, req)
			}
		case k, ok := <-e.newKey:
			if !ok {
				stop <- nil
			}
			e.decryptionKeys[k.Epoch] = k.SecretKey
			newWaitingRequests := []*keyRequest{}
			for _, req := range waitingRequests {
				if !e.processWaitingRequest(req) {
					newWaitingRequests = append(newWaitingRequests, req)
				}
			}
			waitingRequests = newWaitingRequests
		}
	}
}
