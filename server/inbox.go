package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/storage"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityInbox struct {
	id        string
	ownerID   string // id of the owner of the inbox
	followers storage.Followers
}

// GetHTTP handles GET requests to the inbox, which we don't do
func (ai *ActivityInbox) GetHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "ActivityInbox.ServeHTTP [%s]", ai.id)
	telemetry.Increment("get_requests", 1)
	collection := activity.OrderedNoteCollection{
		Context: activity.Context,
		Type:    activity.OrderedCollectionType,
		ID:      ai.id,
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
	telemetry.Request(r, "ActivityInbox.ServeHTTP %s", ai.id)
	telemetry.Increment("post_requests", 1)

	jsonBytes, err := io.ReadAll(io.LimitReader(r.Body, 4000))
	if err != nil {
		telemetry.Error(err, "reading body bytes")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var act activity.Activity
	if err := json.Unmarshal(jsonBytes, &act); err != nil {
		telemetry.Error(err, "unmarshaling activity [%s]", string(jsonBytes))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch act.Type {
	case "Follow":
		ai.Follow(w, act, jsonBytes)
	case "Undo":
		telemetry.Trace("Undo")
		w.WriteHeader(http.StatusMethodNotAllowed)
	default:
		// unrecognized Activity Type
		telemetry.Trace("unrecognized activity type [%s] %s", act.Type, string(jsonBytes))
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (ai *ActivityInbox) Follow(w http.ResponseWriter, act activity.Activity, jsonBytes []byte) {
	// Expecting:
	// actor = account that wants to follow
	// object = account to follow

	telemetry.Increment("follow_requests", 1)

	// The actor is the id of the person who wants to follow
	actorID := getKey(act.Actor, "id")

	// The object is the user that is to be followed. It should match the owner of the Inbox.
	objectID := getKey(act.Object, "id")

	var message = fmt.Sprintf("POST follow [%s] by [%s] at inbox [%s]", objectID, actorID, ai.id)
	defer func() {
		telemetry.Log(message)
	}()

	if objectID != ai.ownerID {
		// Trying to follow someone other than the owner of this inbox, not allowed.
		// #ActivityPub There is no information about what to do in this situation in the spec.
		message += " - rejected, wrong inbox"
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if act.ID == "" {
		// No ID provided. #ActivityPub does not describe what to do in this situation.
		// We make one up by combining the follower and the followee.
		act.ID = strings.Join([]string{objectID, actorID}, "-")
	}

	existing, err := ai.followers.FindFollow(act.ID)
	if err != nil {
		message += " - rejected, database read error"
		telemetry.Error(err, "database error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// TODO: why is this always finding an existing row even if the table is empty??
	if existing != nil {
		// Already following, no need to do anything.
		// #ActivityPub says nothing about what to do in this situation. Just winging it.
		message += " - already following"
		w.WriteHeader(http.StatusOK)
		return
	}

	follow := storage.Follow{
		ID:         act.ID,
		FollowerID: actorID,
	}
	if err := ai.followers.SaveFollow(&follow); err != nil {
		message += " - rejected, database write error"
		telemetry.Error(err, "database error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// The #ActivityPub spec isn't telling me what to return on a successful follow, so I guess just OK?
	message += " - success"
	w.WriteHeader(http.StatusOK)
}

func getKey(v interface{}, key string) (val string) {
	// Rather annoyingly, JSON-LD parameters could be a simple string or they can be expansive maps,
	// so we should be prepared to handle either situation. A valid JSON-LD object could be very complex,
	// which, if I can get on my soapbox for a moment, is a deficiency of the [ActivityPub] spec.
	// Nobody likes to deal with data types that are actually variants in the data interchange game.
	// This simple implementation is to try to avoid having to include a full-blown LSON-LD implementation just for corner cases.
	switch t := v.(type) {
	case string:
		// The data is a string, so we can just return it.
		// e.g. { "actor": "https://id" }
		val = t
	case map[string]interface{}:
		// The data is a map, so we need to retrieve one of the keys.
		// e.g. { "actor": { "name": "Alice", "id": "https://id" } }
		switch s := t[key].(type) {
		case string:
			val = s
		case fmt.Stringer:
			val = s.String()
		}
	}
	return val
}
