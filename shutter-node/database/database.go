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
	&models.Epoch{},
	&models.State{},
	&models.PublicKey{},
	&models.Keyper{},
}

func (d *Database) DB() *gorm.DB {
	return d.db
}

func (d *Database) Connect(path string) error {
	if path == "" {
		return errors.New("no db path provided")
	}
	// path += "?cache=shared&mode=rwc"
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil
	}
	d.db = db
	idb, err := d.db.DB()
	if err != nil {
		return errors.Wrap(err, "get sql interface db")
	}

	// TODO: enable read/write concurrency
	// PRAGMA journal_mode=WAL;

	// TODO: tweak if necessary, or remove...
	// TODO: we might have to enable concurrency:
	// PRAGMA busy_timeout = milliseconds;
	idb.SetConnMaxIdleTime(500)
	idb.SetMaxOpenConns(20)
	idb.SetMaxIdleConns(10)

	return errors.Wrap(d.AutoMigrate(), "auto migrate database")
}

func (d *Database) Session(ctx context.Context, l log.Logger) *gorm.DB {
	log := NewLogger(l)
	// FIXME: what options?
	config := &gorm.Session{
		NewDB:   true,
		Context: ctx,
		Logger:  log,
	}

	return d.db.Session(config)
}

func (d *Database) Models() []any {
	return modelsV1
}

func (d *Database) Close() error {
	dbSQL, err := d.db.DB()
	if err != nil {
		return err
	}
	return dbSQL.Close()
}

func (d *Database) AutoMigrate() error {
	return d.db.AutoMigrate(d.Models()...)
}
