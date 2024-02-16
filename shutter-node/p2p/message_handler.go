package p2p

import (
	"context"
	"math"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/keys"
	"github.com/ethereum-optimism/optimism/shutter-node/keys/identity"
)

func NewDecryptionKeyHandler(instanceID uint64, manager keys.Manager, logger log.Logger) *DecryptionKeyHandler {
	c := manager.GetChannelNewEpoch()
	return &DecryptionKeyHandler{
		InstanceID:   instanceID,
		Manager:      manager,
		newSecretKey: c,
		log:          logger,
	}
}

// DecryptionKeyHandler listens for new decryption-keys.
type DecryptionKeyHandler struct {
	InstanceID   uint64
	Manager      keys.Manager
	newSecretKey chan<- *models.Epoch
	log          log.Logger
}

func (h DecryptionKeyHandler) ValidateMessage(_ context.Context, msg p2pmsg.Message) (pubsub.ValidationResult, error) {
	// NOCHECKIN:
	h.log.Info("received unvalidated message on DecryptionKeyHandler topic")
	key := msg.(*p2pmsg.DecryptionKey)
	if key.GetInstanceID() != h.InstanceID {
		return pubsub.ValidationReject, errors.Errorf("instance ID mismatch (want=%d, have=%d)", h.InstanceID, key.GetInstanceID())
	}
	if key.Eon > math.MaxInt64 {
		return pubsub.ValidationReject, errors.Errorf("eon %d overflows int64", key.Eon)
	}

	// TODO: check for keyper set membership of the sender.
	// FIXME: we can't do this, because the DecryptionKey message is not signed!
	// h.KeyperSetManager.IsKeyperForEon(key)

	epochSecretKey, err := key.GetEpochSecretKey()
	if err != nil {
		return pubsub.ValidationReject, err
	}
	epochId := key.GetEpochID()
	h.log.Info("received decryption key", "eon", key.GetEon(), "epoch-id", hexutil.Encode(epochId))
	_ = epochSecretKey
	return pubsub.ValidationAccept, nil
	// TODO: validate that we fulfill all eon preconditions for this..

	// publicKey := h.Manager.GetPublicKey(key.Eon)
	// if publicKey == nil {
	// 	return pubsub.ValidationReject, errors.Errorf("no public-key known for eon %d", key.Eon)
	// }

	// ok, err := shcrypto.VerifyEpochSecretKey(epochSecretKey, publicKey, key.EpochID)
	// if err != nil {
	// 	return pubsub.ValidationReject, errors.Wrapf(err, "error while checking epoch secret key for epoch %v", key.EpochID)
	// }
	// if !ok {
	// 	return pubsub.ValidationReject, errors.Wrapf(err, "epoch secret key for epoch %v is not valid", key.EpochID)
	// }
	// return pubsub.ValidationAccept, nil
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
	idPreim, err := identity.BytesToPreimage(key.EpochID)
	h.log.Info("handling decryption-key", "key", hexutil.Encode(sk.Marshal()), "epoch", idPreim.String())
	if err != nil {
		return nil, err
	}
	// this is only partially filled with data
	_ = &models.Epoch{
		Eon: models.Eon{
			Index: uint(key.Eon),
		},
		Identity:  idPreim,
		SecretKey: sk,
	}
	return nil, nil
}

func (DecryptionKeyHandler) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{
		&p2pmsg.DecryptionKey{},
	}
}
