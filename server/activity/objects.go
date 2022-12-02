package activity

type Note struct {
	Context   string `json:"@context,omitempty"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content,omitempty"`
	Published string `json:"published"`
	URL       string `json:"url"` // plain url string
}
