package page

import (
	"fmt"
	"net/url"
)

// MetaData contains server information typically used in templates
type MetaData struct {
	URL      string // full server URL with scheme, host, port
	Scheme   string // http or https
	HostName string // server hostname
	Port     int    // server port
}

// These functions set the base paths for endpoints

// WebFingerAccount gets a webfinger user account name
func (m MetaData) WebFingerAccount(name string) string {
	return fmt.Sprintf("acct:%s@%s", name, m.HostName)
}

// ActorURL gets an ActivtyPub Actor ID and endpoint URL
func (m MetaData) ActorURL(name string) string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("a/%s", name))
	return s
}

// ProfileURL gets an HTML profile page for a user name
func (m MetaData) ProfileURL(name string) string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("profile/%s", name))
	return s
}

func (m MetaData) NewUserMetaData(name string) UserMetaData {
	return UserMetaData{
		MetaData:       m,
		UserName:       name,
		UserID:         m.ActorURL(name),
		UserProfileURL: m.ProfileURL(name),
	}
}

func NewMetaData(u *url.URL) MetaData {
	return MetaData{
		URL:      u.String(),
		Scheme:   u.Scheme,
		HostName: u.Hostname(),
	}
}

// UserMetaData contains user information typically used in templates
type UserMetaData struct {
	MetaData
	UserName        string // Plain undecorated username
	UserID          string // ActivityPub user ID (an URL for application/json+activity)
	UserProfileURL  string // HTML user profile page (an URL)
	UserDisplayName string
	UserSummary     string
	UserType        string // ActivityPub Actor type (Person, Organization, etc.)
	AvatarURL       string
	AvatarWidth     int
	AvatarHeight    int
}

func (m UserMetaData) InboxURL() string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("a/%s/inbox", m.UserName))
	return s
}

func (m UserMetaData) OutboxURL() string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("a/%s/outbox", m.UserName))
	return s
}
