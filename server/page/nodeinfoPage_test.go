package page

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeInfo(t *testing.T) {
	u, err := url.Parse("http://test")
	require.NoError(t, err)
	meta := NewMetaData(u)

	page := NewStaticPage(WellKnownNodeInfo).(*internalStaticPage)
	assert.NoError(t, page.Init(meta))
	assert.NotNil(t, page.rendered)

	// Test that the JSON is valid
	var data map[string][]map[string]interface{} // map to an array of maps heh
	require.NoError(t, json.Unmarshal(page.rendered, &data))
	assert.Equal(t, "http://test/nodeinfo/2.1", data["links"][0]["href"])
}

func TestWellKnown(t *testing.T) {
	u, err := url.Parse("http://test")
	require.NoError(t, err)
	meta := NewMetaData(u)

	page := NewStaticPage(NodeInfo).(*internalStaticPage)
	assert.NoError(t, page.Init(meta))
	assert.NotNil(t, page.rendered)

	// Test that the JSON is valid
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(page.rendered, &data))
	assert.Equal(t, "2.1", data["version"])
}
