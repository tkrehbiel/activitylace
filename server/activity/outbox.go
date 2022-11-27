package activity

type OrderedCollection struct {
	Context  string `json:"@context,omitempty"`
	Type     string `json:"type"`
	Identity string `json:"id"`
	NumItems int    `json:"numItems"`
	Items    []Note `json:"orderedItems"` // TODO: should support any object
}
