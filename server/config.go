package server

import (
	"encoding/json"
)

type serverConfig struct {
	HostName    string `json:"host"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"privatekey"`
	Port        int    `json:"port"`
	AcceptAll   bool   `json:"accept_all"` // for debugging
}

func (s serverConfig) useTLS() bool {
	return s.Certificate != "" && s.PrivateKey != ""
}

type userConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"displayName"`
	SourceURL   string `json:"outboxSource"`
	PubKeyFile  string `json:"pubKey,omitempty"`
	PrivKeyFile string `json:"privKey,omitempty"`
}

type Config struct {
	URL    string       `json:"url"` // public-facing URL
	Server serverConfig `json:"server"`
	Users  []userConfig `json:"users"`
}

func ReadConfig(b []byte) (config Config, err error) {
	if uErr := json.Unmarshal(b, &config); uErr != nil {
		return config, uErr
	}
	return config, nil
}
