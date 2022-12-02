package activity

type Object struct {
	Context interface{} `json:"@context,omitempty"`
	Type    string      `json:"type"`
	ID      string      `json:"id"`
	To      []string    `json:"to,omitempty"`
	BTo     []string    `json:"bto,omitempty"`
	CC      []string    `json:"cc,omitempty"`
	BCC     []string    `json:"bcc,omitempty"`
	Actor   interface{} `json:"actor,omitempty"`
	Object  interface{} `json:"object,omitempty"`
	Target  interface{} `json:"target,omitempty"`
}

type Note struct {
	Context   interface{} `json:"@context,omitempty"`
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Title     string      `json:"title,omitempty"`
	Content   string      `json:"content,omitempty"`
	Published string      `json:"published"`
	URL       string      `json:"url"` // plain url string
}

type Actor struct {
	Context   interface{} `json:"@context,omitempty"`
	Type      string      `json:"type"`
	ID        string      `json:"id"`
	Inbox     string      `json:"inbox"`
	Outbox    string      `json:"outbox"`
	Following string      `json:"following,omitempty"`
	Followers string      `json:"followers,omitempty"`
	Liked     string      `json:"liked,omitempty"`
	Preferred string      `json:"preferredUsername,omitempty"`
}
