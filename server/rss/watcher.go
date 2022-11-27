package rss

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/mmcdole/gofeed"
)

// Item is our internal, minimalist representation of a blog post
type Item struct {
	ID        string
	Title     string
	Published time.Time
	Updated   time.Time
	Content   string
	URL       string
	Hashtags  []string
}

// ItemHandler is an interface that defines what to do when new RSS items are discovered
type ItemHandler interface {
	StatusCode(code int) // called after any fetch, normally either 200 (OK) or 304 (NotModified)
	NewItem(item Item)   // a new feed item is discovered
}

// FeedWatcher implements a small service to watch an RSS feed and discover new activity
type FeedWatcher struct {
	URL     string
	Client  http.Client
	Handler ItemHandler

	itemParser   ItemParser
	etag         string
	lastModified string
	known        map[string]time.Time // known guids to track new and updated items
}

type ItemParser interface {
	Parse(r io.Reader) ([]Item, error)
}

type gofeedParser struct {
	parser *gofeed.Parser // helper to parse rss, atom, json
}

// Parse an HTTP body as an RSS feed (or Atom or JSON, it turns out)
func (p gofeedParser) Parse(reader io.Reader) ([]Item, error) {
	feed, err := p.parser.Parse(reader)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0)
	for _, item := range feed.Items {
		parsedItem := Item{
			ID:      item.Link,
			Title:   item.Title,
			Content: item.Description,
			URL:     item.Link,
			// TODO: Tags
		}
		if item.PublishedParsed != nil {
			parsedItem.Published = *item.PublishedParsed
		} else {
			// Some feeds have mangled dates
			// e.g. CNN "Sat, 26 Nov 2022 11:04:03 GMT"
			// TODO: Should be smarter than this
			parsedItem.Published = time.Now().UTC()
		}
		if item.UpdatedParsed != nil {
			parsedItem.Updated = *item.UpdatedParsed
		} else {
			parsedItem.Updated = parsedItem.Published
		}
		items = append(items, parsedItem)
	}
	return items, nil
}

// Check remote RSS feed for changes
func (c *FeedWatcher) Check(ctx context.Context) error {
	r, err := http.NewRequestWithContext(ctx, "GET", c.URL, nil)
	if err != nil {
		return err
	}
	if c.lastModified != "" {
		r.Header.Set("If-Modified-Since", c.lastModified)
		r.Header.Set("If-None-Match", c.etag)
	}

	resp, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	c.Handler.StatusCode(resp.StatusCode)
	if resp.StatusCode == http.StatusNotModified {
		// Feed not modified, nothing to do
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response code %d", resp.StatusCode)
	}

	newItems, err := c.parseItems(resp.Body)
	if err != nil {
		return err
	}

	for _, item := range newItems {
		c.Handler.NewItem(item)
	}

	if resp.Header.Get("ETag") != "" {
		c.etag = resp.Header.Get("ETag")
		c.lastModified = resp.Header.Get("Last-Modified")
	}

	return nil
}

func (c *FeedWatcher) AddKnown(item Item) {
	c.known[item.ID] = item.Updated
}

func (c *FeedWatcher) parseItems(body io.Reader) ([]Item, error) {
	allItems, err := c.itemParser.Parse(body)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	newItems := make([]Item, 0)
	for _, item := range allItems {
		if _, ok := c.known[item.ID]; !ok {
			c.known[item.ID] = item.Updated
			newItems = append(newItems, item)
		}
	}

	// sort from oldest to newest
	sort.Slice(newItems, func(i int, j int) bool {
		return newItems[i].Published.Before(newItems[j].Published)
	})

	return newItems, nil
}

func (c *FeedWatcher) Watch(ctx context.Context, period time.Duration) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	c.Check(ctx)
	for {
		select {
		case <-ctx.Done():
			// Parent context cancelled somehow
			log.Println("context ended", ctx.Err())
			return
		case <-sigChannel:
			// CTRL-C
			log.Println("received end signal")
			return
		case <-ticker.C:
			err := c.Check(ctx)
			if err != nil {
				// We just ignore the error for now
				// TODO: Should be smarter
				log.Println("checking feed", c.URL, err)
			}
		}
	}
}
func NewFeedWatcher(url string, handler ItemHandler) FeedWatcher {
	return FeedWatcher{
		URL:     url,
		Client:  http.Client{},
		Handler: handler,
		itemParser: gofeedParser{
			parser: gofeed.NewParser(),
		},
		known: make(map[string]time.Time),
	}
}
