package page

import (
	"fmt"
	"net/url"
)

const SubPath = "activity"

// MetaData contains server information typically used in templates
type MetaData struct {
	URL      string // full server URL with scheme, host, port
	HostName string // server hostname
}

// These functions set the base paths for endpoints

// WebFingerAccount gets a webfinger user account name
func (m MetaData) WebFingerAccount(name string) string {
	return fmt.Sprintf("acct:%s@%s", name, m.HostName)
}

// ActorURL gets an ActivtyPub Actor ID and endpoint URL
func (m MetaData) ActorURL(name string) string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("%s/%s", SubPath, name))
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
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("%s/%s/inbox", SubPath, m.UserName))
	return s
}

func (m UserMetaData) OutboxURL() string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("%s/%s/outbox", SubPath, m.UserName))
	return s
}

func (m UserMetaData) FollowingURL() string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("%s/%s/following", SubPath, m.UserName))
	return s
}

func (m UserMetaData) FollowersURL() string {
	s, _ := url.JoinPath(m.URL, fmt.Sprintf("%s/%s/followers", SubPath, m.UserName))
	return s
}
