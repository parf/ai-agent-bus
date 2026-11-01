package sqlite

// The bigfile index: one row per bigfile held in a file, keyed by the record's
// internal ID, which is never reused (Plans/R1.0-Release/bigfile.md#trust-and-durability).

import (
	"database/sql"
	"errors"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

const bigCols = `record_id, name, file, size, sha256, held, expires`

func scanBig(sc interface{ Scan(...any) error }) (ports.BigRow, error) {
	var r ports.BigRow
	var id int64
	var exp int64
	if err := sc.Scan(&id, &r.Name, &r.File, &r.Size, &r.SHA256, &r.Held, &exp); err != nil {
		return ports.BigRow{}, err
	}
	r.Record = uint32(id)
	r.Expires = time.Unix(0, exp).UTC()
	return r, nil
}

func (s *Store) bigQuery(where string, args ...any) ([]ports.BigRow, error) {
	rows, err := s.db.Query(`SELECT `+bigCols+` FROM bigfiles `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.BigRow
	for rows.Next() {
		r, err := scanBig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) BigRows() ([]ports.BigRow, error) {
	return s.bigQuery(`ORDER BY record_id, name`)
}

func (s *Store) BigRowsOf(record uint32) ([]ports.BigRow, error) {
	return s.bigQuery(`WHERE record_id = ? ORDER BY name`, record)
}

func (s *Store) BigRowGet(record uint32, name string) (ports.BigRow, bool, error) {
	r, err := scanBig(s.db.QueryRow(`SELECT `+bigCols+` FROM bigfiles WHERE record_id = ? AND name = ?`, record, name))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.BigRow{}, false, nil
	}
	return r, err == nil, err
}

func (s *Store) BigRowPut(row ports.BigRow) (ports.BigRow, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ports.BigRow{}, false, err
	}
	defer tx.Rollback()
	old, err := scanBig(tx.QueryRow(`SELECT `+bigCols+` FROM bigfiles WHERE record_id = ? AND name = ?`, row.Record, row.Name))
	replaced := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ports.BigRow{}, false, err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO bigfiles (`+bigCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.Record, row.Name, row.File, row.Size, row.SHA256, row.Held, row.Expires.UnixNano()); err != nil {
		return ports.BigRow{}, false, err
	}
	return old, replaced, tx.Commit()
}

func (s *Store) BigRowDelete(record uint32, name string) (ports.BigRow, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ports.BigRow{}, false, err
	}
	defer tx.Rollback()
	old, err := scanBig(tx.QueryRow(`SELECT `+bigCols+` FROM bigfiles WHERE record_id = ? AND name = ?`, record, name))
	if errors.Is(err, sql.ErrNoRows) {
		return ports.BigRow{}, false, nil
	}
	if err != nil {
		return ports.BigRow{}, false, err
	}
	if _, err := tx.Exec(`DELETE FROM bigfiles WHERE record_id = ? AND name = ?`, record, name); err != nil {
		return ports.BigRow{}, false, err
	}
	return old, true, tx.Commit()
}

// bigDrop deletes the rows a query selects and returns them, in one transaction.
func (s *Store) bigDrop(where string, args ...any) ([]ports.BigRow, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT `+bigCols+` FROM bigfiles `+where, args...)
	if err != nil {
		return nil, err
	}
	var out []ports.BigRow
	for rows.Next() {
		r, err := scanBig(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if _, err := tx.Exec(`DELETE FROM bigfiles `+where, args...); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

func (s *Store) BigRowsDropRecord(record uint32) ([]ports.BigRow, error) {
	return s.bigDrop(`WHERE record_id = ?`, record)
}

func (s *Store) BigRowsDropExpired(now time.Time) ([]ports.BigRow, error) {
	return s.bigDrop(`WHERE expires <= ?`, now.UnixNano())
}
