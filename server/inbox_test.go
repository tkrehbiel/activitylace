package server

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/karlseguin/ccache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/storage"
)

func TestGetKey_String(t *testing.T) {
	const testJSON = `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"summary": "Sally followed John",
		"type": "Follow",
		"actor": {
		 "type": "Person",
		 "name": "Sally",
		 "id": "sally"
		},
		"object": {
		 "type": "Person",
		 "name": "John",
		 "id": "john"
		}
	   }`
	var act activity.Activity
	require.NoError(t, json.Unmarshal([]byte(testJSON), &act))
	assert.Equal(t, "sally", parseID(act.Actor))
	assert.Equal(t, "john", parseID(act.Object))
}

type mockDatabase struct {
	mock.Mock
}

func (m *mockDatabase) FindFollow(id string) (*storage.Follow, error) {
	args := m.Called(id)
	if f, ok := args.Get(0).(*storage.Follow); ok {
		return f, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockDatabase) DeleteFollow(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockDatabase) SaveFollow(f storage.Follow) error {
	args := m.Called(f)
	return args.Error(0)
}

// A complex integration test of the happy path for Follow and Accept logic
func TestInbox_Follow(t *testing.T) {
	pipeline := NewPipeline()
	go pipeline.Run(context.Background())
	defer pipeline.Stop()

	const id = "followed_id"
	const followID = "follow_request_id"
	var remoteID string

	inbox := ActivityInbox{
		id:         "test",
		ownerID:    id,
		pipeline:   pipeline,
		actorCache: ccache.New(ccache.Configure[activity.Actor]()),
	}

	// Simulate the remote inbox
	remoteInbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept"), "json")
		w.WriteHeader(http.StatusOK)
		var act activity.Activity
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&act)
		assert.NoError(t, err)
		assert.Equal(t, followID, parseID(act.Object)) // id of follow request
		assert.Equal(t, id, parseID(act.Actor))        // if of recipient of follow request
	}))
	defer remoteInbox.Close()

	// Simulate the remote actor URL
	remoteActor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept"), "json")
		actor := activity.Actor{
			Context: activity.Context,
			Type:    activity.PersonType,
			ID:      remoteID,
			Inbox:   remoteInbox.URL,
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes(&actor))
	}))
	defer remoteActor.Close()

	remoteID = remoteActor.URL

	database := &mockDatabase{}
	database.On("FindFollow", remoteID).Return(nil, nil).Once()
	database.On("SaveFollow", storage.Follow{
		ID:            remoteID,
		RequestID:     followID,
		RequestStatus: "pending",
	}).Return(nil).Once()
	database.On("SaveFollow", storage.Follow{
		ID:            remoteID,
		RequestID:     followID,
		RequestStatus: "accepted",
	}).Return(nil).Once()
	inbox.followers = database

	recorder := httptest.NewRecorder()
	follow := activity.Activity{
		Context: activity.Context,
		Type:    activity.FollowType,
		ID:      followID,
		Actor:   remoteID,
		Object:  id,
	}

	// wrap in a timeout to avoid potential deadlock
	timeout := time.After(3 * time.Second)
	done := make(chan bool)
	go func() {
		inbox.Follow(recorder, follow)
		done <- true
	}()
	select {
	case <-timeout:
		t.Fatal("Test didn't finish")
	case <-done:
		break
	}

	pipeline.Flush() // wait for queued requests to finish

	database.AssertExpectations(t)
}

// A complex integration test of the happy path for Undo Follow logic
func TestInbox_UnFollow(t *testing.T) {
	pipeline := NewPipeline()
	go pipeline.Run(context.Background())
	defer pipeline.Stop()

	const id = "followed_id"
	const undoID = "undo_request_id"
	var remoteID string

	inbox := ActivityInbox{
		id:         "test",
		ownerID:    id,
		pipeline:   pipeline,
		actorCache: ccache.New(ccache.Configure[activity.Actor]()),
	}

	// Simulate the remote inbox
	remoteInbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept"), "json")
		w.WriteHeader(http.StatusOK)
		var act activity.Activity
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&act)
		assert.NoError(t, err)
		assert.Equal(t, undoID, parseID(act.Object)) // id of undo request
	}))
	defer remoteInbox.Close()

	// Simulate the remote actor URL
	remoteActor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept"), "json")
		actor := activity.Actor{
			Context: activity.Context,
			Type:    activity.PersonType,
			ID:      remoteID,
			Inbox:   remoteInbox.URL,
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonBytes(&actor))
	}))
	defer remoteActor.Close()

	remoteID = remoteActor.URL

	database := &mockDatabase{}
	database.On("DeleteFollow", remoteID).Return(nil, nil).Once()
	inbox.followers = database

	recorder := httptest.NewRecorder()
	follow := activity.Activity{
		Type:   activity.FollowType,
		Actor:  remoteID,
		Object: id,
	}
	undo := activity.Activity{
		Context: activity.Context,
		Type:    activity.UndoType,
		ID:      undoID,
		Actor:   remoteID,
		Object:  follow,
	}

	// wrap in a timeout to avoid potential deadlock
	timeout := time.After(3 * time.Second)
	done := make(chan bool)
	go func() {
		inbox.Unfollow(recorder, undo, follow)
		done <- true
	}()
	select {
	case <-timeout:
		t.Fatal("Test didn't finish")
	case <-done:
		break
	}

	pipeline.Flush() // wait for queued requests to finish

	database.AssertExpectations(t)
}

type mockLoader struct {
	mock.Mock
}

func (m *mockLoader) GetActorPublicKey(id string) crypto.PublicKey {
	args := m.Called(id)
	if k, ok := args.Get(0).(crypto.PublicKey); ok {
		return k
	}
	return nil
}

func TestSignAndVerify_Self(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	body := bytes.NewBuffer([]byte("test body content"))
	pubKeyID := "abc"
	r := httptest.NewRequest("POST", "http://127.0.0.1", body)
	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	r.Header.Set("Host", "testhost")

	sign(privKey, pubKeyID, r)
	assert.NotEmpty(t, r.Header.Get("Digest"))
	assert.NotEmpty(t, r.Header.Get("Signature"))

	loader := &mockLoader{}
	loader.On("GetActorPublicKey", pubKeyID).Return(&privKey.PublicKey)

	assert.NoError(t, verify(loader, r))
	loader.AssertExpectations(t)
}

// don't worry, these were created just for these tests :)
const (
	testPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDVPboEbfAQW+F7
6eF7TKihgmYNycOwwLkIQx11uZQpfw++adfmryGAVTl8ve2DXDdCHVTgcfwNvREh
A8Fy+RLlQYezZpcBJqgtbHFISSCCWsHzpOPK2AYiKUy6QIKPAp9l3MvSdv8qrhcS
1G3fc5oksjTqiQfBee/xwXWOJOeuWaq++hl9Bv5inXILB6YqJMASvhNaeWYHlH2F
t/5n1XAvdTDZ4J6z+Ink8YMTJuh7O5JO0xeLjdDSlwvKxYb4in7FY7MV8/OJxYXf
brhCwF7nwAfcXQ0ku5JGPnHbTFC0R2mDNt7ppDuaGQiul3qgJXNCfCnsLBgoem7x
LPzrkz/zAgMBAAECggEAami4jCBONOOctAiER9J7rcjT41qFKA0r6FcRet8l88Uf
lp3bqpZHCfK9UqW2QaBBROE9KxlGNZbc1tQ4cwaeqr9WF6yAewcO0kf0iaVQLyxZ
75qfP2goO2DPlHu/ity8rQiOv1I5R9OC2RcfUuuthlVVOZoywBX4qfOnlUyOPj4/
ac1ZYEJeW7wlKt6DwK4WNUZX3E9ry3etVEr3OwGO/QMX79sZa5tCdsYsxkjRFe/5
sU7UAijRysBMutgQKZYdTFoj2bjtyEZM60c6kbbQVyuQgFnm4X5aep1jr4uNGb+2
Ir96HLT58CDlzQY2zGEkXRMJS97kXgWBWXcfsXmM8QKBgQD/lo6tI6Hr+tvunYEX
O/1XxAh+eLErelPxMRirTy2viByWeJSuvNz1kKn5b4YAe2HzObljb7TFkUrLoFyM
pRzv3KW5WemxUMGfdeJtVo+j7axhO/W2R//3JRvUz6FVHRn+ixmvfsEc0Efw5b1L
s4yVWmLQ4Un9qfDaqCw7e3+EwwKBgQDVlbL3CxEc3NF+rRo7GYWU/IANo1MxXdi3
amS0gsypQ2sVy5Zs8qz8da8XuU6lzvzglorjgsI+3Smbv0VnYxXk9yxe9VKQpiC3
i8Y+bgJhgwbZ1WA50E3f49j+WNRkKkXqi3s4FOKbPVGfsX5iBShcupKbZ4Jmyyhx
mJToxrPlEQKBgQD/Ta0HVeiQh+zY1Yv1YX8XBEJX0sdm3rKq4pf5xwWjqRqlU51x
TkaJJRAkkToRkS2uf6KnqRWxpAhKjszj0KqvDoCcPSwqarh+SIr9HNIutWLTXcl7
Y0BT50V9tkk5c/BbSydFHiBYX9T81P/ZdmifZ8H9VI1MTUzBnetRH3OpcQKBgQCY
I8FOfmCbOaQ04uNLc9uWi+I/VLbe9GV6CVxgxMc6Tt7JsLKfOqIEV2P4tzQRoga5
iCK4+xyYoPuRiMbMZWVkKrk9juxYQy4M8JCvSbeCdE39/yNDK2E9eVTJoMbx7rbM
4rxL73yXbi9lXI6VDe15WCE0d6AIzvApMrHnuhrMsQKBgCjOscC/GIKcch+EcN5b
nk7+r/l+B6mLtUn1/NQ3B8sqiunfutyBaEHmzxrWEzs5gd+3KtBLnRpqzG+gt5Ab
U0P4yxNndu4VZyLyPwtjU4vEuWKX3KaypZD+/LLirPVK1jIIALEsOGjP3scNoYs1
Pof7wbtMN0+Q75mqo8d3Gpyy
-----END PRIVATE KEY-----
`
	testPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1T26BG3wEFvhe+nhe0yo
oYJmDcnDsMC5CEMddbmUKX8PvmnX5q8hgFU5fL3tg1w3Qh1U4HH8Db0RIQPBcvkS
5UGHs2aXASaoLWxxSEkgglrB86TjytgGIilMukCCjwKfZdzL0nb/Kq4XEtRt33Oa
JLI06okHwXnv8cF1jiTnrlmqvvoZfQb+Yp1yCwemKiTAEr4TWnlmB5R9hbf+Z9Vw
L3Uw2eCes/iJ5PGDEyboezuSTtMXi43Q0pcLysWG+Ip+xWOzFfPzicWF3264QsBe
58AH3F0NJLuSRj5x20xQtEdpgzbe6aQ7mhkIrpd6oCVzQnwp7CwYKHpu8Sz865M/
8wIDAQAB
-----END PUBLIC KEY-----
`
)

func TestSignAndVerify_External(t *testing.T) {
	privBlock, _ := pem.Decode([]byte(testPrivateKey))
	require.NotNil(t, privBlock)
	pubBlock, _ := pem.Decode([]byte(testPublicKey))
	require.NotNil(t, pubBlock)

	privKey, err := x509.ParsePKCS8PrivateKey([]byte(privBlock.Bytes))
	require.NoError(t, err)
	require.NotNil(t, privKey)
	pubKey, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	require.NoError(t, err)
	require.NotNil(t, pubKey)

	body := bytes.NewBuffer([]byte("test body content"))
	pubKeyID := "abc"
	r := httptest.NewRequest("POST", "http://127.0.0.1", body)
	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	r.Header.Set("Host", "testhost")

	sign(privKey, pubKeyID, r)
	assert.NotEmpty(t, r.Header.Get("Digest"))
	assert.NotEmpty(t, r.Header.Get("Signature"))

	loader := &mockLoader{}
	loader.On("GetActorPublicKey", pubKeyID).Return(pubKey)

	assert.NoError(t, verify(loader, r))
}
