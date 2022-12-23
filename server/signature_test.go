package server

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/karlseguin/ccache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/page"
)

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
	// Test that sign and verify works with a generated key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	content := []byte("test body content")

	digest := sha256.New()
	digest.Write(content)
	encoded64 := base64.StdEncoding.EncodeToString(digest.Sum(nil))
	expectedDigest := fmt.Sprintf("SHA-256=%s", encoded64)

	date := time.Date(2022, 12, 21, 6, 7, 8, 0, time.UTC)

	body := bytes.NewBuffer(content)
	pubKeyID := "abc"
	r := httptest.NewRequest("POST", "http://127.0.0.1/path", body)
	r.Header.Set("Date", date.Format(http.TimeFormat))
	r.Header.Set("Host", "testhost")
	r.Header.Set("Content-Type", "text/plain")
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))

	err = sign(privKey, pubKeyID, r)
	require.NoError(t, err)
	assert.NotEmpty(t, r.Header.Get("Digest"))
	assert.NotEmpty(t, r.Header.Get("Signature"))
	assert.Equal(t, expectedDigest, r.Header.Get("Digest"))
	// silly way of checking these
	assert.Contains(t, r.Header.Get("Signature"), "rsa-sha256")
	assert.Contains(t, r.Header.Get("Signature"), "(request-target)")
	assert.Contains(t, r.Header.Get("Signature"), "host")
	assert.Contains(t, r.Header.Get("Signature"), "date")
	assert.Contains(t, r.Header.Get("Signature"), "digest")
	//assert.Contains(t, r.Header.Get("Signature"), "content-length")

	fmt.Println(r.Header.Get("Host"))
	fmt.Println(r.Header.Get("Date"))
	fmt.Println(r.Header.Get("Content-Type"))
	fmt.Println(r.Header.Get("Content-Length"))
	fmt.Println(r.Header.Get("Digest"))
	fmt.Println(r.Header.Get("Signature"))

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
	// test that sign and verify works with an external key pem
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

	// Testing the whole round trip to the actor URL and fetching the public key

	svc := ActivityService{
		actorCache: ccache.New(ccache.Configure[activity.Actor]()),
	}

	actorPage := page.ActorEndpoint
	staticPage := page.NewStaticPage(actorPage)

	remoteActor := httptest.NewServer(staticPage)
	defer remoteActor.Close()

	pubKeyID := remoteActor.URL + "#main-key"

	umeta := page.UserMetaData{
		UserID:          remoteActor.URL,
		UserPublicKeyID: pubKeyID,
		UserPublicKey:   testPublicKey,
	}
	staticPage.Init(umeta)

	date := time.Date(2022, 12, 25, 6, 7, 8, 0, time.UTC)

	body := bytes.NewBuffer([]byte("test body content"))
	r := httptest.NewRequest("POST", "http://127.0.0.1/path", body)
	r.Header.Set("Date", date.Format(http.TimeFormat))
	r.Header.Set("Host", "testhost")
	r.Header.Set("Content-Type", "text/plain")
	r.Header.Set("Content-Length", fmt.Sprintf("%d", body.Len()))

	sign(privKey, pubKeyID, r)

	// no need to duplicate the validations in the other unit test here

	fmt.Println(r.Header.Get("Host"))
	fmt.Println(r.Header.Get("Date"))
	fmt.Println(r.Header.Get("Content-Type"))
	fmt.Println(r.Header.Get("Content-Length"))
	fmt.Println(r.Header.Get("Digest"))
	fmt.Println(r.Header.Get("Signature"))

	assert.NoError(t, verify(&svc, r))
}
