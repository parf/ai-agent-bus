package protocol

// NodeIdentity is the public description of a daemon. Private status, caller
// identity, per-record data and credentials do not belong in this response.
type NodeIdentity struct {
	Version string `json:"version"`
	Build   string `json:"build_info"`
	Owner   string `json:"owner"`
	Up      string `json:"up"`
	// Empty Host means the OS did not provide a hostname.
	Host  string     `json:"host"`
	Calls *CallStats `json:"calls"`
}

// CallStats counts HTTP requests entering daemon handlers, including refused
// calls and long polls still in flight. It does not count completed work.
type CallStats struct {
	Total   uint64       `json:"total"`
	Windows []CallWindow `json:"windows"`
}

// Observed is the actual sampled span, which can differ from Window.
type CallWindow struct {
	Window    string `json:"window"`
	Observed  string `json:"observed"`
	Available bool   `json:"available"`
	Count     uint64 `json:"count"`
}
