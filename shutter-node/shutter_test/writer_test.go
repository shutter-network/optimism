package shutter_test

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum-optimism/optimism/shutter-node/database/query"
	"github.com/pkg/errors"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"gorm.io/gorm"
	"gotest.tools/assert"
)

func TestSimpleInsert(t *testing.T) {
	kpr := NewKeypers(t, 0, 3, 2, 3)
	kprAddrs := kpr.KeyperSet(0).Members

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	tt := Setup(ctx, t)

	tt.Events(
		NewTestEvent("block 0 finalized",
			Block(0),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("initial keyperset known, active block 3",
			kpr.KeyperSet(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 1 finalized",
			Block(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("pubkey keyper-set 0 received",
			kpr.EonPubkey(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 2 finalized",
			Block(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("shutter active in block 4",
			ShutterActive(3),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 3 finalized, keyper set is active now",
			Block(3),
			WithPostCheck(ExpectEventDB()),
			WithFinalCheck(
				LatestState(
					&models.State{
						Metadata: models.Metadata{InsertBlock: 3},
						Block:    3,
						Active:   false,
						Eon: &models.Eon{
							Metadata: models.Metadata{
								InsertBlock: 1,
							},
							EonIndex:        0,
							IsFinalized:     true,
							ActivationBlock: 3,
							Threshold:       2,
							Keypers: []*models.Keyper{
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprAddrs[0],
								},
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprAddrs[1],
								},
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprAddrs[2],
								},
							},
						},
						ActiveUpdate: &models.ActiveUpdate{
							Metadata: models.Metadata{
								InsertBlock: 3,
							},
							Block:  4,
							Active: true,
						},
					})),
		),

		// Stop the handler and all started services
		Close(),
	)

	err := service.Run(ctx, tt)
	assert.NilError(t, err)
}

func TestReorg(t *testing.T) {
	kpr := NewKeypers(t, 0, 3, 2, 3)
	kprAddrs := kpr.KeyperSet(0).Members

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	tt := Setup(ctx, t)

	tt.Events(
		NewTestEvent("block 0 finalized",
			Block(0),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("initial keyperset known, active block 3",
			kpr.KeyperSet(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 1 finalized",
			Block(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("pubkey keyper-set 0 received",
			kpr.EonPubkey(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 2 finalized",
			Block(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("shutter active in block 4",
			ShutterActive(3),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("receive epoch",
			kpr.EpochKey(3, false),
			// epoch should be there after the event is processed
			WithPostCheck(ExpectEventDB()),
			// epoch should not be there anymore after the reorg happened
			WithFinalCheck(
				func(db *gorm.DB, _ *TestEvent) error {
					epoch, err := query.GetEpochForInclusion(db, 3, 0)
					if err != nil {
						return errors.Wrap(err, "retrieve non-existing epoch")
					}
					if epoch != nil {
						return errors.New("retrieved epoch, although reorg should have deleted it")
					}
					return nil
				},
			),
		),
		NewTestEvent("block 3 finalized",
			Block(3),
			WithPostCheck(
				LatestState(
					&models.State{
						Metadata: models.Metadata{InsertBlock: 3},
						Block:    3,
						Active:   false,
						Eon: &models.Eon{
							Metadata: models.Metadata{
								InsertBlock: 1,
							},
							EonIndex:        0,
							IsFinalized:     true,
							ActivationBlock: 3,
							Threshold:       2,
							Keypers: []*models.Keyper{
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprAddrs[0],
								},
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprAddrs[1],
								},
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprAddrs[2],
								},
							},
						},
						ActiveUpdate: &models.ActiveUpdate{
							Metadata: models.Metadata{
								InsertBlock: 3,
							},
							Block:  4,
							Active: true,
						},
					})),
		),
		NewTestEvent("reorg incoming, signaling with parent of reorged latest head",
			Block(0),
		),
		NewTestEvent("block 1 (reorg) finalized, everything deleted",
			Block(1),
			WithPostCheck(ExpectEventDB()),
			WithFinalCheck(
				LatestState(
					&models.State{
						Metadata:     models.Metadata{InsertBlock: 1},
						Block:        1,
						Active:       false,
						ActiveUpdate: nil,
					})),
		),

		// Stop the handler and all started services
		Close(),
	)

	err := service.Run(ctx, tt)
	assert.NilError(t, err)
}

func TestKeyperChange(t *testing.T) {
	kpr := NewKeypers(t, 0, 3, 2, 3)
	kpr2 := NewKeypers(t, 1, 3, 2, 4)
	kprSet := kpr.KeyperSet(1)
	kprSet2 := kpr2.KeyperSet(1)

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	tt := Setup(ctx, t)

	tt.Events(
		NewTestEvent("block 0 finalized",
			Block(0),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("keyperset 0, active block 3",
			kpr.KeyperSet(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("keyperset 1, active block 4",
			kpr2.KeyperSet(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 1 finalized",
			Block(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("pubkey keyper-set 0 received",
			kpr.EonPubkey(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("pubkey keyper-set 1 received",
			kpr2.EonPubkey(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("shutter active in block 3",
			ShutterActive(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 2 finalized",
			Block(2),
			WithPreCheck(
				ExpectError(IsKeyperSetActive(kprSet)),
			),
			WithPostCheck(ExpectEventDB()),
			WithPostCheck(
				IsKeyperSetActive(kprSet),
			),
			WithPostCheck(
				LatestState(
					&models.State{
						Metadata: models.Metadata{InsertBlock: 2},
						Block:    2,
						// The KS 0 should be active now (in pending block 3),
						// and produce decryption keys.
						// This is not reflected in the state,
						// because the latest state does not consider
						// the pending block state
						Eon:    nil,
						Active: false,
						ActiveUpdate: &models.ActiveUpdate{
							Metadata: models.Metadata{
								InsertBlock: 2,
							},
							Block:  3,
							Active: true,
						},
					})),
		),

		// The DBwriter doesn't mind that there is no
		// epoch key from keyper-set 0 for the block
		NewTestEvent("block 3 finalized",
			Block(3),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 4 finalized, keyper set is active now",
			Block(4),
			WithPostCheck(ExpectEventDB()),
			WithFinalCheck(
				LatestState(
					&models.State{
						Metadata: models.Metadata{InsertBlock: 4},
						Block:    4,
						Active:   true,
						Eon: &models.Eon{
							Metadata: models.Metadata{
								InsertBlock: 1,
							},
							EonIndex:        1,
							IsFinalized:     true,
							ActivationBlock: 4,
							Threshold:       2,
							Keypers: []*models.Keyper{
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprSet2.Members[0],
								},
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprSet2.Members[1],
								},
								{
									Metadata: models.Metadata{
										InsertBlock: 1,
									},
									Address: kprSet2.Members[2],
								},
							},
						},
						ActiveUpdate: nil,
					})),
		),

		// Stop the handler and all started services
		Close(),
	)

	err := service.Run(ctx, tt)
	assert.NilError(t, err)
}

func TestEpochInsert(t *testing.T) {
	kpr := NewKeypers(t, 0, 3, 2, 3)

	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	tt := Setup(ctx, t)

	tt.Events(
		NewTestEvent("block 0 finalized",
			Block(0),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("shutter active block 1",
			ShutterActive(0),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("initial keyperset known, active block 3",
			kpr.KeyperSet(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("schedule the decr. key request for block 4",
			DecryptionKeyRequest(4),
			WithFinalCheck(
				KeyRequestExpectResult(
					ctx,
					kpr.EpochKey(4, false),
					4,
					nil,
				)),
		),
		NewTestEvent("block 1 finalized",
			Block(1),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("pubkey keyper-set 0 received",
			kpr.EonPubkey(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 2 finalized",
			Block(2),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 3 finalized, keyper set is active now",
			Block(3),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("receive epoch",
			kpr.EpochKey(4, false),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("receive epoch another time",
			kpr.EpochKey(4, false),
			WithPostCheck(ExpectEventDB()),
		),
		NewTestEvent("block 4 finalized",
			Block(4),
			WithPostCheck(ExpectEventDB()),
		),

		// Stop the handler and all started services
		Close(),
	)

	err := service.Run(ctx, tt)
	assert.NilError(t, err)
}
