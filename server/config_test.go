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
		  "port": 234,
		  "accept_all": true,
		  "send_unsigned": true,
		  "receive_unsigned": true,
		  "max_followers": 100
		},
		"users": [
		  {
			"name": "testuser",
			"type": "testtype",
			"displayName": "testdisplayname",
			"outboxSource": "testurl",
			"pubKey": "testpub",
			"privKey": "testprivate"
		  }
		]
	  }`)
	cfg, err := ReadConfig(b)
	require.NoError(t, err)

	expected := Config{
		Server: serverConfig{
			HostName:        "testhost",
			Certificate:     "testcert",
			PrivateKey:      "testkey",
			Port:            234,
			AcceptAll:       true,
			SendUnsigned:    true,
			ReceiveUnsigned: true,
			MaxFollowers:    100,
		},
		Users: []userConfig{
			{
				Name:        "testuser",
				Type:        "testtype",
				DisplayName: "testdisplayname",
				SourceURL:   "testurl",
				PubKeyFile:  "testpub",
				PrivKeyFile: "testprivate",
			},
		},
	}
	assert.Equal(t, expected, cfg)
}
