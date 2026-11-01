// Package bigfile holds big keys in a private directory, or in memory when a
// put asks for it (Plans/R1.0-Release/big-keys.md#trust-and-durability).
//
// One file holds one key: the bytes, then a footer naming the key, its size,
// its SHA-256 and its expiry, then the footer's length. A file is written to a
// temporary name, synced and renamed into place, so a key exists whole or not
// at all, and a crash leaves at most a temporary file that start removes. A
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

type key struct {
	record uint32
	name   string
}

type entry struct {
	info ports.BigInfo
	data []byte // set for a memory key
	path string // set for a file key
}

type footer struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Expires string `json:"expires"`
}

// Store is safe for concurrent use.
type Store struct {
	dir  string
	mu   sync.Mutex
	keys map[key]entry
}

// Open prepares dir, removes what a crash left, and indexes the files that
// are whole and not expired. Anything else in it is removed.
func Open(dir string, now time.Time) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("big-key directory %s is not a directory", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, keys: map[key]entry{}}
	if err := os.RemoveAll(filepath.Join(dir, tmpDir)); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(dir, tmpDir), 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Name() == tmpDir {
			continue
		}
		id, perr := strconv.ParseUint(e.Name(), 10, 32)
		path := filepath.Join(dir, e.Name())
		if perr != nil || !e.IsDir() {
			os.RemoveAll(path)
			continue
		}
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			fp := filepath.Join(path, f.Name())
			// A FIFO, device or link is removed untouched: opening one could block.
			if !f.Type().IsRegular() {
				os.Remove(fp)
				continue
			}
			info, err := readFooter(fp)
			if err != nil || f.Name() != fileName(info.Name) || !now.Before(info.Expires) {
				os.Remove(fp)
				continue
			}
			s.keys[key{uint32(id), info.Name}] = entry{info: info, path: fp}
		}
	}
	return s, nil
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
		return ports.BigInfo{}, errors.New("not a big-key file")
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

// expire removes every expired key. Caller holds s.mu.
func (s *Store) expire(now time.Time) {
	for k, e := range s.keys {
		if !now.Before(e.info.Expires) {
			s.remove(k, e)
		}
	}
}

func (s *Store) remove(k key, e entry) {
	if e.path != "" && s.parentOK(e.path) == nil {
		os.Remove(e.path)
	}
	delete(s.keys, k)
}

// Sweep removes expired keys and their files; the daemon calls it on a timer,
// and every read expires lazily too.
func (s *Store) Sweep(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
}

func (s *Store) BigPut(record uint32, name string, data io.Reader, sum string, memory bool, expires, now time.Time, lim ports.BigLimits) (ports.BigInfo, bool, error) {
	var (
		tmp  string
		body []byte
		size int64
		hexs string
	)
	h := sha256.New()
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
	hexs = hex.EncodeToString(h.Sum(nil))
	if sum != "" && sum != hexs {
		return fail(ports.ErrBigHash)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	var inRecord, total int64
	names := 0
	for k, e := range s.keys {
		if k.record == record && k.name == name {
			continue
		}
		total += e.info.Size
		if k.record == record {
			inRecord += e.info.Size
			names++
		}
	}
	switch {
	case names+1 > lim.Names:
		return fail(fmt.Errorf("%w: a record holds at most %d big keys", ports.ErrBigFull, lim.Names))
	case inRecord+size > lim.Record:
		return fail(fmt.Errorf("%w: a record holds at most %d bytes of big keys", ports.ErrBigFull, lim.Record))
	case total+size > lim.Total:
		return fail(fmt.Errorf("%w: the daemon holds at most %d bytes of big keys", ports.ErrBigFull, lim.Total))
	}
	old, replaced := s.keys[key{record, name}]
	info := ports.BigInfo{Name: name, Size: size, SHA256: hexs, Held: "memory", Expires: expires}
	e := entry{info: info, data: body}
	if !memory {
		dir := filepath.Join(s.dir, strconv.FormatUint(uint64(record), 10))
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			if err := os.Mkdir(dir, 0o700); err != nil {
				return fail(err)
			}
		}
		if err := realDir(dir); err != nil {
			return fail(err)
		}
		path := filepath.Join(dir, fileName(name))
		if err := os.Rename(tmp, path); err != nil {
			return fail(err)
		}
		syncDir(dir)
		syncDir(s.dir)
		info.Held = "file"
		e = entry{info: info, path: path}
	}
	if replaced && old.path != "" && old.path != e.path {
		os.Remove(old.path) // the same key moved from a file to memory
	}
	s.keys[key{record, name}] = e
	return info, replaced, nil
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

func (s *Store) BigGet(record uint32, name string, now time.Time) (io.ReadCloser, ports.BigInfo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	e, ok := s.keys[key{record, name}]
	if !ok {
		return nil, ports.BigInfo{}, false, nil
	}
	if e.path == "" {
		return io.NopCloser(bytes.NewReader(e.data)), e.info, true, nil
	}
	if err := s.parentOK(e.path); err != nil {
		return nil, ports.BigInfo{}, false, err
	}
	f, err := os.OpenFile(e.path, os.O_RDONLY|nofollow|nonblock, 0)
	if err != nil {
		return nil, ports.BigInfo{}, false, err
	}
	return sectionCloser{io.NewSectionReader(f, 0, e.info.Size), f}, e.info, true, nil
}

func (s *Store) BigList(record uint32, now time.Time) ([]ports.BigInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	out := []ports.BigInfo{}
	for k, e := range s.keys {
		if k.record == record {
			out = append(out, e.info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) BigDelete(record uint32, name string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire(now)
	e, ok := s.keys[key{record, name}]
	if ok {
		s.remove(key{record, name}, e)
	}
	return ok, nil
}

func (s *Store) BigDrop(record uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.keys {
		if k.record == record {
			s.remove(k, e)
		}
	}
	return os.RemoveAll(filepath.Join(s.dir, strconv.FormatUint(uint64(record), 10)))
}

// BigRecords is the IDs of every record holding a key, for the start-up check
// against the registry.
func (s *Store) BigRecords() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[uint32]bool{}
	var out []uint32
	for k := range s.keys {
		if !seen[k.record] {
			seen[k.record] = true
			out = append(out, k.record)
		}
	}
	return out
}
