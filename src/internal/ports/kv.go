package ports

// The key-value port: each record's store of named values, kept in the
// daemon's database and never in its memory. See Plans/R1.0-Release/kv.md.

// KVKind is which of a record's three stores a name is in. Each is its own
// namespace: a string and an int under one name are two values.
type KVKind string

const (
	KVString KVKind = "string"
	KVInt    KVKind = "int"
	KVJSON   KVKind = "json"
)

// KVValue is one stored value: Bytes for a string or a JSON value, Int for an
// int.
type KVValue struct {
	Bytes []byte
	Int   int64
}

// KVEdit is what an edit decides: write Value, remove the name, or leave it.
type KVEdit struct {
	Write, Delete bool
	Value         KVValue
}

// KVStore keeps the values. Every edit runs as one transaction: the value it
// is handed is the one it replaces, whoever else is editing, and the edit
// commits before the call returns.
type KVStore interface {
	// KVGet reads one value; found is false when the name is absent.
	KVGet(kind KVKind, record uint32, name string) (v KVValue, found bool, err error)
	// KVEdit reads one value and how many names record holds in kind, hands
	// both to edit and applies what edit returns, in one transaction. It
	// writes nothing for a record the store does not hold, answering
	// ErrKVNoRecord, so a record removed meanwhile keeps no value behind it.
	// An error from edit is returned and nothing changes.
	KVEdit(kind KVKind, record uint32, name string, edit func(old KVValue, found bool, names int) (KVEdit, error)) error
	// KVOrphans lists record IDs that hold values but name no stored record.
	KVOrphans() ([]uint32, error)
	// KVList is every name record holds in kind, sorted, each with its
	// value's size and the first KVPreview bytes of it; an int's Preview is
	// the value itself.
	KVList(kind KVKind, record uint32) ([]KVEntry, error)
	// KVCounts is how many names each record holding any has, per kind.
	KVCounts() (map[uint32]KVCount, error)
}

// KVPreview is how much of a string or JSON value a listing carries.
const KVPreview = 200

// KVEntry is one name in a listing.
type KVEntry struct {
	Name    string
	Size    int
	Preview KVValue
}

// KVCount is how many names one record holds of each kind.
type KVCount struct{ String, Int, JSON int }

// ErrKVNoRecord is a value written for a record the store does not hold.
var ErrKVNoRecord = kvError("no stored record holds that store")

type kvError string

func (e kvError) Error() string { return string(e) }
