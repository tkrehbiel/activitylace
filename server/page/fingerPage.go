package page

import (
	"log"
	"net/http"
	"regexp"
)

// MultiStaticPage can render one of many different StaticPages depending on a request param.
// TODO: Generalize this, it only works for webfinger right now.
type MultiStaticPage struct {
	StaticPage
	HostName string
	Pages    map[string]StaticPageHandler
}

var WellKnownWebFinger = MultiStaticPage{
	StaticPage: StaticPage{
		Path:        "/.well-known/webfinger",
		Accept:      "*/*",
		ContentType: "application/json",
	},
}

var WebFingerAccount = StaticPage{
	ContentType: "application/json",
	Template: `
{
	"subject": "{{ .WebFingerAccount .UserName }}",
	"aliases": [
		"{{ .UserID }}",
		"{{ .UserProfileURL }}"
	],
	"links": [
		{
			"rel": "self",
			"type": "application/activity+json",
			"href": "{{ .UserID }}"
		},
		{
			"rel": "http://webfinger.net/rel/profile-page",
			"type": "text/html",
			"href": "{{ .UserProfileURL }}"
		}
	]
}`,
}

var acctRegex = regexp.MustCompile(`acct:(.+)@(.+)`)

// Add a user resource to be served
func (s *MultiStaticPage) Add(username string, meta MetaData) {
	s.HostName = meta.HostName
	if s.Pages == nil {
		s.Pages = make(map[string]StaticPageHandler)
	}
	userMeta := UserMetaData{
		MetaData:       meta,
		UserName:       username,
		UserID:         meta.ActorURL(username),
		UserProfileURL: meta.ProfileURL(username),
	}
	userPage := NewStaticPage(WebFingerAccount) // copy
	err := userPage.Init(userMeta)
	if err == nil {
		s.Pages[username] = userPage
	}
}

func (s MultiStaticPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This one specifically uses the resource query parameter to lookup webfinger resources.
	logRequest("MultiStaticPage.ServeHTTP", r)
	resource := r.URL.Query().Get("resource")
	if resource != "" {
		matches := acctRegex.FindSubmatch([]byte(resource))
		if len(matches) > 0 {
			hostname := string(matches[2])
			username := string(matches[1])
			if hostname == s.HostName && s.Pages[username] != nil {
				s.Pages[username].ServeHTTP(w, r)
				return
			} else {
				log.Printf("WARNING: unrecognized webfinger resource request for [%s]", resource)
			}
		} else {
			log.Printf("WARNING: malformed webfinger resource request [%s]", resource)
		}
	} else {
		log.Println("WARNING: webfinger request without resource param")
	}
	w.WriteHeader(http.StatusNotFound)
}

func (s MultiStaticPage) Path() string {
	return s.StaticPage.Path
}

func (s MultiStaticPage) Accept() string {
	return s.StaticPage.Accept
}

func (s MultiStaticPage) Init(meta any) error {
	return nil // no template here, only user templates
}
