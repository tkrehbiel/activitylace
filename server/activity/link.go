package activity

type Link struct {
	Context   string `json:"@context,omitempty"`
	Type      string `json:"type"`
	HRef      string `json:"href"`
	MediaType string `json:"mediatype,omitempty"`
}
