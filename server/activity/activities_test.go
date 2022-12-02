package activity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActivity(t *testing.T) {
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
}
