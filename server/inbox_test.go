package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/storage"
)

func TestGetKey_String(t *testing.T) {
	const testJSON = `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"summary": "Sally followed John",
		"type": "Follow",
		"actor": {
		 "type": "Person",
		 "name": "Sally",
		 "id": "sally"
		},
		"object": {
		 "type": "Person",
		 "name": "John",
		 "id": "john"
		}
	   }`
	var act activity.Activity
	require.NoError(t, json.Unmarshal([]byte(testJSON), &act))
	assert.Equal(t, "sally", parseID(act.Actor))
	assert.Equal(t, "john", parseID(act.Object))
}

type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) FindFollow(id string) (*storage.Follow, error) {
	args := m.Called(id)
	if f, ok := args.Get(0).(*storage.Follow); ok {
		return f, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockDatabase) DeleteFollow(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockDatabase) SaveFollow(f storage.Follow) error {
	args := m.Called(f)
	return args.Error(0)
}

// A complex integration test of the happy path for Follow and Accept logic
func TestInbox_Follow(t *testing.T) {
	pipeline := NewPipeline()
	go pipeline.Run(context.Background())
	defer pipeline.Stop()

	const id = "followed_id"
	const followID = "follow_request_id"
	var remoteID string

	inbox := ActivityInbox{
		id:       "test",
		ownerID:  id,
		pipeline: pipeline,
	}

	// Simulate the remote inbox
	remoteInbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		var act activity.Activity
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&act)
		assert.NoError(t, err)
		assert.Equal(t, followID, parseID(act.Object)) // id of follow request
		assert.Equal(t, id, parseID(act.Actor))        // if of recipient of follow request
	}))
	defer remoteInbox.Close()

	// Simulate the remote actor URL
	remoteActor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := activity.Actor{
			Context: activity.Context,
			Type:    activity.PersonType,
			ID:      remoteID,
			Inbox:   remoteInbox.URL,
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes(&actor))
	}))
	defer remoteActor.Close()

	remoteID = remoteActor.URL

	database := &mockDatabase{}
	database.On("FindFollow", remoteID).Return(nil, nil).Once()
	database.On("SaveFollow", storage.Follow{
		ID:            remoteID,
		RequestID:     followID,
		RequestStatus: "pending",
	}).Return(nil).Once()
	database.On("SaveFollow", storage.Follow{
		ID:            remoteID,
		RequestID:     followID,
		RequestStatus: "accepted",
	}).Return(nil).Once()
	inbox.followers = database

	recorder := httptest.NewRecorder()
	follow := activity.Activity{
		Context: activity.Context,
		Type:    activity.FollowType,
		ID:      followID,
		Actor:   remoteID,
		Object:  id,
	}

	// wrap in a timeout to avoid potential deadlock
	timeout := time.After(3 * time.Second)
	done := make(chan bool)
	go func() {
		inbox.Follow(recorder, follow)
		done <- true
	}()
	select {
	case <-timeout:
		t.Fatal("Test didn't finish")
	case <-done:
		break
	}

	database.AssertExpectations(t)
}

// A complex integration test of the happy path for Undo Follow logic
func TestInbox_UnFollow(t *testing.T) {
	pipeline := NewPipeline()
	go pipeline.Run(context.Background())
	defer pipeline.Stop()

	const id = "followed_id"
	const undoID = "undo_request_id"
	var remoteID string

	inbox := ActivityInbox{
		id:       "test",
		ownerID:  id,
		pipeline: pipeline,
	}

	// Simulate the remote inbox
	remoteInbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		var act activity.Activity
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&act)
		assert.NoError(t, err)
		assert.Equal(t, undoID, parseID(act.Object)) // id of undo request
	}))
	defer remoteInbox.Close()

	// Simulate the remote actor URL
	remoteActor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := activity.Actor{
			Context: activity.Context,
			Type:    activity.PersonType,
			ID:      remoteID,
			Inbox:   remoteInbox.URL,
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes(&actor))
	}))
	defer remoteActor.Close()

	remoteID = remoteActor.URL

	database := &mockDatabase{}
	database.On("DeleteFollow", remoteID).Return(nil, nil).Once()
	inbox.followers = database

	recorder := httptest.NewRecorder()
	follow := activity.Activity{
		Type:   activity.FollowType,
		Actor:  remoteID,
		Object: id,
	}
	undo := activity.Activity{
		Context: activity.Context,
		Type:    activity.UndoType,
		ID:      undoID,
		Actor:   remoteID,
		Object:  follow,
	}

	// wrap in a timeout to avoid potential deadlock
	timeout := time.After(3 * time.Second)
	done := make(chan bool)
	go func() {
		inbox.Unfollow(recorder, undo, follow)
		done <- true
	}()
	select {
	case <-timeout:
		t.Fatal("Test didn't finish")
	case <-done:
		break
	}

	database.AssertExpectations(t)
}
