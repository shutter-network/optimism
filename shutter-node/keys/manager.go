package keys

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type Manager interface {
	// TODO: the getters require a mutex...?
	// at least for retrieving the eonData from the dict,
	// under the assumption that the keypers and keys can't
	// change anymore
	GetPublicKey(eon uint64) *shcrypto.EonPublicKey
	IsKeyperInEon(eon uint64, address common.Address) bool

	GetChannelNewSecretKey() chan<- *NewSecretKey
	GetChannelNewEon() chan<- *NewEon

	RequestDecryptionKey(
		eon uint64,
		batch uint64,
	) error
}

type KeyRequestResult struct {
	Batch     uint64
	SecretKey *shcrypto.EpochSecretKey
	Error     error
}

type NewEon struct {
	Eon       uint64
	Keypers   []common.Address
	PublicKey *shcrypto.EonPublicKey
}

func newManager() *manager {
	return &manager{
		eonDataMux:  sync.RWMutex{},
		eonData:     map[uint64]*eonData{},
		newKey:      make(chan *NewSecretKey, 1),
		newEon:      make(chan *NewEon, 1),
		keyRequests: make(chan *keyRequest, 1),
	}
}

type manager struct {
	eonDataMux sync.RWMutex
	eonData    map[uint64]*eonData

	newKey      chan *NewSecretKey
	newEon      chan *NewEon
	keyRequests chan *keyRequest
}

var (
	ErrNoEonForBatch  = errors.New("no eon found for batch")
	ErrRequestAborted = errors.New("request was aborted")
)

func (m *manager) eonForBatch(batch uint64) (uint64, error) {
	// TODO: infer the eon for the requested batch
	// based on L2 onchain contract data
	return 1, nil
}

func (m *manager) processDecryptionKeyRequest(ctx context.Context, batch uint64) <-chan *KeyRequestResult {
	return nil
}

// TODO: maybe return error when eon doesn't exist
func (m *manager) GetPublicKey(eon uint64) *shcrypto.EonPublicKey {
	m.eonDataMux.RLock()
	defer m.eonDataMux.RUnlock()
	ed, ok := m.eonData[eon]
	if !ok {
		return nil
	}
	return ed.publicKey
}

// TODO: maybe return error when eon doesn't exist
func (m *manager) IsKeyperInEon(eon uint64, address common.Address) bool {
	m.eonDataMux.RLock()
	defer m.eonDataMux.RUnlock()
	ed, ok := m.eonData[eon]
	if !ok {
		return false
	}
	_, isKeyper := ed.keyperSet[address]
	return isKeyper
}

func (m *manager) GetChannelNewSecretKey() chan<- *NewSecretKey {
	return m.newKey
}

func (m *manager) GetChannelNewEon() chan<- *NewEon {
	return m.newEon
}

// RequestDecryptionKey does not actively request the decryption at the
// keypers, but it will initiate an internal subscription that
// fulfils as soon as the decryption key was received from the keypers.
func (m *manager) RequestDecryptionKey(ctx context.Context, batch uint64) <-chan *KeyRequestResult {
	req := &keyRequest{
		batch:     batch,
		requested: time.Now(),
		promise:   make(chan *KeyRequestResult, 1),
	}
	eon, err := m.eonForBatch(batch)
	if errors.Is(err, ErrNoEonForBatch) {
		err = errors.Errorf("no eon known for batch '%d'", batch)
	}
	req.eon = eon
	if err != nil {
		return errorPromis(req, err)
	}
	epochID, err := BatchNumberToEpochID(batch)
	if err != nil {
		return errorPromis(req, err)
	}
	req.epoch = epochID

	select {
	case m.keyRequests <- req:
		return req.promise
	case <-ctx.Done():
		return errorPromis(req, ErrRequestAborted)
	}
}

func (m *manager) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		return m.eventLoop(ctx)
	},
	)
	return nil
}

func (m *manager) eventLoop(ctx context.Context) error {
	stop := make(chan error, 1)
	for {
		select {
		case err := <-stop:
			close(stop)
			return err
		case <-ctx.Done():
			stop <- ctx.Err()
		case req := <-m.keyRequests:
			d, ok := m.eonData[req.eon]
			if !ok {
				// XXX: internal server error..
				errorPromis(req, ErrNoEonForBatch)
				continue
			}
			// XXX: can this be long blocking?
			d.requestKey <- req
		case e, ok := <-m.newEon:
			if !ok {
				stop <- nil
			}
			_, exists := m.eonData[e.Eon]
			if exists {
				// XXX: what to do? can this happen on the contract?
				continue
			}
			m.eonData[e.Eon] = newEonData(e.Keypers, e.PublicKey)
		}
	}
}
