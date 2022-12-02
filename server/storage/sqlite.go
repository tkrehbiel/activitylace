package storage

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database interface {
	Open() error
	Close()
}

// sqliteDatabase is a collection of ActivityObjects backed by a sqlite database
type sqliteDatabase struct {
	Actors
	Notes
	Followers
	connection string
	db         *gorm.DB
	sqldb      *sql.DB
}

func (s *sqliteDatabase) Open() error {
	if s.db != nil {
		s.Close()
	}
	newLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,  // Slow SQL threshold
			LogLevel:                  logger.Error, // Log level
			IgnoreRecordNotFoundError: true,         // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,        // Disable color
		},
	)
	db, err := gorm.Open(sqlite.Open(s.connection), &gorm.Config{
		Logger: newLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return err
	}
	s.sqldb, err = db.DB()
	if err != nil {
		return err
	}
	s.db = db
	// create tables
	s.db.Migrator().AutoMigrate(&Actor{})
	s.db.Migrator().AutoMigrate(&Note{})
	s.db.Migrator().AutoMigrate(&Follow{})
	return nil
}

func (s *sqliteDatabase) Close() {
	if s.db != nil {
		s.sqldb.Close()
		s.sqldb = nil
		s.db = nil
	}
}

func NewDatabase(connection string) Database {
	return &sqliteDatabase{
		connection: connection,
	}
}
