package storage

import (
	"time"

	"gorm.io/gorm"
)

// Note represents an ORM object to store local or remote note
type Note struct {
	ID        string    `json:"id"`
	Published time.Time `json:"published"`
	Content   string    `json:"content"`
	URL       string    `json:"url"`
	Source    string    // json source
}

type Notes interface {
	GetLatestNotes(n int) ([]Note, error)
	FindNote(id string) (*Note, error)
	SaveNote(n *Note) error
}

func (s *sqliteDatabase) GetLatestNotes(n int) (notes []Note, err error) {
	tx := s.db.Order("published desc").Limit(n).Find(&notes)
	if tx.Error == gorm.ErrRecordNotFound {
		return nil, nil
	} else if tx.Error != nil {
		return nil, tx.Error
	}
	return notes, nil
}

func (s *sqliteDatabase) FindNote(id string) (*Note, error) {
	var note Note
	tx := s.db.First(&note, Note{ID: id})
	if tx.Error == gorm.ErrRecordNotFound {
		return nil, nil
	} else if tx.Error != nil {
		return nil, tx.Error
	}
	return &note, nil
}

func (s *sqliteDatabase) SaveNote(n *Note) error {
	tx := s.db.Save(n)
	return tx.Error
}
