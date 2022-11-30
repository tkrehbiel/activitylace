package storage

import "gorm.io/gorm"

type Follow struct {
	ID         string
	FollowerID string
	TargetID   string
}

type Follows interface {
	FindFollow(id string) (*Follow, error)
	SaveFollow(f *Follow) error
}

func (s *sqliteDatabase) FindFollow(id string) (*Follow, error) {
	var follow Follow
	tx := s.db.Find(&follow, Follow{ID: id})
	if tx.Error == gorm.ErrRecordNotFound {
		return nil, nil
	} else if tx.Error != nil {
		return nil, tx.Error
	}
	return &follow, nil
}

func (s *sqliteDatabase) SaveFollow(f *Follow) error {
	tx := s.db.Save(f)
	return tx.Error
}
