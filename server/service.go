package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/tkrehbiel/activitylace/server/data"
	"github.com/tkrehbiel/activitylace/server/page"
)

type ActivityService struct {
	Config Config
	Server http.Server
	router *mux.Router
	meta   page.MetaData
	outbox []ActivityOutbox
}

func (s ActivityService) addStaticHandlers() {
	s.router.HandleFunc("/", homeHandler).Methods("GET")
	//r.HandleFunc("/activity/{username}", personHandler).Methods("GET")
	//r.HandleFunc("/activity/{username}/inbox", inboxHandler).Methods("POST")
	//r.HandleFunc("/activity/{username}/outbox", outboxHandler).Methods("GET")
	//r.HandleFunc("/@{username}", webHandler).Methods("GET")

	s.addPageHandler(page.NewStaticPage(page.WellKnownNodeInfo), s.meta)
	s.addPageHandler(page.NewStaticPage(page.NodeInfo), s.meta)

	for _, u := range s.Config.Users {
		page.WellKnownWebFinger.Add(u.Name, s.meta)
	}
	s.addPageHandler(&page.WellKnownWebFinger, s.meta)

	for _, usercfg := range s.Config.Users {
		umeta := s.meta.NewUserMetaData(usercfg.Name)
		umeta.UserDisplayName = usercfg.DisplayName
		umeta.UserType = "Person"
		if usercfg.Type != "" {
			umeta.UserType = usercfg.Type
		}
		pg := page.ActorEndpoint // copy
		pg.Path = fmt.Sprintf("/%s/%s", page.SubPath, usercfg.Name)
		s.addPageHandler(page.NewStaticPage(pg), umeta)
		pg = page.ProfilePage // copy
		pg.Path = fmt.Sprintf("/profile/%s", usercfg.Name)
		s.addPageHandler(page.NewStaticPage(pg), umeta)
	}

	// Dynamic handlers

	// Setup outbox for each user
	for i, outbox := range s.outbox {
		path := fmt.Sprintf("/%s/%s/outbox", page.SubPath, outbox.username)
		s.router.HandleFunc(path, s.outbox[i].ServeHTTP).Methods("GET")
	}

	// TODO: actor endpoints
	// TODO: robots.txt
}

func (s ActivityService) addPageHandler(pg page.StaticPageHandler, meta any) {
	pg.Init(meta)
	router := s.router.HandleFunc(pg.Path(), pg.ServeHTTP).Methods("GET")
	if !s.Config.Server.AcceptAll && pg.Accept() != "" && pg.Accept() != "*/*" {
		router.Headers("Accept", pg.Accept())
	}
}

func (s ActivityService) ListenAndServe() error {
	// Spawn RSS feed watcher goroutines
	for _, outbox := range s.outbox {
		go outbox.WatchRSS(context.Background())
	}
	if s.Config.Server.useTLS() {
		log.Println("TLS server starting on port", s.Config.Server.Port)
		return s.Server.ListenAndServeTLS(s.Config.Server.Certificate, s.Config.Server.PrivateKey)
	} else {
		log.Println("HTTP server starting on port", s.Config.Server.Port)
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
		fmt.Println(err)
	}

	// metadata available to page templates
	svc.meta = page.MetaData{
		URL:      cfg.URL,
		HostName: u.Hostname(),
	}

	// configure outboxes
	for _, user := range cfg.Users {
		dbname := fmt.Sprintf("outbox_%s.db", user.Name)
		outbox := ActivityOutbox{
			id:       path.Join(svc.meta.URL, fmt.Sprintf("%s/%s/outbox", page.SubPath, user.Name)),
			username: user.Name,
			rssURL:   user.SourceURL,
			storage:  data.NewSQLiteCollection("outbox", dbname),
		}
		svc.outbox = append(svc.outbox, outbox)
	}

	// configure web handlers
	svc.addStaticHandlers()

	svc.Server = http.Server{
		Handler:      svc.router,
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
	}
	return svc
}

func logRequest(r *http.Request) {
	log.Println(r.URL.String())
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "<html><body>This is activitylace. There's nothing to see here.</body></html>")
}
