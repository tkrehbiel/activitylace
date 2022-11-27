package page

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActorPage_JSON(t *testing.T) {
	const testName = "testUserName"

	u, err := url.Parse("http://test")
	require.NoError(t, err)
	meta := NewMetaData(u)
	umeta := meta.NewUserMetaData(testName)

	// Test that the template parses
	page := NewStaticPage(ActorEndpoint)
	assert.NoError(t, page.Init(umeta))

	internalPage := page.(*internalStaticPage)
	assert.NotNil(t, internalPage.rendered)

	// Test that the JSON is valid
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(internalPage.rendered, &data))
	assert.Equal(t, testName, data["preferredUserName"])
}
