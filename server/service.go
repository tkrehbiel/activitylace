package server

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/karlseguin/ccache/v3"
	"github.com/tkrehbiel/activitylace/server/activity"
	"github.com/tkrehbiel/activitylace/server/page"
	"github.com/tkrehbiel/activitylace/server/storage"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityService struct {
	config     Config
	server     http.Server     // for serving http responses
	router     *mux.Router     // for request routing
	pipeline   *OutputPipeline // on which output messages are queued
	client     http.Client     // http client for outgoing requests
	meta       page.MetaData   // metadata for page templates
	users      []ActivityUser  // ActivityPub user accounts handled
	actorCache *ccache.Cache[activity.Actor]
}

type ActivityUser struct {
	name     string            // name of the account
	meta     page.UserMetaData // metadata for the account
	store    storage.Database  // associated data storage
	outbox   ActivityOutbox    // outbox
	inbox    ActivityInbox     // inbox
	privKey  crypto.PrivateKey // private key
	pubKeyID string            // public key ID
}

// addHandlers creates page routes for handling ActivityPub endpoints
func (s *ActivityService) addHandlers() {
	s.router.HandleFunc("/", homeHandler).Methods("GET")

	s.addPageHandler(page.NewStaticPage(page.WellKnownHostMeta), s.meta)
	s.addPageHandler(page.NewStaticPage(page.WellKnownNodeInfo), s.meta)
	s.addPageHandler(page.NewStaticPage(page.NodeInfo), s.meta)

	for _, u := range s.config.Users {
		page.WellKnownWebFinger.Add(u.Name, s.meta)
	}
	s.addPageHandler(&page.WellKnownWebFinger, s.meta)

	for i := range s.users {
		// TODO: umeta.LatestNotes = s.outbox[i].GetLatestNotes(10)
		user := &s.users[i]

		pg := page.ActorEndpoint // copy
		pg.Path = fmt.Sprintf("/%s/%s", page.SubPath, user.name)
		s.addPageHandler(page.NewStaticPage(pg), user.meta)

		// TODO: This should be a dynamic page since it should include latest activity
		pg = page.ProfilePage // copy
		pg.Path = fmt.Sprintf("/profile/%s", user.name)
		s.addPageHandler(page.NewStaticPage(pg), user.meta)

		outpath := fmt.Sprintf("/%s/%s/outbox", page.SubPath, user.name)
		route := s.router.HandleFunc(outpath, user.outbox.ServeHTTP).Methods("GET") // TODO: filter by Accept
		if !s.config.Server.AcceptAll {
			route.HeadersRegexp("Accept", "application/.*json")
		}

		inpath := fmt.Sprintf("/%s/%s/inbox", page.SubPath, user.name)
		route = s.router.HandleFunc(inpath, user.inbox.GetHTTP).Methods("GET") // TODO: filter by Accept
		if !s.config.Server.AcceptAll {
			route.HeadersRegexp("Accept", "application/.*json")
		}
		route = s.router.HandleFunc(inpath, user.inbox.PostHTTP).Methods("POST")
		if !s.config.Server.AcceptAll {
			route.HeadersRegexp("Content-Type", "application/.*json")
		}

	}

	// TODO: robots.txt
}

func (s *ActivityService) addPageHandler(pg page.StaticPageHandler, meta any) {
	pg.Init(meta)
	router := s.router.HandleFunc(pg.Path(), pg.ServeHTTP).Methods("GET")
	if !s.config.Server.AcceptAll && pg.Accept() != "" && pg.Accept() != "*/*" {
		router.HeadersRegexp("Accept", pg.Accept())
	}
}

// Start running the ActivityPub service and return immediately
func (s *ActivityService) Start(ctx context.Context) {
	go s.pipeline.Run(ctx)
	go func() {
		err := s.ListenAndServe(ctx)
		if err != nil && err != http.ErrServerClosed {
			telemetry.Error(err, "while listening")
		}
	}()
}

// Stop anything related to the service before exiting
func (s *ActivityService) Stop(ctx context.Context) {
	if err := s.server.Shutdown(ctx); err != nil {
		telemetry.Error(err, "while shutting down server")
	}
	for i := range s.users {
		s.users[i].store.Close()
	}
	telemetry.LogCounters()
}

// ListenAndServe; listen for http requests and serve responses
func (s *ActivityService) ListenAndServe(ctx context.Context) error {
	// Spawn RSS feed watcher goroutines
	if s.pipeline == nil {
		panic("ActivityService doesn't have a Pipeline")
	}
	for i := range s.users {
		go s.users[i].outbox.WatchRSS(ctx)
	}
	if s.config.Server.useTLS() {
		telemetry.Log("tls listener starting on port %d", s.config.Server.Port)
		return s.server.ListenAndServeTLS(s.config.Server.Certificate, s.config.Server.PrivateKey)
	} else {
		telemetry.Log("http listener starting on port %d", s.config.Server.Port)
		return s.server.ListenAndServe()
	}
}

// ActivityRequest creates an http request for sending ActivityPub communications
func (s *ActivityService) ActivityRequest(method string, url string, v any) (*http.Request, error) {
	var reader io.Reader
	if v != nil {
		body, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshaling json from object: %w", err)
		}
		reader = bytes.NewBuffer(body)
	}
	r, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("creating ActivityPub request: %w", err)
	}
	r.Header.Add("User-Agent", "Activitylace/0.1 (+https://github.com/tkrehbiel/activitylace)")
	r.Header.Add("Accept", activity.ContentType)
	r.Header.Add("Content-Type", activity.ContentType)
	r.Header.Add("Host", s.config.PublicHost())
	r.Header.Add("Date", time.Now().UTC().Format(http.TimeFormat))
	return r, nil
}

// GetActor finds the remote endpoint for the actor ID, which is assumed to be a URL.
// Blocks until we get a response or the context is cancelled or times out.
// TODO: Include a context param.
func (s *ActivityService) GetActor(id string) (*activity.Actor, error) {
	item := s.actorCache.Get(id)
	if item != nil && !item.Expired() {
		telemetry.Trace("found actor %s in cache", id)
		cached := item.Value()
		return &cached, nil
	}

	// TODO: maybe support webfingering an acct:x@y resource too
	// TODO: make this more asynchronous, and (optionally?) cache the results locally
	// TODO: retry periodically?

	r, err := s.ActivityRequest("GET", id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(r)
	if err != nil {
		return nil, err
	}
	var actor activity.Actor
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&actor); err != nil {
		telemetry.Error(err, "decoding json body")
		return nil, err
	}

	s.actorCache.Set(id, actor, 10*time.Minute)

	return &actor, nil
}

// GetActorPublicKey fetches the public key ID associated with the given actor URL.
// Blocks until a result is returned.
// TODO: Include a context param.
func (s *ActivityService) GetActorPublicKey(id string) crypto.PublicKey {
	// TODO: Cache this result!
	url, err := url.Parse(id)
	if err != nil {
		telemetry.Error(err, "parsing public key ID [%s]", id)
		return nil
	}
	url.Fragment = "" // remove the fragment

	actor, err := s.GetActor(url.String())
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
	pubKeyPem := actor.PublicKey.TransformedKey()
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

func decodePrivateKey(content []byte) (any, error) {
	p, _ := pem.Decode(content)
	if p == nil {
		return nil, fmt.Errorf("failed to decode pem")
	}

	switch p.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(p.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS8 private key file: %w", err)
		}
		return key, nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(p.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS1 private key file: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unknown private key type %s", p.Type)
	}
}

// NewService creates an http service to listen for ActivityPub requests
func NewService(cfg Config) *ActivityService {
	svc := ActivityService{
		client: http.Client{
			Timeout: time.Second * 15,
		},
		config:     cfg,
		router:     mux.NewRouter(),
		users:      make([]ActivityUser, 0),
		actorCache: ccache.New(ccache.Configure[activity.Actor]()),
	}

	svc.pipeline = NewPipeline()
	svc.pipeline.client = svc.client

	u, err := url.Parse(cfg.URL)
	if err != nil {
		telemetry.Error(err, "parsing url [%s]", cfg.URL)
		return &svc
	}

	svc.pipeline.host = u.Host

	// metadata available to page templates
	svc.meta = page.MetaData{
		URL:      cfg.URL,
		HostName: u.Hostname(),
	}

	// configure inboxes and outboxes
	for _, usercfg := range cfg.Users {
		serverUser := ActivityUser{
			name: usercfg.Name,
			meta: svc.meta.NewUserMetaData(usercfg.Name),
		}

		umeta := &serverUser.meta

		if usercfg.PrivKeyFile != "" {
			der, err := os.ReadFile(usercfg.PrivKeyFile)
			if err != nil {
				telemetry.Error(err, "reading private key file [%s]", usercfg.PrivKeyFile)
				continue
			}

			key, err := decodePrivateKey(der)
			if err != nil {
				telemetry.Error(err, "decoding private key [%s]", usercfg.PrivKeyFile)
				continue
			}
			serverUser.privKey = key
		}

		if usercfg.PubKeyFile != "" {
			b, err := os.ReadFile(usercfg.PubKeyFile)
			if err != nil {
				telemetry.Error(err, "reading public key file [%s]", usercfg.PubKeyFile)
				continue
			}
			umeta.UserPublicKey = string(b)
		}

		dbName := fmt.Sprintf("user_%s.db", usercfg.Name)
		store := storage.NewDatabase(dbName)
		serverUser.store = store

		umeta.UserDisplayName = usercfg.DisplayName
		umeta.UserType = "Person"
		if usercfg.Type != "" {
			umeta.UserType = usercfg.Type
		}

		serverUser.outbox = ActivityOutbox{
			service:        &svc,
			id:             path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/outbox", page.SubPath, usercfg.Name)),
			ownerID:        usercfg.Name,
			rssURL:         usercfg.SourceURL,
			notes:          store.(storage.Notes),
			followers:      store.(storage.Followers),
			pipeline:       svc.pipeline,
			privKey:        serverUser.privKey,
			pubKeyID:       umeta.UserPublicKeyID,
			acceptUnsigned: cfg.Server.ReceiveUnsigned,
			sendUnsigned:   cfg.Server.SendUnsigned,
		}

		serverUser.inbox = ActivityInbox{
			service:        &svc,
			id:             path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/inbox", page.SubPath, usercfg.Name)),
			ownerID:        serverUser.meta.UserID,
			followers:      store.(storage.Followers),
			pipeline:       svc.pipeline,
			privKey:        serverUser.privKey,
			pubKeyID:       umeta.UserPublicKeyID,
			acceptUnsigned: cfg.Server.ReceiveUnsigned,
			sendUnsigned:   cfg.Server.SendUnsigned,
		}

		if err := serverUser.store.Open(); err != nil {
			telemetry.Error(err, "opening sqlite database [%s]", dbName)
		} else {
			svc.users = append(svc.users, serverUser)
		}

		telemetry.Trace("user %s initialized", serverUser.name)
	}

	// configure web handlers
	svc.addHandlers()

	// Log all requests in the router without having to explicitly do so
	svc.router.Use(RequestLoggerMiddleware(svc.router))

	svc.server = http.Server{
		Handler:      svc.router,
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}

	telemetry.Trace("service initialized")
	return &svc
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "homeHandler")
	telemetry.Increment("home_requests", 1)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<html><title>activitylace</title>
<body>
<p>This is <a href="https://github.com/tkrehbiel/activitylace/">activitylace</a>,
an experimental ActivityPub server implementation to complement static blogs.
There's nothing to see here.</p>
</body>
</html>`)
}

func RequestLoggerMiddleware(r *mux.Router) mux.MiddlewareFunc {
	// This logs all requests that go through the router
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			telemetry.Request(req, "")
			next.ServeHTTP(w, req)
		})
	}
}
