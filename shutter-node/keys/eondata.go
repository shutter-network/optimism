package keys

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/keys/identity"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"gorm.io/gorm"
)

var ErrServerClosedKeyRequest error = errors.New("server closed the key request")

type keyRequest struct {
	eon         uint64
	batch       uint64
	epoch       identity.Preimage
	requested   time.Time
	lastChecked time.Time
	promise     chan *KeyRequestResult
}

func newEonData() *KeyRequestHandler {
	return &KeyRequestHandler{
		newEpoch:    make(chan *models.Epoch, 1),
		keyRequests: make(chan *keyRequest, 10),
	}
}

type KeyRequestHandler struct {
	newEpoch    chan *models.Epoch
	keyRequests chan *keyRequest
	db          *gorm.DB
}

func errorPromise(req *keyRequest, err error) chan *KeyRequestResult {
	req.promise <- &KeyRequestResult{
		Batch:     req.batch,
		SecretKey: nil,
		Error:     err,
	}
	close(req.promise)
	return req.promise
}

func (e *KeyRequestHandler) processWaitingRequest(req *keyRequest) bool {
	// XXX: poll db
	// key, ok := e.decryptionKeys[req.epoch]
	//
	// req.batch
	// if !ok {
	// 	req.lastChecked = time.Now()
	// 	return false
	// }
	// req.promise <- &KeyRequestResult{
	// 	Batch:     req.batch,
	// 	SecretKey: key,
	// 	Error:     nil,
	// }
	// close(req.promise)
	return true
}

func (e *KeyRequestHandler) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		return e.eventLoop(ctx)
	},
	)
	return nil
}

// RequestDecryptionKey does not actively request the decryption at the
// keypers, but it will initiate an internal subscription that
// fulfils as soon as the decryption key was received from the keypers.
func (k *KeyRequestHandler) RequestDecryptionKey(ctx context.Context, batch uint64) <-chan *KeyRequestResult {
	req := &keyRequest{
		batch:     batch,
		requested: time.Now(),
		promise:   make(chan *KeyRequestResult, 1),
	}
	eon, err := k.eonForBatch(batch)
	if errors.Is(err, ErrNoEonForBatch) {
		err = errors.Errorf("no eon known for batch '%d'", batch)
	}
	req.eon = eon
	if err != nil {
		return errorPromise(req, err)
	}
	epochID, err := identity.BatchNumberToEpochID(batch)
	if err != nil {
		return errorPromise(req, err)
	}
	req.epoch = epochID

	select {
	case k.keyRequests <- req:
		return req.promise
	case <-ctx.Done():
		return errorPromise(req, ErrRequestAborted)
	}
}

func (k *KeyRequestHandler) eonForBatch(batch uint64) (uint64, error) {
	// TODO: infer the eon for the requested batch
	// based on L2 onchain contract data
	return 1, nil
}

func (e *KeyRequestHandler) eventLoop(ctx context.Context) error {
	waitingRequests := []*keyRequest{}
	stop := make(chan error, 1)
	for {
		select {
		case err := <-stop:
			for _, req := range waitingRequests {
				errorPromise(req, ErrServerClosedKeyRequest)
			}
			close(stop)
			return err
		case <-ctx.Done():
			stop <- ctx.Err()
		case req := <-e.keyRequests:
			if !e.processWaitingRequest(req) {
				waitingRequests = append(waitingRequests, req)
			}
		case k, ok := <-e.newEpoch:
			if !ok {
				stop <- nil
			}

			// e.decryptionKeys[k.Identity] = k.SecretKey
			_ = k

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
