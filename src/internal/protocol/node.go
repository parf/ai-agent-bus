package protocol

// NodeIdentity is the public description of a daemon. Private status, caller
// identity, per-record data and credentials do not belong in this response.
type NodeIdentity struct {
	Version string `json:"version"`
	Build   string `json:"build_info"`
	Owner   string `json:"owner"`
	Up      string `json:"up"`
	// Empty Host means the OS did not provide a hostname.
	Host     string          `json:"host"`
	Load     *[3]float64     `json:"load"`
	Messages []MessageWindow `json:"messages"`
}

// MessageWindow counts accepted inbox deliveries and dequeues, not API calls.
// Observed is the actual sampled span, which can differ from Window.
type MessageWindow struct {
	Window    string `json:"window"`
	Observed  string `json:"observed"`
	Available bool   `json:"available"`
	Accepted  int    `json:"accepted"`
	Dequeued  int    `json:"dequeued"`
}
