// Starts an http server to respond to ActivityPub requests.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/tkrehbiel/activitylace/server"
)

func validUser(username string) bool {
	switch username {
	case "blog":
		return true
	}
	return false
}

const activityPubMimeType = "application/activity+json"
const activityStreamMimeType = `application/ld+json`

func readConfig(filename string) server.Config {
	var cfg server.Config
	b, err := os.ReadFile(filename)
	if err != nil {
		log.Println(err)
	} else {
		c, err := server.ReadConfig(b)
		if err != nil {
			log.Println(err)
		}
		cfg = c
	}

	return cfg
}

func main() {
	configFile := flag.String("config", "config.json", "config json file")
	host := flag.String("host", "", "this hostname")
	pubCert := flag.String("cert", "", "public certificate")
	privCert := flag.String("key", "", "private key")
	port := flag.Int("port", 0, "listen port")

	flag.Parse()

	cfg := readConfig(*configFile)
	if *host != "" {
		cfg.Server.HostName = *host
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *pubCert != "" {
		cfg.Server.Certificate = *pubCert
	}
	if *privCert != "" {
		cfg.Server.PrivateKey = *privCert
	}

	srv := server.NewService(cfg)

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	srv.Server.Shutdown(ctx)
	log.Println("server stopping")
	os.Exit(0)
}

/* type itemHandler struct {
	storage data.Collection
}

func (i *itemHandler) NewItem(item rss.Item) {
	fmt.Println(item.Title)
	obj := &activity.Note{
		Context:   activity.Context,
		Type:      activity.NoteType,
		Identity:  item.ID,
		Content:   item.Title,
		Published: item.Published.Format(activity.TimeFormat),
		URL:       item.URL,
	}
	fmt.Println(string(obj.JSON()))
	if err := i.storage.Upsert(context.TODO(), obj); err != nil {
		fmt.Println(err)
	}
}

func (i *itemHandler) StatusCode(code int) {
	fmt.Println("result code", code)
}

func watch(url string) {
	outbox := itemHandler{
		storage: data.NewSQLiteCollection("outbox", "collection_outbox.db"),
	}
	err := outbox.storage.Open()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer outbox.storage.Close()

	w := rss.NewFeedWatcher(url, &outbox)

	// Load previously-stored items
	objects, err := outbox.storage.SelectAll(context.Background())
	if err == nil {
		for _, obj := range objects {
			fmt.Println("loading", obj.ID())
			note := data.ToNote(obj)
			item := rss.Item{
				ID:        obj.ID(),
				Published: obj.Timestamp(),
				Title:     note.Title,
				Content:   note.Content,
			}
			w.AddKnown(item)
		}
	}

	fmt.Println("watching", url)
	w.Watch(context.Background(), time.Minute)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("received home request")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "<html><body>nothing to see here</body></html>")
}
*/
/*
func outboxBody(name string) []byte {
	// GET outbox
	x := map[string]interface{}{
		"@context": []interface{}{
			"https://www.w3.org/ns/activitystreams",
			map[string]interface{}{
				"@language": "en",
			},
		},
		"type":       "OrderedCollection",
		"totalItems": 1,
		"orderedItems": []interface{}{
			map[string]interface{}{
				"type":      "Note",
				"published": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
				"content":   "Hello. I'm a developer testing an ActivityPub implementation for my blog. Pay no attention.",
				"inReplyTo": nil,
			},
		},
	}
	b, err := json.Marshal(&x)
	if err != nil {
		return nil
	}
	log.Println(string(b))
	return b
}

// (a POST to the outbox is the user broadcasting a new post)

func outboxHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("received Outbox request")
	//log.Printf("headers: %v\n", r.Header)
	// if r.Header.Get("Content-Type") != activityPubMimeType {
	// 	http.NotFound(w, r)
	// 	return
	// }
	name := mux.Vars(r)["username"]
	if !validUser(name) {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	w.Write(outboxBody(name))
}

func inboxHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("received Inbox request, refusing for now")
	w.WriteHeader(http.StatusMethodNotAllowed)
}
*/
