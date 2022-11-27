package page

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"
)

// StaticPage configures how to render a static web page response.
// Static pages always return the same thing when requested.
type StaticPage struct {
	Path        string // Server path to the static page
	Accept      string // Accept header required to receive this page
	ContentType string // ContentType of this page
	Template    string // Golang template to create the page
}

// internalStaticPage contains pre-processed data for rendering a static page response
type internalStaticPage struct {
	source   StaticPage // Configuration for this static page
	rendered []byte     // Rendered template, i.e. the static page to serve
}

// NewStaticPage returns an interface for static page handling.
// It turned out to be more convenient to use an interface, so
// this function essentially turns a StaticPage configuration into
// an interface to pass into other functions.
func NewStaticPage(page StaticPage) StaticPageHandler {
	return &internalStaticPage{
		source: page,
	}
}

// StaticPageHandler is an http.Handler and extras for setting up and rendering a static page.
type StaticPageHandler interface {
	http.Handler
	Init(any) error // Initialize the page by processing its template before rendering
	Path() string   // Path at which the page should respond
	Accept() string // Accept header required to respond
}

func (s internalStaticPage) Path() string {
	return s.source.Path
}

func (s internalStaticPage) Accept() string {
	return s.source.Accept
}

func (s *internalStaticPage) Init(meta any) error {
	t, err := template.New("").Parse(strings.TrimSpace(s.source.Template))
	if err != nil {
		s.rendered = []byte(fmt.Sprintf("template error: %s", err))
		return err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, meta); err != nil {
		s.rendered = []byte(fmt.Sprintf("executing template: %s", err))
		return err
	}
	s.rendered = buf.Bytes()
	return nil
}

// ServeHTTP is an http handler to serve the rendered static page.
// Must call Init() on the static page first, otherwise
// this will return a 500 Internal Server Error.
func (s internalStaticPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logRequest("StaticPage.ServeHTTP", r)
	if s.rendered == nil {
		// Server error because we didn't render a page yet.
		// TODO: Would like to render it here on-demand but there's no access to metadata.
		log.Println("no static rendered content")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", s.source.ContentType)
	w.Write(s.rendered)
}

func logRequest(message string, r *http.Request) {
	log.Println(message, r.URL.String())
}
