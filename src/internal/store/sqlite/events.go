package sqlite

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/parf/ai-agent-bus/internal/ports"
)

func eventIDKey(id uint32) string { return "event-high:" + strconv.FormatUint(uint64(id), 10) }
func eventHigh(raw string, has bool) (uint64, error) {
	if !has {
		return 0, nil
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n > ports.EventMaxID || strconv.FormatUint(n, 10) != raw {
		return 0, ports.ErrEventStateInvalid
	}
	return n, nil
}
func (s *Store) EventHighWater(id uint32) (uint64, error) {
	s.eventMu.Lock()
	if s.eventIssues == nil {
		issues, err := eventIDIssues(s.db)
		if err != nil {
			s.eventMu.Unlock()
			return 0, err
		}
		s.eventIssues = issues
	}
	invalid := s.eventIssues[id]
	s.eventMu.Unlock()
	if invalid {
		return 0, ports.ErrEventStateInvalid
	}
	raw, has, err := meta(s.db, eventIDKey(id))
	if err != nil {
		return 0, err
	}
	return eventHigh(raw, has)
}
func (s *Store) ReserveEventIDs(id uint32, through uint64) error {
	if id == 0 || through == 0 || through > ports.EventMaxID {
		return ports.ErrEventStateInvalid
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	raw, has, err := meta(tx, eventIDKey(id))
	if err != nil {
		return err
	}
	old, err := eventHigh(raw, has)
	if err != nil {
		return err
	}
	if through < old {
		return fmt.Errorf("event ID reservation cannot decrease")
	}
	if err = setMeta(tx, eventIDKey(id), strconv.FormatUint(through, 10)); err != nil {
		return err
	}
	return tx.Commit()
}

func eventIDIssues(tx interface {
	Query(string, ...any) (*sql.Rows, error)
}) (map[uint32]bool, error) {
	rows, err := tx.Query(`SELECT key,value FROM meta WHERE key LIKE 'event-high:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := map[uint32]bool{}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		raw := strings.TrimPrefix(key, "event-high:")
		id, er := strconv.ParseUint(raw, 10, 32)
		if er != nil || id == 0 || strconv.FormatUint(id, 10) != raw {
			issues[uint32(id)] = true
			continue
		}
		if _, er = eventHigh(value, true); er != nil {
			issues[uint32(id)] = true
		}
	}
	return issues, rows.Err()
}

func (s *Store) DropEventState(id uint32) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT key FROM meta WHERE key LIKE 'event-high:%'`)
	if err != nil {
		return err
	}
	keys := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		n, er := strconv.ParseUint(strings.TrimPrefix(key, "event-high:"), 10, 32)
		if er == nil && n == uint64(id) {
			keys = append(keys, key)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if _, err = tx.Exec(`DELETE FROM meta WHERE key=?`, key); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.eventMu.Lock()
	delete(s.eventIssues, id)
	s.eventMu.Unlock()
	return nil
}
