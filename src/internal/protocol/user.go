package protocol

// User is a maintainer-vouched profile, separate from service registration.
type User struct {
	Name       string `json:"name"`
	PersonName string `json:"person_name,omitempty"`
	Email      string `json:"email,omitempty"`
	GithubUser string `json:"github_user,omitempty"`
	State      string `json:"state"`
	// Derived by the daemon for the current visitor, never accepted as claims.
	Maintainer  bool     `json:"maintainer,omitempty"`
	DaemonOwner bool     `json:"daemon_owner,omitempty"`
	CanEdit     bool     `json:"can_edit,omitempty"`
	CanActivate bool     `json:"can_activate,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	Services    []string `json:"services,omitempty"`
}
