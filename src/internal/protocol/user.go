package protocol

import "time"

const (
	DirectoryUser       = "user"
	DirectoryRecord     = "record"
	DirectoryCredential = "credential"
)

// User is an administrator-vouched profile, separate from service registration.
type User struct {
	Name       string `json:"name"`
	PersonName string `json:"person_name,omitempty"`
	Email      string `json:"email,omitempty"`
	GithubUser string `json:"github_user,omitempty"`
	// GitHubProfileAt dates the provider answer below. PersonName and Email are
	// imports into the ordinary profile fields above; they are not mirrors and
	// are deliberately not cleared when the provider later omits them.
	GithubProfileAt       time.Time `json:"github_profile_at,omitempty,omitzero"`
	GithubCompany         string    `json:"github_company,omitempty"`
	GithubLocation        string    `json:"github_location,omitempty"`
	GithubTwitterUsername string    `json:"github_twitter_username,omitempty"`
	GithubAvatarURL       string    `json:"github_avatar_url,omitempty"`
	GithubGravatarID      string    `json:"github_gravatar_id,omitempty"`
	// PhotoPNG is provider-controlled imagery only after the trusted adapter
	// has bounded, decoded and re-encoded it. Browsers never receive a remote
	// image URL as an img source.
	PhotoPNG       []byte    `json:"photo_png,omitempty"`
	PhotoSource    string    `json:"photo_source,omitempty"`
	PhotoFetchedAt time.Time `json:"photo_fetched_at,omitempty,omitzero"`
	State          string    `json:"state"`
	// Derived by the daemon for the current visitor, never accepted as claims.
	Kind          string   `json:"kind"`
	CanRemove     bool     `json:"can_remove,omitempty"`
	Administrator bool     `json:"administrator,omitempty"`
	DaemonOwner   bool     `json:"daemon_owner,omitempty"`
	CanEdit       bool     `json:"can_edit,omitempty"`
	CanSetEmail   bool     `json:"can_set_email,omitempty"`
	CanActivate   bool     `json:"can_activate,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Services      []string `json:"services,omitempty"`
}
