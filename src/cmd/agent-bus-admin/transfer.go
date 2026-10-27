package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/backup"
	"github.com/parf/ai-agent-bus/internal/core"
)

type transferArgs struct {
	selection               core.RecordSelection
	choices                 core.ImportChoices
	out, info, key, archive string
}

func parseTransfer(verb string, args []string) (transferArgs, error) {
	var o transferArgs
	o.choices.RenameRealm = map[string]string{}
	for _, arg := range args {
		key, val, eq := strings.Cut(arg, "=")
		if verb == "import" && key == "--dry-run" && !eq {
			o.choices.DryRun = true
			continue
		}
		if !strings.HasPrefix(arg, "-") && verb == "import" && o.archive == "" {
			o.archive = arg
			continue
		}
		if !eq || val == "" {
			return o, fmt.Errorf("expected --name=value: %s", key)
		}
		if verb == "export" {
			switch key {
			case "--include":
				o.selection.Include = append(o.selection.Include, val)
			case "--exclude":
				o.selection.Exclude = append(o.selection.Exclude, val)
			case "--out":
				if o.out != "" {
					return o, errors.New("--out repeated")
				}
				o.out = val
			case "--info":
				if o.info != "" {
					return o, errors.New("--info repeated")
				}
				o.info = val
			default:
				return o, fmt.Errorf("unknown export option: %s", key)
			}
		} else {
			switch key {
			case "--key":
				o.key = val
			case "--keep-existing":
				o.choices.KeepExisting = append(o.choices.KeepExisting, val)
			case "--replace":
				o.choices.Replace = append(o.choices.Replace, val)
			case "--replace-users":
				o.choices.ReplaceUsers = append(o.choices.ReplaceUsers, val)
			case "--replace-tokens":
				o.choices.ReplaceTokens = append(o.choices.ReplaceTokens, val)
			case "--rename-realm":
				from, to, ok := strings.Cut(val, "=")
				if !ok {
					return o, errors.New("--rename-realm needs <from>=<to>")
				}
				if _, ok = o.choices.RenameRealm[from]; ok {
					return o, errors.New("source realm repeated")
				}
				o.choices.RenameRealm[from] = to
			default:
				return o, fmt.Errorf("unknown import option: %s", key)
			}
		}
	}
	if verb == "export" && o.info != "" && o.out != "-" {
		return o, errors.New("--info requires --out=-")
	}
	if verb == "import" {
		if o.archive == "" {
			return o, errors.New("import needs <archive>")
		}
		if o.key == "" {
			home, e := os.UserHomeDir()
			if e != nil {
				return o, e
			}
			o.key = filepath.Join(home, ".ssh/id_ed25519")
		}
	}
	return o, nil
}

// This path runs before beTheAccount. The caller's login socket, never the
// daemon account's socket after sudo, establishes the export/import authority.
func transferCall(path string, body any) ([]byte, error) {
	addr := os.Getenv("AGENT_BUS_ADDR")
	if addr == "" {
		addr = api.ClientSocket()
	}
	if !filepath.IsAbs(addr) {
		return nil, errors.New("record transfer requires your local login socket")
	}
	client, base := api.Dial(addr)
	client.Timeout = 3 * time.Minute
	raw, e := json.Marshal(body)
	if e != nil {
		return nil, errors.New("cannot encode transfer request")
	}
	req, e := http.NewRequest("POST", base+path, bytes.NewReader(raw))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(req)
	if e != nil {
		return nil, errors.New("cannot reach your local login socket")
	}
	defer resp.Body.Close()
	out, e := io.ReadAll(io.LimitReader(resp.Body, backup.MaxBytes*2+1))
	if e != nil || int64(len(out)) > backup.MaxBytes*2 {
		return nil, errors.New("invalid transfer response")
	}
	if resp.StatusCode != http.StatusOK {
		var reply struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(out, &reply) != nil || reply.Error == "" {
			return nil, fmt.Errorf("record transfer refused (%d)", resp.StatusCode)
		}
		return nil, errors.New(reply.Error)
	}
	return out, nil
}
func readTransferFile(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, backup.MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > backup.MaxBytes {
		return nil, errors.New("backup exceeds 256 MiB")
	}
	return b, nil
}
func recordTransfer(verb string, args []string) error {
	o, e := parseTransfer(verb, args)
	if e != nil {
		return e
	}
	if verb == "export" {
		raw, e := transferCall("/records/export", api.ExportRequest{RecordSelection: o.selection, Download: o.out != ""})
		if e != nil {
			return e
		}
		var reply api.ExportReply
		if json.Unmarshal(raw, &reply) != nil {
			return errors.New("invalid export response")
		}
		if o.out == "" {
			fmt.Printf("%s\n%s\n", reply.ArchivePath, reply.InfoPath)
			return nil
		}
		if backup.Hash(reply.Ciphertext) != reply.Sidecar.SHA256 {
			return errors.New("invalid exported ciphertext hash")
		}
		sidecar, e := json.MarshalIndent(reply.Sidecar, "", "  ")
		if e != nil {
			return e
		}
		sidecar = append(sidecar, '\n')
		if o.out == "-" {
			if o.info != "" {
				f, e := os.OpenFile(o.info, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if e != nil {
					return e
				}
				_, e = f.Write(sidecar)
				ce := f.Close()
				if e != nil {
					return e
				}
				if ce != nil {
					return ce
				}
			} else {
				fmt.Fprintln(os.Stderr, "no sidecar: use --info=<path> with --out=-")
			}
			_, e = os.Stdout.Write(reply.Ciphertext)
			return e
		}
		if e = backup.WriteFiles(o.out, reply.Ciphertext, sidecar); e != nil {
			return e
		}
		fmt.Printf("%s\n%s.info\n", o.out, o.out)
		return nil
	}
	ciphertext, e := readTransferFile(o.archive)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plain, e := backup.Decrypt(ctx, ciphertext, o.key)
	if e != nil {
		return e
	}
	manifest, files, e := backup.Unpack(plain)
	if e != nil {
		return e
	}
	sidecar, e := readTransferFile(o.archive + ".info")
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if e = backup.CheckSidecar(ciphertext, manifest, sidecar); e != nil {
		return e
	}
	if len(sidecar) == 0 {
		fmt.Fprintln(os.Stderr, "no sidecar: using encrypted manifest")
	}
	if len(files) != 1 {
		return errors.New("not a record backup")
	}
	var data core.RecordBackup
	dec := json.NewDecoder(bytes.NewReader(files["records.json"]))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&data); e != nil {
		return errors.New("invalid record backup payload")
	}
	if dec.Decode(new(any)) != io.EOF {
		return errors.New("invalid record backup payload")
	}
	raw, e := transferCall("/records/import", api.ImportRequest{Data: data, Choices: o.choices})
	if e != nil {
		return e
	}
	var report core.ImportReport
	if json.Unmarshal(raw, &report) != nil {
		return errors.New("invalid import response")
	}
	for from, to := range report.Renamed {
		fmt.Printf("renamed %s -> %s\n", from, to)
	}
	for _, name := range report.Kept {
		fmt.Printf("kept %s\n", name)
	}
	for _, name := range report.Imported {
		fmt.Printf("imported %s\n", name)
	}
	for _, name := range report.TokensKept {
		fmt.Printf("%s: existing token retained; archived token not retained\n", name)
	}
	if report.DryRun {
		fmt.Println("dry-run: nothing changed")
	}
	return nil
}
