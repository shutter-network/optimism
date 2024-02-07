package models

// ActiveUpdate is the "paused" / "unpaused"
// update event.
// It will get emitted in block Metadata.InsertBlock,
// but the state it represents ("ActiveUpdate") will
// only take effect at block Block,
// or Metadata.InsertBlock + 1 respectively
type ActiveUpdate struct {
	Metadata
	Block uint `gorm:"uniqueIndex"`

	Active bool
}

func (k *ActiveUpdate) ModelVersion() uint {
	return 1
}

type State struct {
	Metadata
	Block uint `gorm:"uniqueIndex"`

	Eon   *Eon
	EonID *uint

	Active bool

	ActiveUpdate   ActiveUpdate
	ActiveUpdateID uint
}

func (k *State) ModelVersion() uint {
	return 1
}
