package server

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
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
	Config   Config
	Server   http.Server
	Pipeline *OutputPipeline
	router   *mux.Router
	meta     page.MetaData
	users    []ActivityUser
}

type ActivityUser struct {
	name     string
	meta     page.UserMetaData
	store    storage.Database
	outbox   ActivityOutbox
	inbox    ActivityInbox
	privKey  crypto.PrivateKey
	pubKeyID string
}

func (s *ActivityService) addHandlers() {
	s.router.HandleFunc("/", homeHandler).Methods("GET")

	s.addPageHandler(page.NewStaticPage(page.WellKnownHostMeta), s.meta)
	s.addPageHandler(page.NewStaticPage(page.WellKnownNodeInfo), s.meta)
	s.addPageHandler(page.NewStaticPage(page.NodeInfo), s.meta)

	for _, u := range s.Config.Users {
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
		route := s.router.HandleFunc(outpath, RequestLogger{Handler: user.outbox.ServeHTTP}.ServeHTTP).Methods("GET") // TODO: filter by Accept
		if !s.Config.Server.AcceptAll {
			route.HeadersRegexp("Accept", "application/.*json")
		}

		inpath := fmt.Sprintf("/%s/%s/inbox", page.SubPath, user.name)
		route = s.router.HandleFunc(inpath, RequestLogger{Handler: user.inbox.GetHTTP}.ServeHTTP).Methods("GET") // TODO: filter by Accept
		if !s.Config.Server.AcceptAll {
			route.HeadersRegexp("Accept", "application/.*json")
		}
		route = s.router.HandleFunc(inpath, RequestLogger{Handler: user.inbox.PostHTTP}.ServeHTTP).Methods("POST")
		if !s.Config.Server.AcceptAll {
			route.HeadersRegexp("Accept", "application/.*json")
		}

	}

	// TODO: robots.txt
}

func (s *ActivityService) addPageHandler(pg page.StaticPageHandler, meta any) {
	pg.Init(meta)
	router := s.router.HandleFunc(pg.Path(), RequestLogger{Handler: pg.ServeHTTP}.ServeHTTP).Methods("GET")
	if !s.Config.Server.AcceptAll && pg.Accept() != "" && pg.Accept() != "*/*" {
		router.Headers("Accept", pg.Accept())
	}
}

// Close anything related to the service before exiting
func (s *ActivityService) Close() {
	for i := range s.users {
		s.users[i].store.Close()
	}
	telemetry.LogCounters()
}

func (s *ActivityService) ListenAndServe(ctx context.Context) error {
	// Spawn RSS feed watcher goroutines
	if s.Pipeline == nil {
		panic("ActivityService doesn't have a Pipeline")
	}
	for i := range s.users {
		go s.users[i].outbox.WatchRSS(ctx)
	}
	if s.Config.Server.useTLS() {
		telemetry.Log("tls listener starting on port %d", s.Config.Server.Port)
		return s.Server.ListenAndServeTLS(s.Config.Server.Certificate, s.Config.Server.PrivateKey)
	} else {
		telemetry.Log("http listener starting on port %d", s.Config.Server.Port)
		return s.Server.ListenAndServe()
	}
}

// NewService creates an http service to listen for ActivityPub requests
func NewService(cfg Config) ActivityService {
	svc := ActivityService{
		Pipeline: NewPipeline(),
		Config:   cfg,
		router:   mux.NewRouter(),
		users:    make([]ActivityUser, 0),
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		telemetry.Error(err, "parsing url [%s]", cfg.URL)
		return svc
	}

	svc.Pipeline.host = u.Host

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

			p, _ := pem.Decode(der)
			if p == nil {
				telemetry.Error(nil, "decoding private key pem [%s]", usercfg.PrivKeyFile)
				continue
			}

			switch p.Type {
			case "PRIVATE KEY":
				key, err := x509.ParsePKCS8PrivateKey(p.Bytes)
				if err != nil {
					telemetry.Error(err, "parsing PKCS8 private key file [%s]", usercfg.PrivKeyFile)
					continue
				}
				serverUser.privKey = key
			case "RSA PRIVATE KEY":
				key, err := x509.ParsePKCS1PrivateKey(p.Bytes)
				if err != nil {
					telemetry.Error(err, "parsing PKCS1 private key file [%s]", usercfg.PrivKeyFile)
					continue
				}
				serverUser.privKey = key
			default:
				telemetry.Error(nil, "unknown private key type %s in file [%s]", p.Type, usercfg.PrivKeyFile)
			}
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
			id:       path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/outbox", page.SubPath, usercfg.Name)),
			username: usercfg.Name,
			rssURL:   usercfg.SourceURL,
			notes:    store.(storage.Notes),
		}

		serverUser.inbox = ActivityInbox{
			id:             path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/inbox", page.SubPath, usercfg.Name)),
			ownerID:        serverUser.meta.UserID,
			followers:      store.(storage.Followers),
			pipeline:       svc.Pipeline,
			privKey:        serverUser.privKey,
			pubKeyID:       umeta.UserPublicKeyID,
			actorCache:     ccache.New(ccache.Configure[activity.Actor]()),
			acceptUnsigned: cfg.Server.AcceptAll,
		}

		if err := serverUser.store.Open(); err != nil {
			telemetry.Error(err, "opening sqlite database [%s]", dbName)
		} else {
			svc.users = append(svc.users, serverUser)
		}
	}

	// configure web handlers
	svc.addHandlers()

	svc.Server = http.Server{
		Handler:      svc.router,
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
	return svc
}

type RequestLogger struct {
	Handler http.HandlerFunc
}

func (rl RequestLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "incoming")
	rl.Handler(w, r)
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
