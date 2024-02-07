package keys

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

var ErrRequestCanceled = errors.New("request was cancelled by caller")

func newKeyRequest(block uint, requestedAt time.Time) *keyRequest {
	return &keyRequest{
		block:     block,
		requested: requestedAt,
		promise:   make(chan *KeyRequestResult, 1),
	}
}

type keyRequest struct {
	block       uint
	requested   time.Time
	lastChecked time.Time
	promise     chan *KeyRequestResult
}

func (req *keyRequest) touch() {
	req.lastChecked = time.Now()
}

// NOTE:
// this is potentially blocking, if the channel
// already has a value.
// However we should guarantee that the promises are
// only fulfilled by one synchronized write loop.
// and thus only called once and then the request is
// deleted
func (req *keyRequest) fillPromise(res *KeyRequestResult) <-chan *KeyRequestResult {
	if req.promise == nil {
		return nil
	}
	promise := req.promise
	promise <- res
	close(promise)
	req.promise = nil
	return promise
}

func (req *keyRequest) success(key *shcrypto.EpochSecretKey) <-chan *KeyRequestResult {
	return req.fillPromise(&KeyRequestResult{
		Block:     req.block,
		SecretKey: key,
		Error:     nil,
	})
}

func (req *keyRequest) errorPromise(err error) <-chan *KeyRequestResult {
	return req.fillPromise(&KeyRequestResult{
		Block:     req.block,
		SecretKey: nil,
		Error:     err,
	})
}

func (req *keyRequest) processed() bool {
	return req.promise == nil
}

func (req *keyRequest) cancelRequest(err error) {
	req.errorPromise(errors.Wrap(ErrRequestCanceled, err.Error()))
}

func (req *keyRequest) getPromise() <-chan *KeyRequestResult {
	return req.promise
}

// RequestDecryptionKey does not actively request the decryption at the
// keypers, but it will initiate an internal subscription that
// fulfils as soon as the decryption key was received from the keypers.
// The passed in context is the HTTP-request's context, so when
// the caller closes the connection, the context is canceled.
func (m *manager) RequestDecryptionKey(ctx context.Context, block uint) (<-chan *KeyRequestResult, CancelRequest) {
	req := newKeyRequest(block, time.Now())
	select {
	// ask for registering the request
	case m.newKeyRequest <- req:
		return req.getPromise(), req.cancelRequest
	case <-ctx.Done():
		return req.errorPromise(ErrRequestAborted), req.cancelRequest
	}
}
