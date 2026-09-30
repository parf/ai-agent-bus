package sqlite

// Each record's key-value store: one table per kind, keyed by the record's
// internal ID, which is never reused (Plans/R1.0-Release/kv.md#per-record-storage).

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/parf/ai-agent-bus/internal/ports"
)

var kvTables = map[ports.KVKind]string{ports.KVString: "kv", ports.KVInt: "kv_int", ports.KVJSON: "kv_json"}

func kvTable(kind ports.KVKind) (string, error) {
	t, ok := kvTables[kind]
	if !ok {
		return "", fmt.Errorf("no key-value kind %q", kind)
	}
	return t, nil
}

// dropKV removes the values of the record stored under name, unless its ID is
// keep. Caller holds tx.
func dropKV(tx *sql.Tx, name string, keep uint32) error {
	for _, t := range []string{"kv", "kv_int", "kv_json"} {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE record_id IN (SELECT id FROM records WHERE name = ? AND id <> ?)`, name, keep); err != nil {
			return err
		}
	}
	return nil
}

func scanKV(kind ports.KVKind, row *sql.Row) (ports.KVValue, bool, error) {
	var v ports.KVValue
	var err error
	if kind == ports.KVInt {
		err = row.Scan(&v.Int)
	} else {
		err = row.Scan(&v.Bytes)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.KVValue{}, false, nil
	}
	return v, err == nil, err
}

func kvArg(kind ports.KVKind, v ports.KVValue) any {
	switch kind {
	case ports.KVInt:
		return v.Int
	case ports.KVJSON:
		return string(v.Bytes)
	}
	if v.Bytes == nil {
		return []byte{}
	}
	return v.Bytes
}

// KVGet reads one value.
func (s *Store) KVGet(kind ports.KVKind, record uint32, name string) (ports.KVValue, bool, error) {
	t, err := kvTable(kind)
	if err != nil {
		return ports.KVValue{}, false, err
	}
	return scanKV(kind, s.db.QueryRow(`SELECT value FROM `+t+` WHERE record_id = ? AND name = ?`, record, name))
}

// KVEdit reads, edits and writes one value in one transaction. The store has
// one connection, so no other transaction runs between the read and the write.
func (s *Store) KVEdit(kind ports.KVKind, record uint32, name string, edit func(ports.KVValue, bool, int) (ports.KVEdit, error)) error {
	t, err := kvTable(kind)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var held int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM records WHERE id = ?`, record).Scan(&held); err != nil {
		return err
	}
	if held == 0 {
		return ports.ErrKVNoRecord
	}
	old, found, err := scanKV(kind, tx.QueryRow(`SELECT value FROM `+t+` WHERE record_id = ? AND name = ?`, record, name))
	if err != nil {
		return err
	}
	var names int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+t+` WHERE record_id = ?`, record).Scan(&names); err != nil {
		return err
	}
	e, err := edit(old, found, names)
	if err != nil {
		return err
	}
	switch {
	case e.Delete:
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE record_id = ? AND name = ?`, record, name); err != nil {
			return err
		}
	case e.Write:
		if _, err := tx.Exec(`INSERT INTO `+t+` (record_id, name, value) VALUES (?, ?, ?) ON CONFLICT(record_id, name) DO UPDATE SET value = excluded.value`, record, name, kvArg(kind, e.Value)); err != nil {
			return err
		}
	default:
		return nil
	}
	return tx.Commit()
}

// KVOrphans lists record IDs holding values that name no stored record.
func (s *Store) KVOrphans() ([]uint32, error) {
	rows, err := s.db.Query(`SELECT DISTINCT record_id FROM (SELECT record_id FROM kv UNION SELECT record_id FROM kv_int UNION SELECT record_id FROM kv_json) WHERE record_id NOT IN (SELECT id FROM records) ORDER BY record_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uint32
	for rows.Next() {
		var id uint32
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// KVList is every name record holds in kind, with its size and a preview.
func (s *Store) KVList(kind ports.KVKind, record uint32) ([]ports.KVEntry, error) {
	t, err := kvTable(kind)
	if err != nil {
		return nil, err
	}
	q := `SELECT name, length(CAST(value AS BLOB)), substr(CAST(value AS BLOB), 1, ?) FROM ` + t + ` WHERE record_id = ? ORDER BY name`
	if kind == ports.KVInt {
		q = `SELECT name, 0, value FROM ` + t + ` WHERE record_id = ? ORDER BY name`
	}
	args := []any{ports.KVPreview, record}
	if kind == ports.KVInt {
		args = []any{record}
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.KVEntry
	for rows.Next() {
		var e ports.KVEntry
		if kind == ports.KVInt {
			err = rows.Scan(&e.Name, &e.Size, &e.Preview.Int)
		} else {
			err = rows.Scan(&e.Name, &e.Size, &e.Preview.Bytes)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// KVCounts is how many names each record holding any has, per kind.
func (s *Store) KVCounts() (map[uint32]ports.KVCount, error) {
	out := map[uint32]ports.KVCount{}
	for _, kind := range []ports.KVKind{ports.KVString, ports.KVInt, ports.KVJSON} {
		t, _ := kvTable(kind)
		rows, err := s.db.Query(`SELECT record_id, COUNT(*) FROM ` + t + ` GROUP BY record_id`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id uint32
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				rows.Close()
				return nil, err
			}
			c := out[id]
			switch kind {
			case ports.KVString:
				c.String = n
			case ports.KVInt:
				c.Int = n
			default:
				c.JSON = n
			}
			out[id] = c
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}
