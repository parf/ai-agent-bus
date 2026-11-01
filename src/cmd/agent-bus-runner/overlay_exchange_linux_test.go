//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOverlayPublicationExchangesDirectoriesWithoutRemovingTheBindSource(t *testing.T) {
	host := t.TempDir()
	if err := publishOverlays(host, "hello", []overlayFile{{name: "settings.json", body: []byte("old")}}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(host, ".merged", "hello", ".files")
	exchanges := 0
	oldTree := ""
	err := publishOverlayTree(target, []overlayFile{{name: "settings.json", body: []byte("new")}}, func(stage, target string) error {
		exchanges++
		oldTree = stage
		if err := exchangeOverlayDirs(stage, target); err != nil {
			return err
		}
		for _, tc := range []struct{ path, want string }{{stage, "old"}, {target, "new"}} {
			data, err := os.ReadFile(filepath.Join(tc.path, "settings.json"))
			if err != nil || string(data) != tc.want {
				t.Errorf("exchange did not keep both complete trees: %s %v want%s", data, err, tc.want)
			}
		}
		return nil
	}, func(string, string) error {
		t.Error("exchange path used a rename that removes the live bind source")
		return nil
	})
	if err != nil || exchanges != 1 {
		t.Fatalf("atomic exchange not used: calls=%d err=%v", exchanges, err)
	}
	data, err := os.ReadFile(filepath.Join(oldTree, "settings.json"))
	if err != nil || string(data) != "old" {
		t.Fatalf("exchange deleted old tree before retirement: %s %v", data, err)
	}
	if errs := recoverOverlays(host); len(errs) != 0 {
		t.Fatal(errs)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".files" {
		t.Fatalf("next recovery retained old exchanged output: %v", entries)
	}
}
