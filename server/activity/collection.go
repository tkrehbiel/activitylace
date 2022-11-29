package activity

type OrderedCollection struct {
	Context  string `json:"@context,omitempty"`
	Type     string `json:"type"`
	Identity string `json:"id"`
	NumItems int    `json:"numItems,omitempty"`
	Items    []Note `json:"orderedItems,omitempty"` // TODO: should support any object
}
