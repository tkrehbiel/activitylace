package data

import (
	"encoding/json"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

// ActivityObject is a largely immutable object with common properties conforming to the ActivityPub spec.
// The key feature is that the JSON representation remains immutable, which allows us to store
// ActivityPub objects exactly as they are sent to us.
// It isn't intended to be able to read all the properties of the object through this interface,
// it's more for loading and saving the object to persistent storage.
type ActivityObject interface {
	JSON() []byte
	ID() string
	Timestamp() time.Time
}

// MapObject represents JSON objects as an unmarshalled map
type MapObject struct {
	jsonBytes []byte
	mapData   map[string]interface{} // TODO: turns out, not used anywhere, so delete
}

// JSON returns the original JSON of the object
func (o *MapObject) JSON() []byte {
	return o.jsonBytes
}

// ID returns the ActivityPub id property of the object or a zero value
func (o *MapObject) ID() string {
	if s, ok := o.mapData[activity.IDProperty].(string); ok {
		return s
	}
	return ""
}

// Timestamp returns the ActivityPub published property of the object or a zero value
func (o *MapObject) Timestamp() time.Time {
	if s, ok := o.mapData[activity.PublishedProperty].(string); ok {
		if t, err := time.Parse(activity.TimeFormat, s); err != nil {
			return t
		}
	}
	return time.Time{}
}

func NewMapObject(b []byte) ActivityObject {
	o := MapObject{
		jsonBytes: b,
	}
	err := json.Unmarshal(b, &o.mapData)
	if err != nil {
		// TODO: should return an error
		telemetry.Error(err, "unmarshaling MapObject json %s", string(b))
		return nil
	}
	return &o
}

func ToNote(obj ActivityObject) activity.Note {
	var note activity.Note
	err := json.Unmarshal(obj.JSON(), &note)
	if err != nil {
		// TODO: handle better
		telemetry.Error(err, "unmarshaling Note json %s", string(obj.JSON()))
	}
	note.JSONBytes = obj.JSON()
	return note
}
