package activity

import (
	"encoding/json"
	"time"

	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type PersistedObject struct {
	JSONBytes []byte `json:"-"`
}

type Note struct {
	PersistedObject
	Context   string `json:"@context,omitempty"`
	Type      string `json:"type"`
	Identity  string `json:"id"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content,omitempty"`
	Published string `json:"published"`
	URL       string `json:"url"` // plain url string
}

type NoteConverter interface {
	ToNote() Note
}

func (o *Note) JSON() []byte {
	if o.JSONBytes == nil {
		b, err := json.Marshal(o)
		if err != nil {
			// TODO: should return an error
			telemetry.Error(err, "marshaling Note json")
			return nil
		}
		o.JSONBytes = b
	}
	return o.JSONBytes
}

func (o *Note) ID() string {
	return o.Identity
}

func (o *Note) Timestamp() time.Time {
	if t, err := time.Parse(TimeFormat, o.Published); err == nil {
		return t
	}
	return time.Time{}
}

func NewNote(jsonBytes []byte) Note {
	var note Note
	err := json.Unmarshal(jsonBytes, &note)
	if err != nil {
		telemetry.Error(err, "unmarshaling Note json %s", string(jsonBytes)) // TODO: handle better
	}
	note.JSONBytes = jsonBytes
	return note
}
