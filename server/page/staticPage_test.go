package page

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStaticPage_ServeHTTP(t *testing.T) {
	page := internalStaticPage{
		source: StaticPage{
			ContentType: "text/plain",
			Template:    `{{ .HostName }}`,
		},
	}
	meta := MetaData{
		HostName: "test",
	}
	err := page.Init(meta)
	assert.NoError(t, err)
	assert.Equal(t, []byte("test"), page.rendered)

	r := httptest.NewRequest("GET", "/anything", nil)
	recorder := httptest.NewRecorder()

	page.ServeHTTP(recorder, r)
	response := recorder.Result()
	body, _ := io.ReadAll(response.Body)

	assert.Equal(t, []byte("test"), body)
	assert.Equal(t, page.source.ContentType, response.Header.Get("Content-Type"))
}

func TestStaticPage_Init_TemplateFails(t *testing.T) {
	page := internalStaticPage{
		source: StaticPage{
			Template: `}}{{`,
		},
	}
	err := page.Init(MetaData{})
	assert.Error(t, err)
}

func TestStaticPage_Init_ExecuteFails(t *testing.T) {
	page := internalStaticPage{
		source: StaticPage{
			Template: `{{ .Missing }}`,
		},
	}
	err := page.Init(MetaData{})
	assert.Error(t, err)
}
