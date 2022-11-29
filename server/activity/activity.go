package activity

type ActivityHeader struct {
	Type     string `json:"type"`
	Identity string `json:"id"`
	Actor    string `json:"actor"`
	Context  string `json:"context"`
}
