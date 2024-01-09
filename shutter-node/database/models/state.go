package models

import (
	"time"

	"gorm.io/gorm"
)

type Block struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Number uint   `gorm:"primarykey"`
	Epoch  *Epoch // 1-to-1 relationship
}

func (k *Block) ModelVersion() uint {
	return 1
}

type State struct {
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	BlockNumber uint           `gorm:"primarykey"`

	// TODO: constraint: only one state can be the latest
	// only when we confirm the block state this latest will get set
	IsLatest  bool
	IsPending bool `gorm:"default:true"`

	// Null values mean this has not been set yet
	EonIndex      *uint
	ShutterActive *bool
}

func (k *State) ModelVersion() uint {
	return 1
}
