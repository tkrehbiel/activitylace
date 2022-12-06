package server

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-fed/httpsig"
	"github.com/google/uuid"
	"github.com/karlseguin/ccache/v3"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/storage"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityInbox struct {
	id             string
	ownerID        string // id of the owner of the inbox
	followers      storage.Followers
	pipeline       *OutputPipeline
	privKey        crypto.PrivateKey
	pubKeyID       string
	actorCache     *ccache.Cache[activity.Actor]
	acceptUnsigned bool
}

// GetHTTP handles GET requests to the inbox, which we don't do
func (ai *ActivityInbox) GetHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "ActivityInbox.ServeHTTP [%s]", ai.id)
	telemetry.Increment("get_requests", 1)
	collection := activity.OrderedNoteCollection{
		Context: activity.Context,
		Type:    activity.OrderedCollectionType,
		ID:      ai.id,
	}
	jsonBytes, err := json.Marshal(&collection)
	if err != nil {
		telemetry.Error(err, "marshaling collection")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", activity.ContentType)
	w.Write(jsonBytes)
}

// PostHTTP handles POST requests to the inbox.
// This is where the bulk of handling communications from remote federated servers happens.
// e.g. Follow requests will come in through here.
func (ai *ActivityInbox) PostHTTP(w http.ResponseWriter, r *http.Request) {
	if ai.pipeline == nil {
		panic("ActivityInbox pipeline missing")
	}

	telemetry.Increment("post_requests", 1)

	if !ai.acceptUnsigned {
		if err := verify(ai, r); err != nil {
			telemetry.Error(err, "verifying signature")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	jsonBytes, err := io.ReadAll(io.LimitReader(r.Body, 4000))
	if err != nil {
		telemetry.Error(err, "reading body bytes")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var act activity.Activity
	if err := json.Unmarshal(jsonBytes, &act); err != nil {
		telemetry.Error(err, "unmarshaling activity [%s]", string(jsonBytes))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch act.Type {
	case "Follow":
		ai.Follow(w, act)
	case "Undo":
		if objectMap, ok := act.Object.(map[string]interface{}); ok {
			switch objectMap[activity.TypeProperty] {
			case activity.FollowType:
				// Unmarshal the object to its own struct
				var unfollow struct {
					Object activity.Activity `json:"object"`
				}
				err := json.Unmarshal(jsonBytes, &unfollow)
				if err != nil {
					telemetry.Error(err, "unmarshalling Undo activity's Object [%s]", string(jsonBytes))
				} else {
					ai.Unfollow(w, act, unfollow.Object)
				}
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}
	default:
		// unrecognized Activity Type
		telemetry.Trace("unrecognized activity type [%s] %s", act.Type, string(jsonBytes))
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (ai *ActivityInbox) Follow(w http.ResponseWriter, act activity.Activity) {
	// Yeesh this is more complex than I thought it would be

	telemetry.Increment("follow_requests", 1)

	// The actor is the id of the person who wants to follow
	actorID := parseID(act.Actor)

	// The object is the user that is to be followed, which should be the owner of the Inbox.
	objectID := parseID(act.Object)

	var message = fmt.Sprintf("POST follow [%s] by [%s] at inbox [%s]", objectID, actorID, ai.id)
	defer func() {
		telemetry.Log(message)
	}()

	if objectID != ai.ownerID {
		// Trying to follow someone other than the owner of this inbox, doesn't make sense.
		// #ActivityPub There is no information about what to do in this situation in the spec.
		message += " - rejected, wrong inbox"
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if act.ID == "" {
		// No Follow ID provided. #ActivityPub doesn't specify if this is required,
		// but we reject these, because the ID is crucial for sending Accept response,
		// so the remote server knows what we're accepting.
		act.ID = strings.Join([]string{objectID, actorID}, "-")
		message += " - rejected, no follow id"
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	existing, err := ai.followers.FindFollow(actorID)
	if err != nil {
		message += " - rejected, database read error"
		telemetry.Error(err, "database error")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if existing != nil {
		// Already following, no need to do anything.
		// #ActivityPub says nothing about what to do in this situation. Just winging it.
		// TODO: Probably should go ahead and send an Accept anyway, in case the activity didn't finish.
		message += " - already following"
		// We don't really need to do anything, but the sender is expecting a response anyway.
		// Falls through and attempts to send an accept anyway (though it may have already been sent)
	}

	// Save the new follower. We mark it as "pending" until we successfully
	// send an Accept request back to the remote server.
	follow := storage.Follow{
		ID:            actorID,
		RequestID:     act.ID,
		RequestStatus: "pending",
	}
	if err := ai.followers.SaveFollow(follow); err != nil {
		message += " - database write error"
		telemetry.Error(err, "database error")
	}

	// Queue an Accept response.
	// We have to do that outside this function or else race conditions.
	telemetry.Trace("queuing an accept response")
	ai.pipeline.Queue(&FollowResponse{
		inbox:        ai,
		followers:    ai.followers,
		follow:       follow,
		followID:     act.ID,
		remoteID:     actorID,
		localID:      ai.ownerID,
		responseType: activity.AcceptType,
	})

	message += " - success"
	w.WriteHeader(http.StatusOK)
}

type FollowResponse struct {
	inbox        *ActivityInbox
	followers    storage.Followers
	follow       storage.Follow
	followID     string
	localID      string
	remoteID     string
	responseType string
}

func (f *FollowResponse) String() string {
	return fmt.Sprintf("Follow %s to %s", f.responseType, f.remoteID)
}

func (f *FollowResponse) Prepare(pipeline *OutputPipeline) (*http.Request, error) {
	// Lookup the follower's inbox
	telemetry.Increment("actor_fetches", 1)
	remote, err := f.inbox.GetActor(f.remoteID)
	if err != nil {
		return nil, fmt.Errorf("looking up remote actor: %w", err)
	}

	// ActivityPub requires us to send an Accept response to the followee's inbox
	// https://www.w3.org/TR/activitypub/#follow-activity-inbox
	telemetry.Trace("sending accept request")

	acceptID := uuid.NewString()

	acceptObject := struct {
		Context string            `json:"@context"`
		Type    string            `json:"type"`
		ID      string            `json:"id"`
		Actor   string            `json:"actor"`
		Object  activity.Activity `json:"object"`
		To      []string          `json:"to"`
		CC      []string          `json:"cc"`
	}{
		Context: activity.Context,
		Type:    f.responseType,
		ID:      acceptID, // Pleroma requires an id
		Actor:   f.localID,
		To:      []string{f.remoteID}, // Pleroma seems to require a to array
		CC:      make([]string, 0),    // Pleroma seems to require a cc array
		Object: activity.Activity{
			// Return the information that was sent to us
			Type:   activity.FollowType,
			ID:     f.followID,
			Actor:  f.remoteID,
			Object: f.localID,
		},
	}

	r, err := pipeline.ActivityRequest(http.MethodPost, remote.Inbox, &acceptObject)
	if err != nil {
		return nil, fmt.Errorf("creating accept request: %w", err)
	}

	if f.inbox.privKey != nil {
		sign(f.inbox.privKey, f.inbox.pubKeyID, r)
	}

	return r, nil
}

func (f *FollowResponse) Receive(resp *http.Response) {
	telemetry.Trace("received response from accept %d", resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		telemetry.Increment("accept_responses", 1)
		// mark transaction was completed successfully
		f.follow.RequestStatus = "accepted"
		if err := f.followers.SaveFollow(f.follow); err != nil {
			// Bad time for a database error. This will leave the follow request
			// marked as "pending" in our local database, but the remote server
			// will believe the transaction was successful and they are following.
			telemetry.Error(err, "database error")
		}
	}
}

func (ai *ActivityInbox) Unfollow(w http.ResponseWriter, undo activity.Activity, follow activity.Activity) {
	if ai.pipeline == nil {
		panic("ActivityInbox pipeline missing")
	}

	telemetry.Increment("undo_requests", 1)

	// The actor is the id of the person who wants to undo
	actorID := parseID(follow.Actor)

	// The object is the user that is to be followed. It should match the owner of the Inbox.
	objectID := parseID(follow.Object)

	var message = fmt.Sprintf("POST unfollow [%s] by [%s] at inbox [%s]", objectID, actorID, ai.id)
	defer func() {
		telemetry.Log(message)
	}()

	if undo.ID == "" {
		// We really should have an ID, but #ActivityPub doesn't tell us what to do if we don't.
		message += " - rejected, no undo ID"
		w.WriteHeader(http.StatusBadRequest)
	}

	if objectID != ai.ownerID {
		// Trying to follow someone other than the owner of this inbox, not allowed.
		// #ActivityPub There is no information about what to do in this situation in the spec.
		message += " - rejected, wrong inbox"
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if err := ai.followers.DeleteFollow(actorID); err != nil {
		message += " - database delete error"
		telemetry.Error(err, "database error")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Queue an Accept response.
	// We have to do that outside this function or else race conditions.
	telemetry.Trace("queuing an accept response")
	ai.pipeline.Queue(&UnfollowResponse{
		inbox:        ai,
		undoID:       undo.ID,
		remoteID:     actorID,
		localID:      ai.ownerID,
		responseType: activity.AcceptType,
	})

	message += " - success"
	w.WriteHeader(http.StatusOK)
}

type UnfollowResponse struct {
	inbox        *ActivityInbox
	undoID       string
	localID      string
	remoteID     string
	responseType string
}

func (f *UnfollowResponse) String() string {
	return fmt.Sprintf("Unfollow %s to %s", f.responseType, f.remoteID)
}

func (f *UnfollowResponse) Prepare(pipeline *OutputPipeline) (*http.Request, error) {
	// Lookup the follower's inbox
	telemetry.Increment("actor_fetches", 1)
	remote, err := f.inbox.GetActor(f.remoteID)
	if err != nil {
		return nil, fmt.Errorf("looking up remote actor: %w", err)
	}

	acceptID := uuid.NewString()

	// ActivityPub requires us to send an Accept response to the followee's inbox
	// https://www.w3.org/TR/activitypub/#follow-activity-inbox
	telemetry.Trace("sending accept request")

	type undoFollow struct {
		Type   string            `json:"type"`
		ID     string            `json:"id"`
		Actor  string            `json:"actor,omitempty"`
		Object activity.Activity `json:"object,omitempty"`
		To     []string          `json:"to"`
		CC     []string          `json:"cc"`
	}
	acceptObject := struct {
		Context string     `json:"@context"`
		Type    string     `json:"type"`
		ID      string     `json:"id"` // Pleroma requires an id
		Actor   string     `json:"actor,omitempty"`
		Object  undoFollow `json:"object,omitempty"`
		To      []string   `json:"to"`
		CC      []string   `json:"cc"`
	}{
		Context: activity.Context,
		Type:    activity.AcceptType,
		ID:      acceptID,
		Actor:   f.localID,
		To:      []string{f.remoteID}, // Pleroma seems to require a to array
		CC:      make([]string, 0),    // Pleroma seems to require a cc array
		Object: undoFollow{
			// Recreate the basics of the information that was sent to us.
			Type:  activity.UndoType,
			ID:    f.undoID,
			Actor: f.localID,
			Object: activity.Activity{
				// Doesn't try to send the Follow ID back, why bother?
				Type:   activity.FollowType,
				Actor:  f.remoteID,
				Object: f.localID,
			},
		},
	}

	r, err := pipeline.ActivityRequest(http.MethodPost, remote.Inbox, &acceptObject)
	if err != nil {
		return nil, fmt.Errorf("creating accept request: %w", err)
	}

	if f.inbox.privKey != nil {
		sign(f.inbox.privKey, f.inbox.pubKeyID, r)
	}

	return r, nil
}

func (f *UnfollowResponse) Receive(resp *http.Response) {
	telemetry.Trace("received response from accept %d", resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		// Nothing really to do
		telemetry.Increment("accept_responses", 1)
	}
}

func parseID(v interface{}) (val string) {
	// Rather annoyingly, JSON-LD parameters could be a simple string or they can be expansive maps,
	// so we should be prepared to handle either situation. A valid JSON-LD object could be very complex,
	// which, if I can get on my soapbox for a moment, is a deficiency of the [ActivityPub] spec.
	// Nobody likes to deal with data types that are actually variants in the data interchange game.
	// This simple implementation is to try to avoid having to include a full-blown LSON-LD implementation just for corner cases.
	switch t := v.(type) {
	case string:
		// The data is a string, so we can just return it.
		// e.g. { "actor": "https://id" }
		val = t
	case map[string]interface{}:
		// The data is a map, so we need to retrieve one of the keys.
		// e.g. { "actor": { "name": "Alice", "id": "https://id" } }
		switch s := t["id"].(type) {
		case string:
			val = s
		case fmt.Stringer:
			val = s.String()
		}
	}
	return val
}

func readerUnmarshal(r io.Reader, v any) {
	decoder := json.NewDecoder(r)
	decoder.Decode(&v)
}

func jsonBytes(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func jsonReader(v any) io.Reader {
	return bytes.NewBuffer(jsonBytes(v))
}

// sign an http request with a public and private key
func sign(privateKey crypto.PrivateKey, pubKeyId string, r *http.Request) error {
	prefs := []httpsig.Algorithm{httpsig.RSA_SHA256}
	digestAlgorithm := httpsig.DigestSha256
	headersToSign := []string{httpsig.RequestTarget, "digest", "date", "host"}
	signer, _, err := httpsig.NewSigner(prefs, digestAlgorithm, headersToSign, httpsig.Signature, 0)
	if err != nil {
		return err
	}
	// To sign the digest, we need to give the signer a copy of the body
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // replace body since we read it
	// If r were a http.ResponseWriter, call SignResponse instead.
	return signer.SignRequest(privateKey, pubKeyId, r, body)
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

// LookupActor finds the remote endpoint for the actor ID, which is assumed to be a URL
// Blocks until we get a response or the context is cancelled or times out
func (ai *ActivityInbox) GetActor(id string) (*activity.Actor, error) {
	item := ai.actorCache.Get(id)
	if item != nil && !item.Expired() {
		telemetry.Trace("found actor %s in cache", id)
		cached := item.Value()
		return &cached, nil
	}

	// TODO: maybe support webfingering an acct:x@y resource too
	// TODO: make this more asynchronous, and (optionally?) cache the results locally
	// TODO: retry periodically?

	r, err := ai.pipeline.ActivityRequest("GET", id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := ai.pipeline.client.Do(r)
	if err != nil {
		return nil, err
	}
	var actor activity.Actor
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&actor); err != nil {
		telemetry.Error(err, "decoding json body")
		return nil, err
	}

	ai.actorCache.Set(id, actor, 10*time.Minute)

	return &actor, nil
}

func (ai *ActivityInbox) GetActorPublicKey(id string) crypto.PublicKey {
	// TODO: Cache this result!
	url, err := url.Parse(id)
	if err != nil {
		telemetry.Error(err, "parsing public key ID [%s]", id)
		return nil
	}
	url.Fragment = "" // remove the fragment

	actor, err := ai.GetActor(url.String())
	if err != nil {
		telemetry.Error(err, "fetching remote actor")
		return nil
	}

	if actor.ID != url.String() {
		telemetry.Error(err, "remote actor ID [%s] doesn't match [%s]", actor.ID, url.String())
		return nil
	}
	if actor.PublicKey.ID != id {
		telemetry.Error(err, "remote public key ID [%s] doesn't match [%s]", actor.PublicKey.ID, id)
		return nil
	}
	pubKeyPem := actor.PublicKey.Key
	der, _ := pem.Decode([]byte(pubKeyPem))
	if der == nil {
		telemetry.Error(nil, "can't decode pem [%s]", pubKeyPem)
		return nil
	}
	pubKey, err := x509.ParsePKIXPublicKey(der.Bytes)
	if err != nil {
		telemetry.Error(err, "parsing public key [%s]", pubKeyPem)
		return nil
	}
	return pubKey
}
