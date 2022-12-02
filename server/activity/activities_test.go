package activity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivity_Strings(t *testing.T) {
	const exampleFollow = `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"summary": "Sally followed John",
		"type": "Follow",
		"actor": "Sally",
		"object": "John"
	  }`
	var act Activity
	err := json.Unmarshal([]byte(exampleFollow), &act)
	require.NoError(t, err)
	// test that the properties unmarshal to strings
	_, ok1 := act.Actor.(string)
	assert.True(t, ok1)
	_, ok2 := act.Object.(string)
	assert.True(t, ok2)
}

func TestActivity_Maps(t *testing.T) {
	const exampleFollow = `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"summary": "Sally followed John",
		"type": "Follow",
		"actor": {
		  "type": "Person",
		  "name": "Sally"
		},
		"object": {
		  "type": "Person",
		  "name": "John"
		}
	  }`
	var act Activity
	err := json.Unmarshal([]byte(exampleFollow), &act)
	require.NoError(t, err)
	// test that the properties unmarshal to maps
	_, ok1 := act.Actor.(map[string]interface{})
	assert.True(t, ok1)
	_, ok2 := act.Object.(map[string]interface{})
	assert.True(t, ok2)
}
