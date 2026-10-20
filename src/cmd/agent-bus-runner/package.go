package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// packageDigest describes the installed tree, not its timestamps. WalkDir is
// ordered and never follows links outside the package; a link's target is part
// of the digest. Permission changes matter just as code changes do.
func packageDigest(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%q %v\n", filepath.ToSlash(rel), info.Mode())
		switch {
		case info.IsDir():
			// Directory sizes depend on the filesystem, not the package contents.
		case info.Mode().IsRegular():
			fmt.Fprintf(h, "%d\n", info.Size())
			f, err := agentmeta.OpenFile(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(h, f)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%q", target)
		default:
			return fmt.Errorf("%s: not a package file", path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
