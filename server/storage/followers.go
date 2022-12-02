package storage

import "gorm.io/gorm"

type Follow struct {
	ID            string
	RequestID     string
	RequestStatus string // pending or accepted
}

type Followers interface {
	FindFollow(id string) (*Follow, error)
	DeleteFollow(id string) error
	SaveFollow(f Follow) error
}

func (s *sqliteDatabase) FindFollow(id string) (*Follow, error) {
	var follow Follow
	tx := s.db.First(&follow, &Follow{ID: id})
	if tx.Error == gorm.ErrRecordNotFound {
		return nil, nil
	} else if tx.Error != nil {
		return nil, tx.Error
	}
	return &follow, nil
}

func (s *sqliteDatabase) DeleteFollow(id string) error {
	tx := s.db.Delete(&Follow{ID: id})
	if tx.Error != nil && tx.Error == gorm.ErrRecordNotFound {
		return tx.Error
	}
	return nil
}

func (s *sqliteDatabase) SaveFollow(f Follow) error {
	tx := s.db.Save(&f)
	return tx.Error
}
