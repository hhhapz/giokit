// Package store keeps an app's persistent data: small named entries, such
// as settings, a saved game or a sync token, each read and replaced whole.
//
// Open gives the platform's place for app data, so an app never looks for a
// config directory itself (Android has none):
//
//	st, err := store.Open("myapp")
//	if err != nil {
//		log.Printf("settings won't be saved: %v", err)
//		st = store.Memory()
//	}
//	b, err := st.Load("settings.json") // fs.ErrNotExist before the first Save
//	...
//	err = st.Save("settings.json", b)
//
// Tests use Dir(t.TempDir()) to persist across a simulated restart, or
// Memory() for state that doesn't need to outlive the test.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store holds named entries. Names are plain file names ("save.json",
// "volume"): no path separators, and not "." or "..". Implementations are
// safe from any goroutine.
type Store interface {
	// Load returns the entry's contents, or an error matching
	// fs.ErrNotExist (errors.Is) if it was never saved.
	Load(name string) ([]byte, error)
	// Save replaces the entry with b, all at once: a crash or power loss
	// mid-save leaves the old contents or the new, never a mix.
	Save(name string, b []byte) error
}

// IsNotExist reports whether err is Load's "never saved" error.
func IsNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// checkName rejects names that aren't plain file names.
func checkName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("store: bad entry name %q", name)
	}
	return nil
}

// Dir returns a Store that keeps each entry as a file in dir, created on
// the first Save.
func Dir(dir string) Store { return dirStore(dir) }

type dirStore string

func (d dirStore) Load(name string) ([]byte, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(string(d), name))
}

// Save writes a temporary file beside the entry and renames it over the
// entry, which replaces it atomically.
func (d dirStore) Save(name string, b []byte) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := os.MkdirAll(string(d), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(string(d), "."+name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(string(d), name))
}

// Memory returns an empty Store that keeps entries in memory, for tests
// and for running on when Open fails. Load and Save copy, so callers may
// reuse their slices.
func Memory() Store { return &memStore{m: map[string][]byte{}} }

type memStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memStore) Load(name string) ([]byte, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[name]
	if !ok {
		return nil, &fs.PathError{Op: "load", Path: name, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), b...), nil
}

func (s *memStore) Save(name string, b []byte) error {
	if err := checkName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[name] = append([]byte(nil), b...)
	return nil
}
