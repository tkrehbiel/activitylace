package page

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebFinger_Template(t *testing.T) {
	meta := UserMetaData{
		MetaData: MetaData{
			HostName: "localhost",
		},
		UserName: "test",
	}
	page := NewStaticPage(WebFingerAccount)
	err := page.Init(meta)
	assert.NoError(t, err)
}

func TestWebFinger_UserPage(t *testing.T) {
	const (
		testScheme = "ftp"
		testHost   = "testhost"
		testUser   = "testuser"
	)

	u, err := url.Parse(fmt.Sprintf("%s://%s", testScheme, testHost))
	require.NoError(t, err)

	meta := MetaData{
		URL:      u.String(),
		Scheme:   u.Scheme,
		HostName: u.Hostname(),
	}
	var (
		testAccount     = fmt.Sprintf("acct:%s@%s", testUser, testHost)
		testUserID      = fmt.Sprintf("%s://%s/a/%s", testScheme, testHost, testUser)
		testUserProfile = fmt.Sprintf("%s://%s/profile/%s", testScheme, testHost, testUser)
	)

	var page MultiStaticPage
	page.Add(testUser, meta)

	r := httptest.NewRequest("GET", fmt.Sprintf("/anything?resource=%s", testAccount), nil)
	recorder := httptest.NewRecorder()

	page.ServeHTTP(recorder, r)
	response := recorder.Result()
	body, _ := io.ReadAll(response.Body)

	assert.NotNil(t, body)
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode)

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	links := data["links"].([]interface{})
	assert.Equal(t, testAccount, data["subject"])
	assert.Equal(t, testUserID, links[0].(map[string]interface{})["href"])
	assert.Equal(t, testUserProfile, links[1].(map[string]interface{})["href"])
}
