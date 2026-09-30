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
