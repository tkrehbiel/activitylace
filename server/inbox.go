package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/data"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityInbox struct {
	username string
	id       string
	storage  data.Collection
}

// GetHTTP handles GET requests to the inbox, which we don't do
func (ai *ActivityInbox) GetHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "ActivityInbox.ServeHTTP %s", ai.username)
	telemetry.Increment("get_requests", 1)
	collection := activity.OrderedCollection{
		Context:  activity.Context,
		Type:     activity.OrderedCollectionType,
		Identity: ai.id,
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

// PostHTTP handles POST requests to the inbox.
// This is where the bulk of handling communications from remote federated servers happens.
// e.g. Follow requests will come in through here.
func (ai *ActivityInbox) PostHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "ActivityInbox.ServeHTTP %s", ai.username)
	telemetry.Increment("post_requests", 1)

	jsonBytes, err := io.ReadAll(r.Body)
	if err != nil {
		telemetry.Error(err, "reading body bytes")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	obj := data.NewMapObject(jsonBytes)
	err = ai.storage.Upsert(context.TODO(), obj)
	if err != nil {
		telemetry.Error(err, "saving object to database")
	}

	var header activity.ActivityHeader
	err = json.Unmarshal(jsonBytes, &header)
	if err != nil {
		telemetry.Error(err, "unmarshaling json")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch header.Type {
	case "Follow":
		telemetry.Trace("Follow")
		w.WriteHeader(http.StatusMethodNotAllowed)
	case "Undo":
		telemetry.Trace("Undo")
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		// unrecognized Activity Type
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
