package page

import (
	"encoding/xml"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWellKnownHostMeta(t *testing.T) {
	u, err := url.Parse("http://test")
	require.NoError(t, err)
	meta := NewMetaData(u)

	page := NewStaticPage(WellKnownHostMeta).(*internalStaticPage)
	assert.NoError(t, page.Init(meta))
	assert.NotNil(t, page.rendered)

	// Just test that the XML is valid
	var data []interface{}
	require.NoError(t, xml.Unmarshal(page.rendered, &data))
}
