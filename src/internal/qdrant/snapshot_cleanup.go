package qdrant

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const cleanupFile = "snapshot-cleanup.json"

type childSnapshot struct {
	Collection string `json:"collection"`
	Name       string `json:"name"`
}
type snapshotCleanup struct {
	Full     string          `json:"full"`
	Children []childSnapshot `json:"children"`
}

func safePart(s string) bool {
	return s != "" && len(s) <= 512 && !strings.ContainsAny(s, "/\\\x00") && !strings.Contains(s, "..")
}

// Read headers and the bounded upstream manifest only; never extract an archive.
func snapshotChildren(path string, maxBytes int64) ([]childSnapshot, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	entries := map[string]bool{}
	var config []byte
	for n := 0; ; n++ {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid full snapshot tar")
		}
		if n >= 1025 || !safePart(hdr.Name) || hdr.Size < 0 || hdr.Size > maxBytes || hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA || entries[hdr.Name] {
			return nil, errors.New("unsafe or oversized full snapshot tar entry")
		}
		entries[hdr.Name] = true
		if hdr.Name == "config.json" {
			if hdr.Size > 1<<20 {
				return nil, errors.New("snapshot manifest exceeds 1 MiB")
			}
			config, err = io.ReadAll(io.LimitReader(tr, (1<<20)+1))
			if err != nil {
				return nil, err
			}
		}
	}
	var manifest struct {
		Collections map[string]string `json:"collections_mapping"`
	}
	if len(config) == 0 || json.Unmarshal(config, &manifest) != nil || manifest.Collections == nil || len(manifest.Collections) > 1024 {
		return nil, errors.New("invalid full snapshot collection manifest")
	}
	delete(entries, "config.json")
	var children []childSnapshot
	for c, n := range manifest.Collections {
		if !safePart(c) || !safePart(n) || !strings.HasSuffix(n, ".snapshot") || !entries[n] {
			return nil, errors.New("snapshot entries disagree with collection manifest")
		}
		delete(entries, n)
		children = append(children, childSnapshot{c, n})
	}
	if len(entries) != 0 {
		return nil, errors.New("unlisted snapshot tar entry; refusing child deletion")
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Collection < children[j].Collection })
	return children, nil
}
func (h *Host) finishSnapshotCleanup(ctx context.Context, state snapshotCleanup) error {
	if !snapshotName.MatchString(state.Full) || strings.Contains(state.Full, "..") || len(state.Children) > 1024 {
		return errors.New("invalid snapshot cleanup journal")
	}
	for _, child := range state.Children {
		if !safePart(child.Collection) || !safePart(child.Name) {
			return errors.New("invalid snapshot cleanup journal child")
		}
		base := "/collections/" + url.PathEscape(child.Collection) + "/snapshots"
		var snapshots []snapshotInfo
		if err := h.json(ctx, h.Base, "GET", base, nil, &snapshots); err != nil {
			return err
		}
		matches := 0
		for _, s := range snapshots {
			if s.Name == child.Name {
				matches++
			}
		}
		if matches > 1 {
			return errors.New("ambiguous collection snapshot list")
		}
		if matches == 1 {
			if err := h.json(ctx, h.Base, "DELETE", base+"/"+url.PathEscape(child.Name), nil, nil); err != nil {
				return err
			}
		}
	}
	// Full snapshot last; retry can still derive children after an interrupted cleanup.
	resp, err := h.request(ctx, h.Base, "DELETE", "/snapshots/"+url.PathEscape(state.Full), nil, true)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return errors.New("full snapshot cleanup refused")
	}
	if err := os.Remove(h.path(backupDir + "/" + cleanupFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDir(h.path(backupDir))
}
func (h *Host) resumeSnapshotCleanup(ctx context.Context) error {
	data, err := privateFile(h.path(backupDir+"/"+cleanupFile), rootUID(h))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var state snapshotCleanup
	if json.Unmarshal(data, &state) != nil {
		return errors.New("invalid snapshot cleanup journal")
	}
	if err := h.finishSnapshotCleanup(ctx, state); err != nil {
		return errors.New("previous snapshot cleanup remains pending; no new snapshot requested")
	}
	return nil
}
func (h *Host) recordSnapshotCleanup(full, path string) (snapshotCleanup, error) {
	children, err := snapshotChildren(path, h.MaxBytes)
	if err != nil {
		return snapshotCleanup{}, err
	}
	state := snapshotCleanup{full, children}
	data, err := json.Marshal(state)
	if err != nil {
		return state, err
	}
	if len(data) > 64<<10 {
		return state, errors.New("snapshot cleanup journal exceeds 64 KiB")
	}
	return state, writeNew(filepath.Join(h.path(backupDir), cleanupFile), data, rootUID(h), rootGID(h), 0600)
}
