package p2p

import (
	"context"
	"math"

	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"github.com/shutter-network/shutter/shlib/shcrypto"

	"github.com/ethereum-optimism/optimism/shutter-node/keys"
)

func NewDecryptionKeyHandler(instanceID uint64, manager keys.Manager) *DecryptionKeyHandler {
	c := manager.GetChannelNewSecretKey()
	return &DecryptionKeyHandler{
		InstanceID:   instanceID,
		Manager:      manager,
		newSecretKey: c,
	}
}

// DecryptionKeyHandler listens for new decryption-keys.
type DecryptionKeyHandler struct {
	InstanceID   uint64
	Manager      keys.Manager
	newSecretKey chan<- *keys.NewSecretKey
}

func (h DecryptionKeyHandler) ValidateMessage(_ context.Context, msg p2pmsg.Message) (bool, error) {
	key := msg.(*p2pmsg.DecryptionKey)
	if key.GetInstanceID() != h.InstanceID {
		return false, errors.Errorf("instance ID mismatch (want=%d, have=%d)", h.InstanceID, key.GetInstanceID())
	}
	if key.Eon > math.MaxInt64 {
		return false, errors.Errorf("eon %d overflows int64", key.Eon)
	}

	// TODO: check for keyper set membership of the sender.
	// FIXME: we can't do this, because the DecryptionKey message is not signed!
	// h.KeyperSetManager.IsKeyperForEon(key)

	epochSecretKey, err := key.GetEpochSecretKey()
	if err != nil {
		return false, err
	}
	publicKey := h.Manager.GetPublicKey(key.Eon)
	if publicKey == nil {
		return false, errors.Errorf("no public-key known for eon %d", key.Eon)
	}

	ok, err := shcrypto.VerifyEpochSecretKey(epochSecretKey, publicKey, key.EpochID)
	if err != nil {
		return false, errors.Wrapf(err, "error while checking epoch secret key for epoch %v", key.EpochID)
	}
	if !ok {
		return false, errors.Wrapf(err, "epoch secret key for epoch %v is not valid", key.EpochID)
	}
	return true, nil
}

func (h *DecryptionKeyHandler) HandleMessage(
	ctx context.Context,
	msg p2pmsg.Message,
) ([]p2pmsg.Message, error) {
	key := msg.(*p2pmsg.DecryptionKey)
	sk, err := key.GetEpochSecretKey()
	if err != nil {
		// we did this already in the validator,
		// so this shouldn't happen
		return nil, err
	}

	epochID, err := keys.BytesToEpochID(key.EpochID)
	if err != nil {
		return nil, err
	}

	select {
	case h.newSecretKey <- &keys.NewSecretKey{
		Eon:       key.Eon,
		Epoch:     epochID,
		SecretKey: sk,
	}:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (DecryptionKeyHandler) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{
		&p2pmsg.DecryptionKey{},
	}
}
