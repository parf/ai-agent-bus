package qdrant

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
)

const fixture = "agent-bus-qdrant-restore-check"
const collection = "ab_r3_acceptance"
const journalFile = "r3-commissioning.json"

func (h *Host) collections(ctx context.Context) ([]string, error) {
	var result struct {
		Collections []struct {
			Name string `json:"name"`
		} `json:"collections"`
	}
	if err := h.json(ctx, h.Base, "GET", "/collections", nil, &result); err != nil {
		return nil, err
	}
	var out []string
	for _, c := range result.Collections {
		out = append(out, c.Name)
	}
	return out, nil
}
func fixturePoints(run string) []any {
	return []any{
		map[string]any{"id": 1, "vector": []float64{1, 0}, "payload": map[string]string{"fixture": "agent-bus-r3", "run": run}},
		map[string]any{"id": 2, "vector": []float64{0, 1}, "payload": map[string]string{"fixture": "agent-bus-r3-marker", "run": run}},
	}
}
func (h *Host) verifyPoints(ctx context.Context, base, run string) error {
	var points []struct {
		ID      int               `json:"id"`
		Vector  []float64         `json:"vector"`
		Payload map[string]string `json:"payload"`
	}
	if err := h.json(ctx, base, "POST", "/collections/"+collection+"/points", map[string]any{"ids": []int{1, 2}, "with_payload": true, "with_vector": true}, &points); err != nil {
		return err
	}
	if len(points) != 2 {
		return errors.New("restore verification did not return exactly both fixture points")
	}
	seen := map[int]bool{}
	for _, p := range points {
		if seen[p.ID] || p.ID != 1 && p.ID != 2 || len(p.Vector) != 2 || p.Payload["run"] != run {
			return errors.New("restored fixture identity, vector or payload differs")
		}
		seen[p.ID] = true
		if p.ID == 1 && (p.Vector[0] != 1 || p.Vector[1] != 0 || p.Payload["fixture"] != "agent-bus-r3") {
			return errors.New("restored fixture data point differs")
		}
		if p.ID == 2 && (p.Vector[0] != 0 || p.Vector[1] != 1 || p.Payload["fixture"] != "agent-bus-r3-marker") {
			return errors.New("restored fixture marker differs")
		}
	}
	var count struct {
		Count int `json:"count"`
	}
	if err := h.json(ctx, base, "POST", "/collections/"+collection+"/points/count", map[string]bool{"exact": true}, &count); err != nil {
		return err
	}
	if count.Count != 2 {
		return errors.New("fixture contains unexpected points; refusing to treat it as tool-owned")
	}
	return nil
}
func (h *Host) emptyForCheck(ctx context.Context) error {
	names, err := h.collections(ctx)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	if len(names) != 1 || names[0] != collection {
		return errors.New("check is commissioning-only: live collections exist; use snapshot instead")
	}
	data, err := privateFile(h.path(backupDir+"/"+journalFile), rootUID(h))
	if err != nil {
		return errors.New("existing acceptance collection has no private tool journal; refusing deletion")
	}
	var state struct {
		Run string `json:"run"`
	}
	if json.Unmarshal(data, &state) != nil || !validRun(state.Run) {
		return errors.New("invalid commissioning journal; refusing existing fixture deletion")
	}
	if err := h.verifyPoints(ctx, h.Base, state.Run); err != nil {
		return err
	}
	if err := h.json(ctx, h.Base, "DELETE", "/collections/"+collection, nil, nil); err != nil {
		return err
	}
	return os.Remove(h.path(backupDir + "/" + journalFile))
}
func validRun(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func (h *Host) fixtureOwned(ctx context.Context, kind string, run, id string) error {
	format := `{{.Id}} {{index .Config.Labels "agent-bus.fixture"}} {{index .Config.Labels "agent-bus.run"}}`
	if kind == "volume" {
		format = `{{.Name}} {{index .Labels "agent-bus.fixture"}} {{index .Labels "agent-bus.run"}}`
	}
	out, err := h.command(ctx, "podman", kind, "inspect", "--format", format, fixture)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(out))
	if len(fields) != 3 || fields[1] != "r3" || fields[2] != run || id != "" && fields[0] != id {
		return errors.New("fixture identity/labels changed; refusing cleanup")
	}
	return nil
}
func (h *Host) check(ctx context.Context) (err error) {
	entries, e := os.ReadDir(h.path(backupDir))
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if ownedBackup.MatchString(entry.Name()) {
			return errors.New("commissioning check refuses existing backups; use snapshot instead")
		}
	}
	for suffix, body := range map[string]string{".container": Container(), ".volume": Volume()} {
		data, e := h.quadlet(h.path(unitDir + "/" + name + suffix))
		if e != nil {
			return e
		}
		if string(data) != body {
			return errors.New("installed Quadlet differs from managed configuration")
		}
	}

	// Never adopt/remove somebody else's fixture; reserve the port before writes.
	for _, kind := range []string{"container", "volume"} {
		has, e := h.exists(ctx, kind, fixture)
		if e != nil {
			return e
		}
		if has {
			return fmt.Errorf("pre-existing %s %s; refusing check", kind, fixture)
		}
	}
	if e := h.PortFree(16333); e != nil {
		return e
	}
	if e := h.emptyForCheck(ctx); e != nil {
		return e
	}
	run, e := randomHex(16)
	if e != nil {
		return e
	}
	journal := h.path(backupDir + "/" + journalFile)
	if _, e := os.Lstat(journal); e == nil {
		return errors.New("stale commissioning journal without its collection; owner inspection required")
	} else if !os.IsNotExist(e) {
		return e
	}
	state, _ := json.Marshal(map[string]string{"run": run})
	if e := writeNew(journal, state, rootUID(h), rootGID(h), 0600); e != nil {
		return e
	}
	createdCollection, createdVolume, createdContainer := false, false, false
	var snapshotFile, tarFile, containerID string
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var failures []error
		if has, e := h.exists(cleanup, "container", fixture); createdContainer && (has || e != nil) {
			if e := h.fixtureOwned(cleanup, "container", run, containerID); e != nil {
				failures = append(failures, e)
			} else if _, e := h.command(cleanup, "podman", "rm", "--force", "--time=2", fixture); e != nil {
				failures = append(failures, e)
			}
		}
		if has, e := h.exists(cleanup, "volume", fixture); createdVolume && (has || e != nil) {
			if e := h.fixtureOwned(cleanup, "volume", run, fixture); e != nil {
				failures = append(failures, e)
			} else if _, e := h.command(cleanup, "podman", "volume", "rm", fixture); e != nil {
				failures = append(failures, e)
			}
		}
		if createdCollection {
			if e := h.json(cleanup, h.Base, "DELETE", "/collections/"+collection, nil, nil); e != nil {
				failures = append(failures, e)
			}
		}
		for _, file := range []string{snapshotFile, tarFile} {
			if file != "" {
				if e := os.Remove(file); e != nil && !os.IsNotExist(e) {
					failures = append(failures, e)
				}
			}
		}
		if len(failures) == 0 {
			if e := os.Remove(journal); e != nil {
				failures = append(failures, e)
			}
		}
		if len(failures) > 0 {
			err = errors.Join(err, fmt.Errorf("commissioning cleanup failed: %w", errors.Join(failures...)))
		}
	}()
	if e := h.json(ctx, h.Base, "PUT", "/collections/"+collection, map[string]any{"vectors": map[string]any{"size": 2, "distance": "Cosine"}}, nil); e != nil {
		return e
	}
	createdCollection = true
	if e := h.json(ctx, h.Base, "PUT", "/collections/"+collection+"/points?wait=true", map[string]any{"points": fixturePoints(run)}, nil); e != nil {
		return e
	}
	if e := h.verifyPoints(ctx, h.Base, run); e != nil {
		return e
	}
	if _, e := h.command(ctx, "systemctl", "restart", name+".service"); e != nil {
		return e
	}
	if e := h.ready(ctx, h.Base); e != nil {
		return e
	}
	if e := h.verifyPoints(ctx, h.Base, run); e != nil {
		return e
	}
	if e := h.exposure(ctx, name, 6333); e != nil {
		return e
	}
	names, e := h.collections(ctx)
	if e != nil {
		return e
	}
	if len(names) != 1 || names[0] != collection {
		return errors.New("non-fixture collection appeared; refusing commissioning snapshot")
	}
	snapshotFile, e = h.snapshot(ctx)
	if e != nil {
		return e
	}
	tarFile = h.path(backupDir + "/r3-acceptance.tar")
	if e := snapshotTar(snapshotFile, tarFile); e != nil {
		return e
	}
	createdVolume = true // A failed create may still have created this run's labelled volume.
	if _, e := h.command(ctx, "podman", "volume", "create", "--uid=1000", "--gid=1000", "--label", "agent-bus.fixture=r3", "--label", "agent-bus.run="+run, fixture); e != nil {
		return e
	}
	if _, e := h.command(ctx, "podman", "volume", "import", fixture, tarFile); e != nil {
		return e
	}
	args := []string{"podman", "run", "--detach", "--pull=never", "--name", fixture, "--label", "agent-bus.fixture=r3", "--label", "agent-bus.run=" + run, "--user=1000:1000", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--tmpfs=/tmp:rw,nosuid,nodev,noexec,size=64m", "--pids-limit=512", "--memory=2g", "--cpus=2", "--network=host", "--volume", fixture + ":/qdrant/storage", "--secret", secretName + ",type=mount,uid=1000,gid=1000,mode=0400", "--env", "QDRANT__SERVICE__HTTP_PORT=16333", "--env", "QDRANT_INIT_FILE_PATH=/qdrant/storage/.qdrant-initialized", "--entrypoint", "/qdrant/qdrant", Image, "--config-path", "/run/secrets/" + secretName, "--storage-snapshot", "/qdrant/storage/restore/r3-acceptance.snapshot"}
	createdContainer = true // A failed run may leave a container; only this run's labels authorize cleanup.
	out, e := h.command(ctx, args...)
	if e != nil {
		return e
	}
	containerID = strings.TrimSpace(string(out))
	if len(containerID) != 64 {
		return errors.New("Podman did not return a container identity")
	}
	base := "http://127.0.0.1:16333"
	if e := h.ready(ctx, base); e != nil {
		return e
	}
	if e := h.verifyPoints(ctx, base, run); e != nil {
		return e
	}
	if e := h.exposure(ctx, fixture, 16333); e != nil {
		return e
	}
	owner, e := h.command(ctx, "podman", "exec", fixture, "stat", "-c", "%u:%g", "/qdrant/storage/restore/r3-acceptance.snapshot")
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(owner)) != "1000:1000" {
		return errors.New("imported snapshot ownership was not 1000:1000")
	}
	h.say("PASS full snapshot recovered with read-only rootfs and writable storage init marker")
	h.say("PASS imported snapshot ownership 1000:1000")
	if e := h.validateGenerator(ctx); e != nil {
		return e
	}
	h.say("PASS installed generator accepted every rendered Quadlet key")
	return nil
}
func snapshotTar(snapshot, path string) (err error) {
	input, err := os.OpenFile(snapshot, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer input.Close()
	st, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		output.Close()
		if !ok {
			os.Remove(path)
		}
	}()
	tw := tar.NewWriter(output)
	if err := tw.WriteHeader(&tar.Header{Name: "restore/r3-acceptance.snapshot", Mode: 0400, Uid: 1000, Gid: 1000, Size: st.Size(), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := io.CopyN(tw, input, st.Size()); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
