// Package bigfile holds bigfiles in a private directory, or in memory when a
// put asks for it (Plans/R1.0-Release/bigfile.md#trust-and-durability).
//
// The daemon's database is the index and the source of truth: one row per
// bigfile held in a file (ports.BigIndex). One file holds one key: the bytes,
// then a footer naming the key, its size, its SHA-256 and its expiry, then the
// footer's length; the footer is kept for recovery by hand and is never read
// to decide what exists. A file is written to a temporary name, synced and
// renamed into place, and only then does its row commit; a delete removes the
// row first and the file after. A crash between any two steps leaves a file
// with no row or a row with no file, and start removes either. A
// name or a record never reaches a path: files are named by the record's ID
// and the hash of the name. Files are 0600 in 0700 directories, and links are
// never followed.
package bigfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
)

const (
	footerMax = 4096
	tmpDir    = ".tmp"
)

// crashPoint is a seam for tests: it is called between the steps whose order
// matters, and a test that panics there leaves the state a crash would.
var crashPoint = func(string) {}

type key struct {
	record uint32
	name   string
}

// memEntry is a bigfile held in memory: no file and no row.
type memEntry struct {
	info ports.BigInfo
	data []byte
}

type footer struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Expires string `json:"expires"`
}

// Store is safe for concurrent use.
type Store struct {
	dir string
	idx ports.BigIndex
	mu  sync.Mutex
	mem map[key]memEntry
}

// Open prepares dir and reconciles it with the index: temporary files, rows
// without a whole file, expired rows, and files without a row are removed.
// Memory bigfiles never survive a start.
func Open(dir string, idx ports.BigIndex, now time.Time) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("bigfile directory %s is not a directory", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, idx: idx, mem: map[key]memEntry{}}
	if err := os.RemoveAll(filepath.Join(dir, tmpDir)); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(dir, tmpDir), 0o700); err != nil {
		return nil, err
	}
	if err := s.reconcile(now); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) rowPath(r ports.BigRow) string {
	return filepath.Join(s.dir, strconv.FormatUint(uint64(r.Record), 10), r.File)
}

// reconcile makes the files agree with the rows, the rows being the truth.
func (s *Store) reconcile(now time.Time) error {
	rows, err := s.idx.BigRows()
	if err != nil {
		return err
	}
	kept := map[string]bool{}
	for _, r := range rows {
		path := s.rowPath(r)
		whole := false
		// A record directory that is a link is not trusted for reading or for
		// removing: the row goes, and nothing behind the link is touched.
		if s.parentOK(path) == nil {
			if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() && now.Before(r.Expires) {
				if info, err := readFooter(path); err == nil && info.Name == r.Name && info.Size == r.Size && info.SHA256 == r.SHA256 {
					whole = true
				}
			}
		}
		if whole {
			kept[path] = true
			continue
		}
		// An expired row, a row without its file, or a file that is not the
		// one the row names goes, row first, as a delete does.
		if _, _, err := s.idx.BigRowDelete(r.Record, r.Name); err != nil {
			return err
		}
		s.unlink(r)
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == tmpDir {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		if _, perr := strconv.ParseUint(e.Name(), 10, 32); perr != nil || !e.IsDir() {
			os.RemoveAll(path)
			continue
		}
		files, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, f := range files {
			fp := filepath.Join(path, f.Name())
			if !kept[fp] {
				// A file no row names: a crash between rename and commit, or
				// between a row's delete and its file's unlink. A FIFO or link
				// is removed untouched.
				os.RemoveAll(fp)
			}
		}
	}
	return nil
}

// realDir refuses a path that is not a directory itself: a link in its place
// would carry a write or a delete outside the root. The tree is the daemon
// account's own, so this guards a planted link, not a race with that account.
func realDir(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

// parentOK checks the directories above a key file before it is touched.
func (s *Store) parentOK(path string) error {
	if err := realDir(s.dir); err != nil {
		return err
	}
	return realDir(filepath.Dir(path))
}

func fileName(name string) string {
	h := sha256.Sum256([]byte(name))
	return hex.EncodeToString(h[:])
}

// readFooter checks a file is whole: its footer parses and agrees with its
// length.
func readFooter(path string) (ports.BigInfo, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|nofollow|nonblock, 0)
	if err != nil {
		return ports.BigInfo{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 5 {
		return ports.BigInfo{}, errors.New("not a bigfile file")
	}
	var n [4]byte
	if _, err := f.ReadAt(n[:], st.Size()-4); err != nil {
		return ports.BigInfo{}, err
	}
	fl := int64(binary.BigEndian.Uint32(n[:]))
	if fl < 2 || fl > footerMax || fl+4 > st.Size() {
		return ports.BigInfo{}, errors.New("bad footer")
	}
	raw := make([]byte, fl)
	if _, err := f.ReadAt(raw, st.Size()-4-fl); err != nil {
		return ports.BigInfo{}, err
	}
	var ft footer
	if json.Unmarshal(raw, &ft) != nil || ft.Size < 1 || ft.Size != st.Size()-4-fl || !hexSum(ft.SHA256) || ft.Name == "" {
		return ports.BigInfo{}, errors.New("bad footer")
	}
	exp, err := time.Parse(time.RFC3339Nano, ft.Expires)
	if err != nil {
		return ports.BigInfo{}, err
	}
	return ports.BigInfo{Name: ft.Name, Size: ft.Size, SHA256: ft.SHA256, Held: "file", Expires: exp}, nil
}

func hexSum(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// unlink removes a row's file, if its directory is a real one.
func (s *Store) unlink(r ports.BigRow) {
	path := s.rowPath(r)
	if s.parentOK(path) == nil {
		os.Remove(path)
	}
}

// expire removes what has expired: memory entries, and the rows the index
// drops, whose files follow. Caller holds s.mu.
func (s *Store) expire(now time.Time) error {
	for k, e := range s.mem {
		if !now.Before(e.info.Expires) {
			delete(s.mem, k)
		}
	}
	gone, err := s.idx.BigRowsDropExpired(now)
	for _, r := range gone {
		s.unlink(r)
	}
	return err
}

// Sweep removes expired bigfiles and their files; the daemon calls it on a
// timer, and every read expires lazily too.
func (s *Store) Sweep(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
}

// usage is what the record holds and what the daemon holds, ignoring one name
// that a put is about to replace. Caller holds s.mu.
func (s *Store) usage(record uint32, name string) (inRecord, total int64, names int, err error) {
	rows, err := s.idx.BigRows()
	if err != nil {
		return 0, 0, 0, err
	}
	add := func(r uint32, n string, size int64) {
		if r == record && n == name {
			return
		}
		total += size
		if r == record {
			inRecord += size
			names++
		}
	}
	for _, r := range rows {
		add(r.Record, r.Name, r.Size)
	}
	for k, e := range s.mem {
		add(k.record, k.name, e.info.Size)
	}
	return
}

func (s *Store) BigPut(record uint32, name string, data io.Reader, sum string, memory bool, expires, now time.Time, lim ports.BigLimits) (ports.BigInfo, bool, error) {
	var (
		tmp  string
		body []byte
		size int64
	)
	h := sha256.New()
	file := fileName(name) + "-" + randomName()
	if memory {
		buf, err := io.ReadAll(io.LimitReader(data, lim.Value+1))
		if err != nil {
			return ports.BigInfo{}, false, err
		}
		body, size = buf, int64(len(buf))
		h.Write(buf)
	} else {
		if err := realDir(s.dir); err != nil {
			return ports.BigInfo{}, false, err
		}
		if err := realDir(filepath.Join(s.dir, tmpDir)); err != nil {
			return ports.BigInfo{}, false, err
		}
		f, err := os.OpenFile(filepath.Join(s.dir, tmpDir, randomName()), os.O_WRONLY|os.O_CREATE|os.O_EXCL|nofollow, 0o600)
		if err != nil {
			return ports.BigInfo{}, false, err
		}
		tmp = f.Name()
		n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(data, lim.Value+1))
		size = n
		if err == nil && size >= 1 && size <= lim.Value {
			err = writeFooter(f, name, size, h.Sum(nil), expires)
		}
		if err == nil {
			err = f.Sync()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(tmp)
			return ports.BigInfo{}, false, err
		}
	}
	fail := func(err error) (ports.BigInfo, bool, error) {
		if tmp != "" {
			os.Remove(tmp)
		}
		return ports.BigInfo{}, false, err
	}
	switch {
	case size > lim.Value:
		return fail(ports.ErrBigTooLarge)
	case size == 0:
		return fail(ports.ErrBigEmpty)
	}
	hexs := hex.EncodeToString(h.Sum(nil))
	if sum != "" && sum != hexs {
		return fail(ports.ErrBigHash)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.expire(now); err != nil {
		return fail(err)
	}
	inRecord, total, names, err := s.usage(record, name)
	if err != nil {
		return fail(err)
	}
	switch {
	case names+1 > lim.Names:
		return fail(fmt.Errorf("%w: a record holds at most %d bigfiles", ports.ErrBigFull, lim.Names))
	case inRecord+size > lim.Record:
		return fail(fmt.Errorf("%w: a record holds at most %d bytes of bigfiles", ports.ErrBigFull, lim.Record))
	case total+size > lim.Total:
		return fail(fmt.Errorf("%w: the daemon holds at most %d bytes of bigfiles", ports.ErrBigFull, lim.Total))
	}
	k := key{record, name}
	_, wasMem := s.mem[k]
	if memory {
		// A file under this name, if any, goes first: row, then file.
		old, wasRow, err := s.idx.BigRowDelete(record, name)
		if err != nil {
			return ports.BigInfo{}, false, err
		}
		if wasRow {
			s.unlink(old)
		}
		info := ports.BigInfo{Name: name, Size: size, SHA256: hexs, Held: "memory", Expires: expires}
		s.mem[k] = memEntry{info: info, data: body}
		return info, wasMem || wasRow, nil
	}
	dir := filepath.Join(s.dir, strconv.FormatUint(uint64(record), 10))
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return fail(err)
		}
	}
	if err := realDir(dir); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, file)); err != nil {
		return fail(err)
	}
	syncDir(dir)
	syncDir(s.dir)
	crashPoint("renamed")
	row := ports.BigRow{Record: record, Name: name, File: file, Size: size, SHA256: hexs, Held: "file", Expires: expires}
	old, wasRow, err := s.idx.BigRowPut(row)
	if err != nil {
		os.Remove(filepath.Join(dir, file)) // the file never got its row
		return ports.BigInfo{}, false, err
	}
	crashPoint("committed")
	if wasRow && old.File != file {
		s.unlink(old)
	}
	delete(s.mem, k)
	return ports.BigInfo{Name: name, Size: size, SHA256: hexs, Held: "file", Expires: expires}, wasRow || wasMem, nil
}

func writeFooter(f *os.File, name string, size int64, sum []byte, expires time.Time) error {
	raw, _ := json.Marshal(footer{name, size, hex.EncodeToString(sum), expires.UTC().Format(time.RFC3339Nano)})
	if len(raw) > footerMax {
		return errors.New("footer too large")
	}
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(raw)))
	_, err := f.Write(append(raw, n[:]...))
	return err
}

func randomName() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func syncDir(path string) {
	if d, err := os.Open(path); err == nil {
		d.Sync()
		d.Close()
	}
}

type sectionCloser struct {
	*io.SectionReader
	f *os.File
}

func (c sectionCloser) Close() error { return c.f.Close() }

func rowInfo(r ports.BigRow) ports.BigInfo {
	return ports.BigInfo{Name: r.Name, Size: r.Size, SHA256: r.SHA256, Held: r.Held, Expires: r.Expires}
}

func (s *Store) BigGet(record uint32, name string, now time.Time) (io.ReadCloser, ports.BigInfo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.expire(now); err != nil {
		return nil, ports.BigInfo{}, false, err
	}
	if e, ok := s.mem[key{record, name}]; ok {
		return io.NopCloser(bytes.NewReader(e.data)), e.info, true, nil
	}
	r, found, err := s.idx.BigRowGet(record, name)
	if err != nil || !found {
		return nil, ports.BigInfo{}, false, err
	}
	path := s.rowPath(r)
	if err := s.parentOK(path); err != nil {
		return nil, ports.BigInfo{}, false, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|nofollow|nonblock, 0)
	if err != nil {
		return nil, ports.BigInfo{}, false, err
	}
	return sectionCloser{io.NewSectionReader(f, 0, r.Size), f}, rowInfo(r), true, nil
}

func (s *Store) BigList(record uint32, now time.Time) ([]ports.BigInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.expire(now); err != nil {
		return nil, err
	}
	rows, err := s.idx.BigRowsOf(record)
	if err != nil {
		return nil, err
	}
	out := []ports.BigInfo{}
	for _, r := range rows {
		out = append(out, rowInfo(r))
	}
	for k, e := range s.mem {
		if k.record == record {
			out = append(out, e.info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// BigDelete removes the row first and the file after, so a crash between the
// two leaves a file no row names, which start removes.
func (s *Store) BigDelete(record uint32, name string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.expire(now); err != nil {
		return false, err
	}
	k := key{record, name}
	_, wasMem := s.mem[k]
	delete(s.mem, k)
	old, wasRow, err := s.idx.BigRowDelete(record, name)
	if err != nil {
		return false, err
	}
	crashPoint("row-deleted")
	if wasRow {
		s.unlink(old)
	}
	return wasMem || wasRow, nil
}

func (s *Store) BigDrop(record uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.mem {
		if k.record == record {
			delete(s.mem, k)
		}
	}
	rows, err := s.idx.BigRowsDropRecord(record)
	if err != nil {
		return err
	}
	for _, r := range rows {
		s.unlink(r)
	}
	return os.RemoveAll(filepath.Join(s.dir, strconv.FormatUint(uint64(record), 10)))
}

// BigRecords is the IDs of every record holding a bigfile, for the start-up
// check against the registry.
func (s *Store) BigRecords() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[uint32]bool{}
	var out []uint32
	rows, _ := s.idx.BigRows()
	for _, r := range rows {
		if !seen[r.Record] {
			seen[r.Record] = true
			out = append(out, r.Record)
		}
	}
	for k := range s.mem {
		if !seen[k.record] {
			seen[k.record] = true
			out = append(out, k.record)
		}
	}
	return out
}
