package store

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall/js"
)

// Open returns the Store for the app called name. In a browser that is
// localStorage, each entry under the key "name/entry"; see the other
// platforms' Open for the rest of the contract. It fails where the
// browser has no localStorage or blocks it (some private modes).
func Open(name string) (st Store, err error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	defer recoverJS(&err)
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return nil, errors.New("store: localStorage is unavailable")
	}
	return webStore{ls: ls, prefix: name + "/"}, nil
}

// webStore keeps entries in localStorage. Browsers run Go's wasm on one
// thread, so it needs no lock.
type webStore struct {
	ls     js.Value
	prefix string
}

func (s webStore) Load(name string) (b []byte, err error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	defer recoverJS(&err)
	v := s.ls.Call("getItem", s.prefix+name)
	if v.IsNull() || v.IsUndefined() {
		return nil, &fs.PathError{Op: "load", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(v.String()), nil
}

// Save stores b as a string: localStorage holds text, so entries should be
// text (JSON, numbers) to round-trip exactly.
func (s webStore) Save(name string, b []byte) (err error) {
	if err := checkName(name); err != nil {
		return err
	}
	defer recoverJS(&err) // quota exceeded, storage disabled
	s.ls.Call("setItem", s.prefix+name, string(b))
	return nil
}

// recoverJS turns a thrown JavaScript exception into an error.
func recoverJS(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("store: localStorage: %v", r)
	}
}
