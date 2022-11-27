package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tkrehbiel/activitylace/server/activity"
)

func TestActivityObject_ID(t *testing.T) {
	expected := "https://github.com"
	j := MapObject{
		mapData: map[string]interface{}{
			activity.IDProperty: expected,
		},
	}
	assert.Equal(t, expected, j.ID())
}

func TestActivityObject_ID_FailsUnmarshal(t *testing.T) {
	j := MapObject{
		mapData: map[string]interface{}{
			"notid": "123",
		},
	}
	assert.Empty(t, j.ID())
}

func TestActivityObject_ID_FailsNotString(t *testing.T) {
	j := MapObject{
		mapData: map[string]interface{}{
			"id": true,
		},
	}
	assert.Empty(t, j.ID())
}

func TestActivityObject_New(t *testing.T) {
	s := `{ "one": 1 }`
	o := NewMapObject([]byte(s)).(*MapObject)
	require.NotNil(t, o)
	n, ok := o.mapData["one"]
	assert.True(t, ok)
	assert.Equal(t, float64(1), n)
}

func TestActivityObject_New_Error(t *testing.T) {
	s := `one`
	o := NewMapObject([]byte(s))
	assert.Nil(t, o)
}
