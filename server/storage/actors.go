package storage

import "gorm.io/gorm"

// Actor represents an ORM object for a _remote_ actor, not a local one
type Actor struct {
	ID          string
	DisplayName string
	Server      string
	Source      string // json source
}

type Actors interface {
	FindActor(id string) (*Actor, error)
	SaveActor(a *Actor) error
}

func (s *sqliteDatabase) FindActor(id string) (*Actor, error) {
	var actor Actor
	tx := s.db.First(&actor, Actor{ID: id})
	if tx.Error == gorm.ErrRecordNotFound {
		return nil, nil
	} else if tx.Error != nil {
		return nil, tx.Error
	}
	return &actor, nil
}

func (s *sqliteDatabase) SaveActor(a *Actor) error {
	tx := s.db.Save(a)
	return tx.Error
}
