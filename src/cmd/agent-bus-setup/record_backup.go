package main

import (
	"github.com/parf/ai-agent-bus/internal/backup"
	"os"
	"path/filepath"
)

// Fresh installs use dirs; packaged upgrades add the same daemon-owned sibling
// without touching the root-owned stopped-daemon backup directory.
func recordBackupDirectory() error {
	if e := os.MkdirAll(backup.DefaultDirectory, 0700); e != nil {
		return e
	}
	if e := run("chown", svcAccount+":"+svcAccount, backup.DefaultDirectory); e != nil {
		return e
	}
	if e := os.Chmod(backup.DefaultDirectory, 0700); e != nil {
		return e
	}
	return installRecordBackupAccess(filepath.Join(filepath.Dir(unitPath), "agent-busd.service.d", "record-backups.conf"))
}

const recordBackupAccess = "[Service]\nReadWritePaths=" + backup.DefaultDirectory + "\n"

func installRecordBackupAccess(path string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, []byte(recordBackupAccess), 0644)
}
