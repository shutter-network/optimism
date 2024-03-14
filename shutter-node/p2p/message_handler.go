package p2p

import (
	"context"
	"math"

	"github.com/ethereum/go-ethereum/log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
	"github.com/shutter-network/shutter/shlib/shcrypto"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/database/query"
	"github.com/ethereum-optimism/optimism/shutter-node/database/writer"
	"github.com/ethereum-optimism/optimism/shutter-node/keys/identity"
)

var (
	ErrContainsNoKeys       = errors.New("message contained no decryption key")
	ErrContainsMultipleKeys = errors.New("message contained more than one decryption key")
)

func getVerifyOneKey(msg *p2pmsg.DecryptionKeys) (*p2pmsg.Key, error) {
	switch l := len(msg.Keys); l {
	case 0:
		return nil, ErrContainsNoKeys
	case 1:
		key := msg.Keys[0]
		if key == nil {
			return nil, errors.New("no key in message")
		}
		return key, nil
	default:
		return nil, ErrContainsMultipleKeys
	}
}

func DecryptionKeysEventToModel(decrKeys *p2pmsg.DecryptionKeys) (*models.Epoch, error) {
	key, err := getVerifyOneKey(decrKeys)
	if err != nil {
		return nil, err
	}
	sk, err := key.GetEpochSecretKey()
	if err != nil {
		return nil, err
	}
	idt := key.GetIdentity()
	if idt == nil {
		return nil, errors.New("no identity value")
	}

	idPreim, err := identity.BytesToPreimage(key.Identity)
	if err != nil {
		return nil, err
	}
	epoch := &models.Epoch{
		Metadata: models.Metadata{
			InsertBlock: uint(idPreim.Uint64()),
		},
		EonIndex:  uint(decrKeys.Eon),
		Identity:  &idPreim,
		SecretKey: sk,
		Block:     uint(idPreim.Uint64()),
	}
	return epoch, nil
}

func NewDecryptionKeyHandler(instanceID uint64, writer *writer.DBWriter, logger log.Logger) *DecryptionKeyHandler {
	return &DecryptionKeyHandler{
		InstanceID: instanceID,
		writer:     writer,
		log:        logger,
	}
}

// DecryptionKeyHandler listens for new decryption-keys.
type DecryptionKeyHandler struct {
	InstanceID uint64
	writer     *writer.DBWriter
	log        log.Logger
}

func (h DecryptionKeyHandler) ValidateMessage(ctx context.Context, msg p2pmsg.Message) (pubsub.ValidationResult, error) {
	h.log.Info("received unvalidated message on DecryptionKeyHandler topic")
	decrKeys := msg.(*p2pmsg.DecryptionKeys)
	if decrKeys.GetInstanceID() != h.InstanceID {
		return pubsub.ValidationReject, errors.Errorf("instance ID mismatch (want=%d, have=%d)", h.InstanceID, decrKeys.GetInstanceID())
	}
	if decrKeys.Eon > math.MaxInt64 {
		return pubsub.ValidationReject, errors.Errorf("eon %d overflows int64", decrKeys.Eon)
	}

	epochSK, err := getVerifyOneKey(decrKeys)
	if err != nil {
		return pubsub.ValidationReject, err
	}

	epochSecretKey, err := epochSK.GetEpochSecretKey()
	if err != nil {
		return pubsub.ValidationReject, err
	}
	identity := epochSK.GetIdentity()
	if identity == nil {
		return pubsub.ValidationReject, errors.New("no identity")
	}
	// This does only validate that we know of "some" publickey belonging to a keyperset
	// that will result in a successful roundtrip encryption.
	// At this point we don't check that the keyperset is a currently active one
	// or wether shutter is enabled at all
	eonIndex := uint(decrKeys.Eon)

	// create a new session for each handler call
	// NOTE: This will be created for every receiving decryption key.
	// Especially when multiple keypers are emitting the key, this might
	// be called frequently in a short amount of time
	db := h.writer.Session(ctx, h.log)
	pk, err := query.GetPubKey(db, eonIndex)
	if err != nil {
		return pubsub.ValidationReject, err
	}
	if pk == nil || pk.Key == nil {
		return pubsub.ValidationReject, errors.Errorf("no public-key known for eon %d", decrKeys.Eon)
	}

	eon, err := query.GetEonByIndex(db, eonIndex)
	if err != nil {
		return pubsub.ValidationReject, errors.Errorf("eon %d not retrievable", decrKeys.Eon)
	}
	if eon == nil {
		return pubsub.ValidationReject, errors.Errorf("no eon %d known", decrKeys.Eon)
	}

	// TODO: check for keyper set membership of the sender.
	// FIXME: currently we can't do this, because the DecryptionKey message is not signed!

	ok, err := shcrypto.VerifyEpochSecretKey(epochSecretKey, pk.Key, identity)
	if err != nil {
		return pubsub.ValidationReject, errors.Wrapf(err, "error while checking epoch secret key for epoch %v", epochSK.Identity)
	}
	if !ok {
		return pubsub.ValidationReject, errors.Wrapf(err, "epoch secret key for epoch %v is not valid", epochSK.Identity)
	}
	return pubsub.ValidationAccept, nil
}

func (h *DecryptionKeyHandler) HandleMessage(
	ctx context.Context,
	msg p2pmsg.Message,
) ([]p2pmsg.Message, error) {
	decrKeys := msg.(*p2pmsg.DecryptionKeys)
	epoch, err := DecryptionKeysEventToModel(decrKeys)
	if err != nil {
		return nil, errors.Wrap(err, "decode message to model")
	}
	h.log.Info("received decryption-key message",
		"reveal-block", epoch,
		"message", decrKeys.LogInfo(),
	)
	// this can be blocking until there is a slot for writing
	// to the DB
	err = h.writer.HandleNewEpoch(ctx, epoch)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (DecryptionKeyHandler) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{
		&p2pmsg.DecryptionKeys{},
	}
}
