package sqlite

import "github.com/parf/ai-agent-bus/internal/ports"

// ExportState holds one read transaction for every selected KV/credential row.
func (s *Store) ExportState(ids []uint32, names []string) (ports.TransferState, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ports.TransferState{}, err
	}
	defer tx.Rollback()
	var out ports.TransferState
	for _, id := range ids {
		for kind, table := range kvTables {
			rows, err := tx.Query("SELECT name,value FROM "+table+" WHERE record_id = ? ORDER BY name", id)
			if err != nil {
				return out, err
			}
			for rows.Next() {
				v := ports.StoredKV{RecordID: id, Kind: kind}
				if kind == ports.KVInt {
					err = rows.Scan(&v.Name, &v.Value.Int)
				} else {
					err = rows.Scan(&v.Name, &v.Value.Bytes)
				}
				if err != nil {
					rows.Close()
					return out, err
				}
				out.KV = append(out.KV, v)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return out, err
			}
		}
	}
	for _, name := range names {
		rows, err := tx.Query("SELECT name,current,previous,issued,used,user_id,agent_id FROM credentials WHERE name = ?", name)
		if err != nil {
			return out, err
		}
		if rows.Next() {
			var c ports.Credential
			var issued, used string
			err = rows.Scan(&c.Name, &c.Current, &c.Previous, &issued, &used, &c.UserID, &c.AgentID)
			if err == nil {
				c.Issued, err = parseTime(issued)
			}
			if err == nil {
				c.Used, err = parseTime(used)
			}
			if err != nil {
				rows.Close()
				return out, err
			}
			out.Credentials = append(out.Credentials, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}
