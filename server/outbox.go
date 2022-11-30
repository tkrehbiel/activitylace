package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/rss"
	"github.com/tkrehbiel/activitylace/server/storage"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityOutbox struct {
	username string
	id       string
	rssURL   string
	notes    storage.Notes
}

// NewItem is called when a new RSS item is detected by the watcher
func (ao *ActivityOutbox) NewItem(item rss.Item) {
	telemetry.Trace("new item [%s]", item.Title)
	telemetry.Increment("rss_newitems", 1)
	obj := &storage.Note{
		ID:        item.ID,
		Content:   item.Title,
		Published: item.Published,
	}
	if err := ao.notes.SaveNote(obj); err != nil {
		telemetry.Error(err, "updating storage for [%s]", item.ID)
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
	telemetry.Request(r, "ActivityOutbox.ServeHTTP %s", ao.username)
	telemetry.Increment("get_requests", 1)

	notes := ao.GetLatestNotes(10)

	type noteObject struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Published string `json:"published"`
		Title     string `json:"title,omitempty"`
		Content   string `json:"content,omitempty"`
		URL       string `json:"url,omitempty"`
	}
	items := make([]noteObject, len(notes))
	for i, note := range notes {
		items[i] = noteObject{
			Type:      activity.NoteType,
			ID:        note.ID,
			Published: note.Published.Format(activity.TimeFormat),
			Content:   note.Content,
		}
	}

	collection := struct {
		Context  string       `json:"@context"`
		Type     string       `json:"type"`
		ID       string       `json:"id"`
		NumItems int          `json:"numItems"`
		Items    []noteObject `json:"items"`
	}{
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
	w.Header().Set("Content-Type", activity.ContentType)
	w.Write(jsonBytes)
}
