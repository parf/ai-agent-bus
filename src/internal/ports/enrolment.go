package ports

// What enrolment needs from outside, in two pieces on purpose.
// See docs/01-identity-and-roles.md#registration.

// DirectoryEntry is what one directory lookup states. PersonName is optional,
// but when present it comes from the same trusted adapter as Keys rather than
// from the caller asking to enrol.
type DirectoryEntry struct {
	Keys       []string
	PersonName string
}

// Directory is a lookup and nothing more: which public keys and optional
// person name a login publishes. **Fetching a key is not authentication** —
// proving the caller holds the private half is a separate step, and it
// deliberately does not live here, so that every future directory is a
// lookup to write and not a protocol to get right.
type Directory interface {
	Lookup(login string) (DirectoryEntry, error)
}

// Signatures says whether a message was signed by the holder of one of keys.
// The system's own tool does this; nothing here is ours to invent
// (docs/10-modules.md#the-rule).
type Signatures interface {
	Verify(id, message, signature string, keys []string) error
}
