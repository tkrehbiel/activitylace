package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	_ "github.com/mattn/go-sqlite3"
	"github.com/tkrehbiel/activitylace/server/activity"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Collection is an interface to a persistent data store for a collection of ActivityObjects
type Collection interface {
	Open() error
	Close()
	SelectAll(context.Context) ([]ActivityObject, error)
	Upsert(context.Context, ActivityObject) error
}

// sqliteCollection is a collection of ActivityObjects backed by a sqlite database
type sqliteCollection struct {
	name       string // collection name mainly for error messages
	connection string
	db         *gorm.DB
	sqldb      *sql.DB
}

// activityObject is the gorm model for a database row
type activityObject struct {
	ID           uint
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ActivityID   string `gorm:"index;unique"`
	ActivityTime time.Time
	ActivityJSON string
}

func (s *sqliteCollection) Open() error {
	if s.db != nil {
		s.Close()
	}
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
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
	s.db.Migrator().AutoMigrate(&activityObject{})
	return nil
}

func (s *sqliteCollection) Close() {
	if s.db != nil {
		s.sqldb.Close()
		s.sqldb = nil
		s.db = nil
	}
}

func (s *sqliteCollection) Upsert(ctx context.Context, obj ActivityObject) error {
	if s.db == nil {
		return fmt.Errorf("collection %s has not been opened", s.name)
	}
	id := obj.ID()
	var row activityObject
	tx := s.db.WithContext(ctx).Where(&activityObject{ActivityID: id}).First(&row)
	if tx.Error == nil {
		// found, update the row
		row.ActivityTime = obj.Timestamp()
		row.ActivityJSON = string(obj.JSON())
		tx := s.db.WithContext(ctx).Save(&row)
		if tx.Error != nil {
			return fmt.Errorf("error updating %s object ID %s: %w", s.name, id, tx.Error)
		}
		return nil
	} else if tx.Error == gorm.ErrRecordNotFound {
		// not found, insert a new row
		tx := s.db.Create(&activityObject{
			ActivityID:   id,
			ActivityTime: obj.Timestamp(),
			ActivityJSON: string(obj.JSON()),
		})
		if tx.Error != nil {
			return fmt.Errorf("error creating %s object ID %s: %w", s.name, id, tx.Error)
		}
		return nil
	}
	// database error
	return fmt.Errorf("error finding object ID %s: %w", id, tx.Error)
}

func (s *sqliteCollection) SelectAll(ctx context.Context) ([]ActivityObject, error) {
	if s.db == nil {
		return nil, fmt.Errorf("collection %s has not been opened", s.name)
	}
	var rows []activityObject
	tx := s.db.Find(&rows)
	if tx.Error == nil {
		// found rows
		objects := make([]ActivityObject, len(rows))
		for i, row := range rows {
			objects[i] = NewMapObject([]byte(row.ActivityJSON))
		}
		return objects, nil
	} else if tx.Error == gorm.ErrRecordNotFound {
		// no rows found, not really an error
		return nil, nil
	}
	// database error
	return nil, fmt.Errorf("database error in %s: %w", s.name, tx.Error)
}

func (o activityObject) ToNote() activity.Note {
	var note activity.Note
	err := json.Unmarshal([]byte(o.ActivityJSON), &note)
	if err == nil {
		note.JSONBytes = []byte(o.ActivityJSON)
	}
	return note
}

func NewSQLiteCollection(name string, connection string) Collection {
	return &sqliteCollection{
		name:       name,
		connection: connection,
	}
}
