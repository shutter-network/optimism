package identity

import (
	"encoding"
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/identitypreimage"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type Preimage []byte

var (
	ErrByteSizeMismatch    = errors.New("byte-slice length is not 64")
	ErrCopiedBytesMismatch = errors.New("did not copy correct amount of bytes")
)

var (
	_ encoding.BinaryUnmarshaler = &Preimage{}
	_ encoding.BinaryMarshaler   = Preimage{}
)

func (eid *Preimage) UnmarshalBinary(data []byte) error {
	// this might be a noop, but in case anything changes
	// upstream use the conversion
	*eid = Preimage(identitypreimage.IdentityPreimage(data).Bytes())
	return nil
}

func (eid Preimage) MarshalBinary() (data []byte, err error) {
	// this might be a noop, but in case anything changes
	// upstream use the conversion
	return identitypreimage.IdentityPreimage(eid[:]).Bytes(), nil
}

func BytesToPreimage(b []byte) (Preimage, error) {
	eid := new(Preimage)
	err := eid.UnmarshalBinary(b)
	return *eid, err
}

func (eid Preimage) String() string {
	return hex.EncodeToString([]byte(eid))
}

func (eid Preimage) Uint64() uint64 {
	return identitypreimage.IdentityPreimage([]byte(eid)).Uint64()
}

// TODO: use LRU cache
func BlockNumberToEpochID(b uint64) (Preimage, error) {
	preim := identitypreimage.Uint64ToIdentityPreimage(b)
	shEID := shcrypto.ComputeEpochID(preim.Bytes())
	return BytesToPreimage(shEID.Marshal())
}
