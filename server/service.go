package server

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/tkrehbiel/activitylace/server/page"
	"github.com/tkrehbiel/activitylace/server/storage"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityService struct {
	Config Config
	Server http.Server
	router *mux.Router
	meta   page.MetaData
	users  []ActivityUser
}

type ActivityUser struct {
	name   string
	meta   page.UserMetaData
	store  storage.Database
	outbox ActivityOutbox
	inbox  ActivityInbox
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

	for _, user := range s.users {
		// TODO: umeta.LatestNotes = s.outbox[i].GetLatestNotes(10)

		pg := page.ActorEndpoint // copy
		pg.Path = fmt.Sprintf("/%s/%s", page.SubPath, user.name)
		s.addPageHandler(page.NewStaticPage(pg), user.meta)

		// TODO: This should be a dynamic page since it should include latest activity
		pg = page.ProfilePage // copy
		pg.Path = fmt.Sprintf("/profile/%s", user.name)
		s.addPageHandler(page.NewStaticPage(pg), user.meta)

		outpath := fmt.Sprintf("/%s/%s/outbox", page.SubPath, user.name)
		s.router.HandleFunc(outpath, user.outbox.ServeHTTP).Methods("GET") // TODO: filter by Accept

		inpath := fmt.Sprintf("/%s/%s/inbox", page.SubPath, user.name)
		s.router.HandleFunc(inpath, RequestLogger{Handler: user.inbox.GetHTTP}.ServeHTTP).Methods("GET")   // TODO: filter by Accept
		s.router.HandleFunc(inpath, RequestLogger{Handler: user.inbox.PostHTTP}.ServeHTTP).Methods("POST") // TODO: filter by Accept
	}

	// TODO: robots.txt
}

func (s *ActivityService) addPageHandler(pg page.StaticPageHandler, meta any) {
	pg.Init(meta)
	router := s.router.HandleFunc(pg.Path(), pg.ServeHTTP).Methods("GET")
	if !s.Config.Server.AcceptAll && pg.Accept() != "" && pg.Accept() != "*/*" {
		router.Headers("Accept", pg.Accept())
	}
}

// Close anything related to the service before exiting
func (s *ActivityService) Close() {
	for _, user := range s.users {
		user.store.Close()
	}
	telemetry.LogCounters()
}

func (s *ActivityService) ListenAndServe(ctx context.Context) error {
	// Spawn RSS feed watcher goroutines
	for _, user := range s.users {
		go user.outbox.WatchRSS(ctx)
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
		Config: cfg,
		router: mux.NewRouter(),
		users:  make([]ActivityUser, 0),
	}

	u, err := url.Parse(cfg.URL)
	if err != nil {
		telemetry.Error(err, "parsing url [%s]", cfg.URL)
		return svc
	}

	// metadata available to page templates
	svc.meta = page.MetaData{
		URL:      cfg.URL,
		HostName: u.Hostname(),
	}

	// configure inboxes and outboxes
	for _, usercfg := range cfg.Users {
		dbName := fmt.Sprintf("user_%s.db", usercfg.Name)
		store := storage.NewDatabase(dbName)

		serverUser := ActivityUser{
			name:  usercfg.Name,
			meta:  svc.meta.NewUserMetaData(usercfg.Name),
			store: store,
		}

		serverUser.meta.UserDisplayName = usercfg.DisplayName
		serverUser.meta.UserType = "Person"
		if usercfg.Type != "" {
			serverUser.meta.UserType = usercfg.Type
		}

		serverUser.outbox = ActivityOutbox{
			id:       path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/outbox", page.SubPath, usercfg.Name)),
			username: usercfg.Name,
			rssURL:   usercfg.SourceURL,
			notes:    store.(storage.Notes),
		}

		serverUser.inbox = ActivityInbox{
			id:       path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/inbox", page.SubPath, usercfg.Name)),
			username: usercfg.Name,
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
	headers := make([]string, 0)
	for k, v := range r.Header {
		s := fmt.Sprintf("%s: %s", k, strings.Join(v, ", "))
		headers = append(headers, s)
	}
	telemetry.Trace(strings.Join(headers, " | "))

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		telemetry.Error(err, "error reading body")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(buf) > 0 {
		telemetry.Trace(string(buf))
	}
	reader := ioutil.NopCloser(bytes.NewBuffer(buf))
	r.Body = reader
	rl.Handler(w, r)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	telemetry.Request(r, "homeHandler")
	telemetry.Increment("home_requests", 1)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<html><title>activitylace</title>
<body>
<p>This is <a href=\"https://github.com/tkrehbiel/activitylace/\">activitylace</a>,
an experimental ActivityPub server implementation to complement static blogs.
There's nothing to see here.</p>
</body>
</html>`)
}
