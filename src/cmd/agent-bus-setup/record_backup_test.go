package main

import (
	"github.com/parf/ai-agent-bus/internal/backup"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordBackupDirectoryIsPrivateAndCreatedOnUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-busd.service.d/record-backups.conf")
	if e := installRecordBackupAccess(path); e != nil {
		t.Fatal(e)
	}
	unit, e := os.ReadFile(path)
	if e != nil || string(unit) != "[Service]\nReadWritePaths="+backup.DefaultDirectory+"\n" {
		t.Fatal("daemon mount namespace cannot write backups")
	}
	found := false
	for _, d := range dirs {
		if d.path == backup.DefaultDirectory {
			found = true
			if d.owner != svcAccount || d.mode != 0700 {
				t.Fatal("record backups are not daemon-owned/private")
			}
		}
	}
	if !found {
		t.Fatal("setup omits record backup directory")
	}
	// Upgrade does not take the fresh-install directory path. Pin its explicit
	// directory creation too without running a host upgrade from a unit test.
	file, e := parser.ParseFile(token.NewFileSet(), "upgrade.go", nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	called := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "upgradeBundle" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if ok {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "recordBackupDirectory" {
					called = true
				}
			}
			return true
		})
	}
	if !called {
		t.Fatal("upgrade omits record backup directory")
	}
	if strings.Contains(backup.DefaultDirectory, "daemon.backup") {
		t.Fatal("export retention overlaps stopped-daemon backups")
	}
}
