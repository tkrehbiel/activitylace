package data

import (
	"encoding/json"
	"log"
	"time"

	"github.com/tkrehbiel/activitylace/server/activity"
)

// ActivityObject is a JSON object with common properties conforming to the ActivityPub spec
type ActivityObject interface {
	JSON() []byte
	ID() string
	Timestamp() time.Time
}

type MapObject struct {
	jsonBytes []byte
	mapData   map[string]interface{}
}

func (o *MapObject) JSON() []byte {
	return o.jsonBytes
}

func (o *MapObject) ID() string {
	if s, ok := o.mapData[activity.IDProperty].(string); ok {
		return s
	}
	return ""
}

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
		log.Println()
		return nil
	}
	return &o
}

func ToNote(obj ActivityObject) activity.Note {
	var note activity.Note
	err := json.Unmarshal(obj.JSON(), &note)
	if err == nil {
		note.JSONBytes = obj.JSON()
	}
	return note
}
