package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tkrehbiel/activitylace/server/activity"
)

func TestGetKey_String(t *testing.T) {
	const testJSON = `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"summary": "Sally followed John",
		"type": "Follow",
		"actor": {
		 "type": "Person",
		 "name": "Sally",
		 "id": "sally"
		},
		"object": {
		 "type": "Person",
		 "name": "John",
		 "id": "john"
		}
	   }`
	var act activity.Activity
	require.NoError(t, json.Unmarshal([]byte(testJSON), &act))
	assert.Equal(t, "sally", getKey(act.Actor, "id"))
	assert.Equal(t, "john", getKey(act.Object, "id"))
}
