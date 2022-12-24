package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/karlseguin/ccache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/rss"
	"github.com/tkrehbiel/activitylace/server/storage"
)

func TestOutbox_NoteActivity(t *testing.T) {
	pipeline := NewPipeline()
	go pipeline.Run(context.Background())
	defer pipeline.Stop()

	const id = "local_id"
	var remoteID string

	svc := ActivityService{
		actorCache: ccache.New(ccache.Configure[activity.Actor]()),
	}

	outbox := ActivityOutbox{
		service:  &svc,
		ownerID:  id,
		pipeline: pipeline,
	}

	testItem := rss.Item{
		ID:        "itemid",
		Title:     "itemtitle",
		Content:   "itemcontent",
		Published: time.Now().UTC(),
		URL:       "itemurl",
	}
	testItem2 := rss.Item{
		ID:        "itemid2",
		Title:     "itemtitle2",
		Content:   "itemcontent2",
		Published: time.Now().UTC(),
		URL:       "itemurl2",
	}

	// Simulate the remote inbox
	remoteInbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		assert.Equal(t, "POST", r.Method)
		var act activity.Activity
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&act)
		assert.NoError(t, err)
		assert.Equal(t, activity.CreateType, act.Type)
		assert.NotEmpty(t, act.ID)
		if note, ok := act.Object.(map[string]interface{}); ok {
			assert.Equal(t, activity.NoteType, note["type"])
			assert.Equal(t, "text/plain", note["mediaType"])
		} else {
			assert.True(t, false, "could not convert note")
		}
	}))
	defer remoteInbox.Close()

	// Simulate the remote actor URL
	remoteActor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
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

	followerList := []storage.Follow{
		{
			ID:            remoteID,
			RequestID:     "", // doesn't matter
			RequestStatus: "", // doesn't matter
		},
	}

	// Should get list of followers from storage
	followerDB := &mockFollowers{}
	followerDB.On("GetFollowers").Return(followerList, nil).Twice()
	outbox.followers = followerDB

	// Should save a note to storage
	notesDB := &mockNotes{}
	notesDB.On("SaveNote", &storage.Note{
		ID:        testItem.ID,
		Published: testItem.Published,
		Content:   testItem.Title,
		URL:       testItem.URL,
	}).Return(nil).Once()
	notesDB.On("SaveNote", &storage.Note{
		ID:        testItem2.ID,
		Published: testItem2.Published,
		Content:   testItem2.Title,
		URL:       testItem2.URL,
	}).Return(nil).Once()
	outbox.notes = notesDB

	// wrap in a timeout to avoid potential deadlock
	timeout := time.After(3 * time.Second)
	done := make(chan bool)
	go func() {
		outbox.NewItem(testItem)
		outbox.NewItem(testItem2)
		done <- true
	}()
	select {
	case <-timeout:
		t.Fatal("Test didn't finish")
	case <-done:
		break
	}

	pipeline.Flush() // wait for queued requests to finish

	followerDB.AssertExpectations(t)
}
