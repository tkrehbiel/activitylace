package server

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/rss"
	"github.com/tkrehbiel/activitylace/server/storage"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityOutbox struct {
	service        *ActivityService
	ownerID        string
	id             string
	rssURL         string
	notes          storage.Notes
	followers      storage.Followers
	pipeline       *OutputPipeline
	privKey        crypto.PrivateKey
	pubKeyID       string
	acceptUnsigned bool
	sendUnsigned   bool
}

// NewItem is called when a new RSS item is detected by the watcher
func (ao *ActivityOutbox) NewItem(item rss.Item) {
	telemetry.Trace("new item [%s]", item.Title)
	telemetry.Increment("rss_newitems", 1)
	obj := storage.Note{
		ID:        item.ID,
		Content:   item.Title,
		Published: item.Published,
	}
	if err := ao.notes.SaveNote(&obj); err != nil {
		telemetry.Error(err, "updating storage for [%s]", item.ID)
	}
	ao.SendToFollowers(obj)
}

func (ao *ActivityOutbox) SendToFollowers(obj storage.Note) {
	// Send note to followers
	users, err := ao.followers.GetFollowers()
	if err != nil {
		telemetry.Error(err, "getting followers")
		return
	}
	for _, follower := range users {
		ao.SendToFollower(obj, follower)
	}
}

func (ao *ActivityOutbox) SendToFollower(obj storage.Note, follower storage.Follow) {
	// Queue an Accept response.
	// We have to do that outside this function or else race conditions.
	telemetry.Trace("queuing an accept response")
	ao.pipeline.Queue(&NoteActivity{
		service:  ao.service,
		outbox:   ao,
		note:     obj,
		remoteID: follower.ID,
		localID:  ao.ownerID,
	})

}

type NoteActivity struct {
	service  *ActivityService
	outbox   *ActivityOutbox
	note     storage.Note
	localID  string
	remoteID string
}

func (f *NoteActivity) String() string {
	return fmt.Sprintf("Note to %s", f.remoteID)
}

func (f *NoteActivity) Prepare(pipeline *OutputPipeline) (*http.Request, error) {
	// Lookup the follower's inbox
	remote, err := f.service.GetActor(f.remoteID)
	if err != nil {
		return nil, fmt.Errorf("looking up remote actor: %w", err)
	}

	activityID := uuid.NewString() // honestly don't care, this is a one-way transaction

	noteObject := struct {
		Context string        `json:"@context"`
		Type    string        `json:"type"`
		ID      string        `json:"id"`
		Actor   string        `json:"actor"`
		Object  activity.Note `json:"object"`
		To      []string      `json:"to"`
		CC      []string      `json:"cc"`
	}{
		Context: activity.Context,
		Type:    activity.CreateType,
		ID:      activityID,
		Actor:   f.localID,
		To:      []string{f.remoteID}, // Pleroma seems to require a to array
		CC:      make([]string, 0),    // Pleroma seems to require a cc array
		Object: activity.Note{
			Type:      activity.NoteType,
			ID:        f.note.ID,
			Content:   f.note.Content,
			Published: f.note.Published.Format(activity.TimeFormat),
		},
	}

	r, err := f.service.ActivityRequest(http.MethodPost, remote.Inbox, &noteObject)
	if err != nil {
		return nil, fmt.Errorf("creating accept request: %w", err)
	}

	if f.outbox.privKey != nil && !f.outbox.sendUnsigned {
		sign(f.outbox.privKey, f.outbox.pubKeyID, r)
	}

	telemetry.Increment("notes_sent", 1)
	return r, nil
}

func (f *NoteActivity) Receive(resp *http.Response) {
	telemetry.Trace("received response from note %d", resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		telemetry.Increment("notes_succeeded", 1)
	} else {
		telemetry.Increment("notes_failed", 1)
	}
}

// StatusCode is called by the RSS watcher to report the latest fetch status code
func (ao *ActivityOutbox) StatusCode(code int) {
	telemetry.Trace("rss feed return code [%d]", code)
	telemetry.Increment("rss_fetches", 1)
}

func (ao *ActivityOutbox) GetLatestNotes(n int) []storage.Note {
	if ao.notes == nil {
		telemetry.Error(nil, "note storage not configured")
		return nil
	}
	notes, err := ao.notes.GetLatestNotes(n)
	if err != nil {
		telemetry.Error(err, "selecting from database")
		return nil
	}
	return notes
}

// WatchRSS watches an RSS feed for new items and saves them as ActivityPub objects
func (ao *ActivityOutbox) WatchRSS(ctx context.Context) {
	watcher := rss.NewFeedWatcher(ao.rssURL, ao)

	// Load previously-stored items
	notes, err := ao.notes.GetLatestNotes(100)
	if err == nil {
		for _, note := range notes {
			item := rss.Item{
				ID:        note.ID,
				Published: note.Published,
				Content:   note.Content,
			}
			watcher.AddKnown(item)
		}
	}

	telemetry.Log("watching [%s]", ao.rssURL)
	watcher.Watch(ctx, 5*time.Minute)
}

func (ao *ActivityOutbox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "ActivityOutbox.ServeHTTP %s", ao.ownerID)
	telemetry.Increment("get_requests", 1)

	notes := ao.GetLatestNotes(10)

	items := make([]activity.Note, len(notes))
	for i, note := range notes {
		items[i] = activity.Note{
			Type:      activity.NoteType,
			ID:        note.ID,
			Published: note.Published.Format(activity.TimeFormat),
			Content:   note.Content,
		}
	}

	collection := activity.OrderedNoteCollection{
		Context:  activity.Context,
		Type:     activity.OrderedCollectionType,
		ID:       ao.id,
		NumItems: len(items),
		Items:    items,
	}

	jsonBytes, err := json.Marshal(&collection)
	if err != nil {
		telemetry.Error(err, "marshaling collection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", activity.ContentTypeLD)
	w.Write(jsonBytes)
}
