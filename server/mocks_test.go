package server

import (
	"github.com/stretchr/testify/mock"
	"github.com/tkrehbiel/activitylace/server/storage"
)

type mockFollowers struct {
	mock.Mock
}

func (m *mockFollowers) GetFollowers() ([]storage.Follow, error) {
	args := m.Called()
	if l, ok := args.Get(0).([]storage.Follow); ok {
		return l, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockFollowers) FindFollow(id string) (*storage.Follow, error) {
	args := m.Called(id)
	if f, ok := args.Get(0).(*storage.Follow); ok {
		return f, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockFollowers) DeleteFollow(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockFollowers) SaveFollow(f storage.Follow) error {
	args := m.Called(f)
	return args.Error(0)
}

type mockNotes struct {
	mock.Mock
}

func (m *mockNotes) GetLatestNotes(n int) (notes []storage.Note, err error) {
	args := m.Called(n)
	if l, ok := args.Get(0).([]storage.Note); ok {
		return l, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockNotes) FindNote(id string) (*storage.Note, error) {
	args := m.Called(id)
	if n, ok := args.Get(0).(*storage.Note); ok {
		return n, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockNotes) SaveNote(n *storage.Note) error {
	args := m.Called(n)
	return args.Error(0)
}
