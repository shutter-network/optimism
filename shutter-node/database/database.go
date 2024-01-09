package database

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ethereum-optimism/optimism/shutter-node/database/models"
	"github.com/ethereum/go-ethereum/log"
)

type Database struct {
	db *gorm.DB
}

var modelsV1 []any = []any{
	&models.Eon{},
	&models.Keyper{},
	&models.Epoch{},
	&models.State{},
}

func (d *Database) Connect(path string) error {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil
	}
	d.db = db
	return errors.Wrap(d.AutoMigrate(), "auto migrate database")
}

func (d *Database) Session(ctx context.Context, l log.Logger) *gorm.DB {
	log := NewLogger(l)
	config := &gorm.Session{
		NewDB:       true,
		Initialized: true,
		Context:     ctx,
		Logger:      log,
	}
	return d.db.Session(config)
}

func (d *Database) Models() []any {
	return modelsV1
}

func (d *Database) AutoMigrate() error {
	return d.db.AutoMigrate(d.Models()...)
}
