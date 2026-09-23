package protocol

import "time"

const (
	DirectoryUser       = "user"
	DirectoryRecord     = "record"
	DirectoryCredential = "credential"
)

// User is an administrator-vouched profile, separate from service registration.
type User struct {
	// ID is the internal user_id: stable, persisted, never reused and never
	// the public identity (docs/constitution.md#-user).
	ID         uint32 `json:"-"`
	Name       string `json:"name"`
	PersonName string `json:"person_name,omitempty"`
	Email      string `json:"email,omitempty"`
	GithubUser string `json:"github_user,omitempty"`
	// GitHubProfileAt dates the provider answer. PersonName, Email, company,
	// location and Twitter/X are ordinary editable profile fields after import;
	// they are not provenance claims.
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
	// Status is active or inactive (docs/constitution.md#-user).
	Status string `json:"status"`
	// Created and Updated are the system's lifecycle stamps.
	Created time.Time `json:"created_at,omitzero"`
	Updated time.Time `json:"updated_at,omitzero"`
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
