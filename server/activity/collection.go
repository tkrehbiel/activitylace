package activity

type OrderedNoteCollection struct {
	Context  string `json:"@context,omitempty"`
	Type     string `json:"type"`
	ID       string `json:"id"`
	NumItems int    `json:"numItems,omitempty"`
	Items    []Note `json:"orderedItems,omitempty"` // TODO: should support any object
}
