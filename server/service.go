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
	"github.com/tkrehbiel/activitylace/server/data"
	"github.com/tkrehbiel/activitylace/server/page"
	"github.com/tkrehbiel/activitylace/server/telemetry"
)

type ActivityService struct {
	Config Config
	Server http.Server
	router *mux.Router
	meta   page.MetaData
	outbox []ActivityOutbox
	inbox  []ActivityInbox
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

	// inboxes and outboxes must be initialized before this
	if len(s.inbox) == 0 || len(s.outbox) == 0 {
		telemetry.Error(nil, "inboxes and outboxes must be initialized")
		return
	}

	for i, usercfg := range s.Config.Users {
		umeta := s.meta.NewUserMetaData(usercfg.Name)
		umeta.UserDisplayName = usercfg.DisplayName
		umeta.UserType = "Person"
		if usercfg.Type != "" {
			umeta.UserType = usercfg.Type
		}
		umeta.LatestNotes = s.outbox[i].GetLatestNotes(10)

		pg := page.ActorEndpoint // copy
		pg.Path = fmt.Sprintf("/%s/%s", page.SubPath, usercfg.Name)
		s.addPageHandler(page.NewStaticPage(pg), umeta)

		// TODO: This should be a dynamic page since it should include latest activity
		pg = page.ProfilePage // copy
		pg.Path = fmt.Sprintf("/profile/%s", usercfg.Name)
		s.addPageHandler(page.NewStaticPage(pg), umeta)
	}

	// Dynamic handlers

	// Outbox handlers for each user
	for i, outbox := range s.outbox {
		path := fmt.Sprintf("/%s/%s/outbox", page.SubPath, outbox.username)
		s.router.HandleFunc(path, s.outbox[i].ServeHTTP).Methods("GET") // TODO: filter by Accept
	}

	// Inbox handlers for each user
	for i, inbox := range s.inbox {
		path := fmt.Sprintf("/%s/%s/inbox", page.SubPath, inbox.username)
		s.router.HandleFunc(path, RequestLogger{Handler: s.inbox[i].GetHTTP}.ServeHTTP).Methods("GET")   // TODO: filter by Accept
		s.router.HandleFunc(path, RequestLogger{Handler: s.inbox[i].PostHTTP}.ServeHTTP).Methods("POST") // TODO: filter by Accept
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
	for i := range s.outbox {
		s.outbox[i].storage.Close()
	}
	for i := range s.inbox {
		s.inbox[i].storage.Close()
	}
	telemetry.LogCounters()
}

func (s *ActivityService) ListenAndServe() error {
	// Spawn RSS feed watcher goroutines
	for _, outbox := range s.outbox {
		go outbox.WatchRSS(context.Background())
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
		outbox: make([]ActivityOutbox, 0),
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
	for _, user := range cfg.Users {
		// TODO: create UserMetaData here so we can reference it

		outboxName := fmt.Sprintf("outbox_%s.db", user.Name)
		outbox := ActivityOutbox{
			id:       path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/outbox", page.SubPath, user.Name)),
			username: user.Name,
			rssURL:   user.SourceURL,
			storage:  data.NewSQLiteCollection("outbox", outboxName),
		}
		if err := outbox.storage.Open(); err != nil {
			telemetry.Error(err, "opening sqlite database [%s]", outboxName)
		} else {
			svc.outbox = append(svc.outbox, outbox)
		}

		inboxName := fmt.Sprintf("inbox_%s.db", user.Name)
		inbox := ActivityInbox{
			id:       path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/inbox", page.SubPath, user.Name)),
			username: user.Name,
			storage:  data.NewSQLiteCollection("inbox", inboxName),
		}
		if err := inbox.storage.Open(); err != nil {
			telemetry.Error(err, "opening sqlite database [%s]", inboxName)
		} else {
			svc.inbox = append(svc.inbox, inbox)
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
