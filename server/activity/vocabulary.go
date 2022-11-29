package activity

// ActivityPub and ActivityStreams vocabulary

const (
	IDProperty        = "id"
	PublishedProperty = "published"
)

const (
	Context     = "https://www.w3.org/ns/activitystreams"
	ContentType = `application/activity+json; profile="https://www.w3.org/ns/activitystreams"`
)

const (
	NoteType              = "Note"
	LinkType              = "Link"
	OrderedCollectionType = "OrderedCollection"
)

const (
	// ActivityPub time format string
	TimeFormat = "2006-01-02T15:04:05Z"
)
