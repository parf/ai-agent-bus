package core

// MigrateLegacyEmptyTags persists only records whose old stored tags contained
// the reserved display sentinel. Restore already normalized their in-memory tags.
// Startup calls this after binding persistence and validating the daemon Owner.
func (b *Bus) MigrateLegacyEmptyTags() (int, error) {
	b.mu.Lock()
	defer b.unlock()
	count := 0
	for _, name := range b.legacyEmptyTags {
		if r, ok := b.records[name]; ok {
			b.setRecord(name, r)
			count++
		}
	}
	if err := b.commit(); err != nil {
		return 0, err
	}
	b.legacyEmptyTags = nil
	return count, nil
}
