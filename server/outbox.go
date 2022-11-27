package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/data"
	"github.com/tkrehbiel/activitylace/server/rss"
)

type ActivityOutbox struct {
	username string
	id       string
	rssURL   string
	storage  data.Collection
}

// NewItem is called when a new RSS item is detected by the watcher
func (ao *ActivityOutbox) NewItem(item rss.Item) {
	log.Println("new item", item.Title)
	obj := &activity.Note{
		Context:   activity.Context,
		Type:      activity.NoteType,
		Identity:  item.ID,
		Content:   item.Title,
		Published: item.Published.Format(activity.TimeFormat),
		URL:       item.URL,
	}
	if err := ao.storage.Upsert(context.TODO(), obj); err != nil {
		log.Println("updating database", err)
	}
}

// StatusCode is called by the RSS watcher to report the latest fetch status code
func (ao *ActivityOutbox) StatusCode(code int) {
	log.Println("feed return code", code)
}

// WatchRSS watches an RSS feed for new items and saves them as ActivityPub objects
func (ao *ActivityOutbox) WatchRSS(ctx context.Context) {
	err := ao.storage.Open()
	if err != nil {
		log.Println("opening database", err)
		return
	}
	defer ao.storage.Close()

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

	log.Println("watching", ao.rssURL)
	watcher.Watch(ctx, 5*time.Minute)
}

func (ao *ActivityOutbox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logRequest("ActivityOutbox.ServeHTTP", r)

	objects, err := ao.storage.SelectAll(context.TODO())
	if err != nil {
		log.Println("selecting from database", err) // TODO: need to sort out these log messages
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection := activity.OrderedCollection{
		Context:  activity.Context,
		Type:     activity.OrderedCollectionType,
		Identity: ao.id,
		NumItems: len(objects),
		Items:    make([]activity.Note, 0),
	}

	// Take only the last 10
	for i := len(objects) - 1; len(collection.Items) <= 10; i-- {
		note := activity.NewNote(objects[i].JSON())
		collection.Items = append(collection.Items, note)
	}

	// sort in reverse chronological order
	sort.Slice(collection.Items, func(a, b int) bool {
		return collection.Items[a].Timestamp().After(collection.Items[b].Timestamp())
	})

	jsonBytes, err := json.Marshal(&collection)
	if err != nil {
		log.Println("marshaling collection", err) // TODO: need to sort out these log messages
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("ContentType", activity.ContentType)
	w.Write(jsonBytes)
}

func logRequest(message string, r *http.Request) {
	log.Println(message, r.URL.String())
}
