package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/data"
	"github.com/tkrehbiel/activitylace/server/rss"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityOutbox struct {
	username string
	id       string
	rssURL   string
	storage  data.Collection
}

// NewItem is called when a new RSS item is detected by the watcher
func (ao *ActivityOutbox) NewItem(item rss.Item) {
	telemetry.Trace("new item [%s]", item.Title)
	telemetry.Increment("rss_newitems", 1)
	obj := &activity.Note{
		Context:   activity.Context,
		Type:      activity.NoteType,
		Identity:  item.ID,
		Content:   item.Title,
		Published: item.Published.Format(activity.TimeFormat),
		URL:       item.URL,
	}
	if err := ao.storage.Upsert(context.TODO(), obj); err != nil {
		telemetry.Error(err, "updating database")
	}
}

// StatusCode is called by the RSS watcher to report the latest fetch status code
func (ao *ActivityOutbox) StatusCode(code int) {
	telemetry.Trace("rss feed return code [%d]", code)
	telemetry.Increment("rss_fetches", 1)
}

func (ao *ActivityOutbox) GetLatestNotes(n int) []activity.Note {
	objects, err := ao.storage.SelectAll(context.TODO())
	if err != nil {
		telemetry.Error(err, "selecting from database")
		return nil
	}

	notes := make([]activity.Note, 0)

	// Take only the last 10
	if len(objects) > 0 {
		for i := len(objects) - 1; len(notes) < n; i-- {
			note := activity.NewNote(objects[i].JSON())
			notes = append(notes, note)
		}

		// sort in reverse chronological order
		sort.Slice(notes, func(a, b int) bool {
			return notes[a].Timestamp().After(notes[b].Timestamp())
		})
	}

	return notes
}

// WatchRSS watches an RSS feed for new items and saves them as ActivityPub objects
func (ao *ActivityOutbox) WatchRSS(ctx context.Context) {
	watcher := rss.NewFeedWatcher(ao.rssURL, ao)

	// Load previously-stored items
	objects, err := ao.storage.SelectAll(context.Background())
	if err == nil {
		for _, obj := range objects {
			note := data.ToNote(obj)
			item := rss.Item{
				ID:        obj.ID(),
				Published: obj.Timestamp(),
				Title:     note.Title,
				Content:   note.Content,
			}
			watcher.AddKnown(item)
		}
	}

	telemetry.Log("watching %s", ao.rssURL)
	watcher.Watch(ctx, 5*time.Minute)
}

func (ao *ActivityOutbox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "ActivityOutbox.ServeHTTP %s", ao.username)
	telemetry.Increment("get_requests", 1)

	notes := ao.GetLatestNotes(10)

	collection := activity.OrderedCollection{
		Context:  activity.Context,
		Type:     activity.OrderedCollectionType,
		Identity: ao.id,
		NumItems: len(notes),
		Items:    notes,
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
