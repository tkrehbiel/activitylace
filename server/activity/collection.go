package activity

type OrderedNoteCollection struct {
	Context  interface{} `json:"@context,omitempty"`
	Type     string      `json:"type"`
	ID       string      `json:"id"`
	NumItems int         `json:"numItems,omitempty"`
	Items    []Note      `json:"orderedItems,omitempty"` // TODO: should support any object
}
