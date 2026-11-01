package ports

import "time"

// BigRow is one bigfile held in a file: the index entry the daemon's database
// keeps, which is the source of truth for what exists. A bigfile held in
// memory has no row. See Plans/R1.0-Release/bigfile.md#trust-and-durability.
type BigRow struct {
	Record  uint32
	Name    string
	File    string // the file's name inside the record's directory
	Size    int64
	SHA256  string
	Held    string
	Expires time.Time
}

// BigIndex keeps the rows. Every change commits before it returns, so a row
// is visible after a restart exactly when its call answered.
type BigIndex interface {
	// BigRows is every row.
	BigRows() ([]BigRow, error)
	// BigRowsOf is one record's rows, sorted by name.
	BigRowsOf(record uint32) ([]BigRow, error)
	// BigRowGet reads one row.
	BigRowGet(record uint32, name string) (BigRow, bool, error)
	// BigRowPut writes a row, replacing the one under the same name, which it
	// returns so its file can be removed.
	BigRowPut(row BigRow) (old BigRow, replaced bool, err error)
	// BigRowDelete removes one row and returns it.
	BigRowDelete(record uint32, name string) (BigRow, bool, error)
	// BigRowsDropRecord removes every row of one record and returns them.
	BigRowsDropRecord(record uint32) ([]BigRow, error)
	// BigRowsDropExpired removes the rows expired at now and returns them.
	BigRowsDropExpired(now time.Time) ([]BigRow, error)
}
