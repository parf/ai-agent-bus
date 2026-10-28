package qdrant

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
)

const unverifiedFile = "snapshot-unverified.json"

// These names are hints, never deletion authority. A private marker blocks a
// second creation even if the first POST's answer/download never arrives.
func (h *Host) snapshotHints(ctx context.Context) (map[string][]string, error) {
	names, err := h.collections(ctx)
	if err != nil {
		return nil, err
	}
	if len(names) > 1024 {
		return nil, errors.New("snapshot hint collection count exceeds 1024")
	}
	hints := map[string][]string{}
	count := 0
	for _, c := range names {
		var snapshots []snapshotInfo
		if err := h.json(ctx, h.Base, "GET", "/collections/"+url.PathEscape(c)+"/snapshots", nil, &snapshots); err != nil {
			return nil, err
		}
		hints[c] = []string{}
		for _, s := range snapshots {
			count++
			if count > 1024 {
				return nil, errors.New("snapshot hint count exceeds 1024")
			}
			hints[c] = append(hints[c], s.Name)
		}
	}
	return hints, nil
}
func (h *Host) beginSnapshot(ctx context.Context) (map[string][]string, error) {
	hints, err := h.snapshotHints(ctx)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(hints)
	if err != nil {
		return nil, err
	}
	if len(data) > 64<<10 {
		return nil, errors.New("snapshot hint journal exceeds 64 KiB")
	}
	if err := writeNew(h.path(backupDir+"/"+unverifiedFile), data, rootUID(h), rootGID(h), 0600); err != nil {
		return nil, err
	}
	return hints, nil
}
func (h *Host) unverifiedHint(ctx context.Context, before map[string][]string) {
	h.say("unverified snapshot: further snapshots blocked by private journal; owner inspection required")
	after, err := h.snapshotHints(ctx)
	if err != nil {
		h.say("snapshot hint delta unavailable (bounded request failed)")
		return
	}
	delta := map[string][]string{}
	for c, names := range after {
		old := map[string]bool{}
		for _, n := range before[c] {
			old[n] = true
		}
		for _, n := range names {
			if !old[n] {
				delta[c] = append(delta[c], n)
			}
		}
	}
	data, _ := json.Marshal(delta)
	if len(data) > 64<<10 {
		h.say("snapshot hint delta exceeds 64 KiB; inspect journal and snapshot lists")
		return
	}
	text := string(data)
	if h.key != "" {
		text = strings.ReplaceAll(text, h.key, "***")
	}
	h.say("snapshot list delta (hint only, never a deletion list): " + text)
}
func (h *Host) refuseUnverified() error {
	_, err := os.Lstat(h.path(backupDir + "/" + unverifiedFile))
	if err == nil {
		return errors.New("unverified snapshot journal exists: owner inspection/explicit clearance required; no new snapshot requested")
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}
