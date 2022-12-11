package server

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-fed/httpsig"
)

// At first I tried to use github.com/go-fed/httpsig but I had trouble communicating with Mastodon.

func computeDigest(body []byte) string {
	hash := sha256.New()
	hash.Write(body)
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func computeSigningString(signedHeaders []string, r *http.Request) string {
	signingStrings := make([]string, 0)
	for _, hdr := range signedHeaders {
		var s string
		switch hdr {
		case "(request-target)":
			s = fmt.Sprintf("(request-target): %s %s", strings.ToLower(r.Method), r.URL.Path)
		default:
			s = fmt.Sprintf("%s: %s", hdr, r.Header.Get(hdr))
		}
		signingStrings = append(signingStrings, s)
	}
	return strings.Join(signingStrings, "\n")
}

// sign an http request with a public and private key
func sign(privateKey crypto.PrivateKey, pubKeyId string, r *http.Request) error {
	// I'm genuinely unsure if go-fed/httpsig signature generation works right,
	// so I'm generating this signature manually.

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("cannot sign with this private key")
	}

	// Read and replace the request body so we can create a digest
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Generate digest of request body to include in the signature
	digest := computeDigest(body)
	r.Header.Add("Digest", fmt.Sprintf("SHA-256=%s", digest))

	// Generate the signing string from headers
	signedHeaders := []string{"(request-target)", "host", "date", "digest", "content-type"}
	signingString := computeSigningString(signedHeaders, r)

	// I imagine these aren't useful unless the receiver checks them
	created := time.Now().UTC()
	r.Header.Add("Created", created.Format(http.TimeFormat))
	expires := created.Add(time.Hour)
	r.Header.Add("Expires", expires.Format(http.TimeFormat))

	// Create the signature
	sigHash := sha256.New()
	sigHash.Write([]byte(signingString))
	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, sigHash.Sum(nil))
	if err != nil {
		return err
	}
	signature64 := base64.StdEncoding.EncodeToString(signature)
	r.Header.Add("Signature", fmt.Sprintf(`keyId="%s",algorithm="rsa-sha256",created=%d,expires=%d,headers="%s",signature="%s"`,
		pubKeyId, created.Unix(), expires.Unix(), strings.Join(signedHeaders, " "), signature64))
	return nil
}

// verify a signed http request, returns an err if the validation fails or nil on success
func verify(cert publicKeyLoader, r *http.Request) error {
	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return err
	}
	pubKeyId := verifier.KeyId()
	pubKey := cert.GetActorPublicKey(pubKeyId)
	if pubKey == nil {
		return fmt.Errorf("no public key to verify request signature")
	}
	algo := httpsig.RSA_SHA256
	// The verifier will verify the Digest in addition to the HTTP signature
	return verifier.Verify(pubKey, algo)
}

type publicKeyLoader interface {
	GetActorPublicKey(id string) crypto.PublicKey
}
