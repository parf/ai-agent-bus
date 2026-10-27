package ports

// StoredKV carries a complete value, never a listing's truncated preview.
type StoredKV struct {
	RecordID uint32
	Kind     KVKind
	Name     string
	Value    KVValue
}

// TransferState is a single store read of private values and credentials.
// Registry records are held stable by core during this read.
type TransferState struct {
	KV          []StoredKV
	Credentials []Credential
}
type TransferStore interface {
	ExportState(recordIDs []uint32, principals []string) (TransferState, error)
}

// CredentialImport serializes token issuance/rotation across the durable commit
// and publication. The callback writes the already-validated store change; it
// must not call the credential index again. No token values enter an error.
type CredentialImport interface {
	CommitImport(values map[string]Credential, pairs map[string]*CredentialPair, commit func() error) error
}
