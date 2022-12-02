package activity

type Activity struct {
	Context string      `json:"@context,omitempty"`
	Type    string      `json:"type"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	Actor   interface{} `json:"actor,omitempty"`
	Object  interface{} `json:"object,omitempty"`
	Target  interface{} `json:"target,omitempty"`
}
