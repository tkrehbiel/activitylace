package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	b := []byte(`
	{
		"server": {
		  "host": "testhost",
		  "certificate": "testcert",
		  "privatekey": "testkey",
		  "port": 234
		},
		"users": [
		  {
			"name": "testuser"
		  }
		]
	  }`)
	cfg, err := ReadConfig(b)
	require.NoError(t, err)

	expected := Config{
		Server: serverConfig{
			HostName:    "testhost",
			Certificate: "testcert",
			PrivateKey:  "testkey",
			Port:        234,
		},
		Users: []userConfig{
			{
				Name: "testuser",
			},
		},
	}
	assert.Equal(t, expected, cfg)
}
