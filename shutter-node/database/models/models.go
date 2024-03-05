package models

import (
	"time"

	"gorm.io/gorm"
)

type Model interface {
	ModelVersion() uint
}

// defines no primarykey, this has to be done
// on the model
type Metadata struct {
	ID          uint `gorm:"primarykey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	InsertBlock uint
}
