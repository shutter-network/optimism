package keys

import (
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/identitypreimage"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type EpochID [64]byte

var (
	ErrByteSizeMismatch    = errors.New("byte-slice length is not 64")
	ErrCopiedBytesMismatch = errors.New("did not copy correct amount of bytes")
)

func BytesToEpochID(b []byte) (EpochID, error) {
	var eid EpochID
	if len(b) != cap(eid) {
		return eid, ErrByteSizeMismatch
	}
	cpd := copy(eid[:], b)
	if cpd != cap(eid) {
		return eid, ErrCopiedBytesMismatch
	}
	return eid, nil
}

// TODO: use LRU cache
func BatchNumberToEpochID(b uint64) (EpochID, error) {
	preim := identitypreimage.Uint64ToIdentityPreimage(b)
	shEID := shcrypto.ComputeEpochID(preim.Bytes())
	return BytesToEpochID(shEID.Marshal())
}
