//go:build !js

package store

import (
	"path/filepath"

	"github.com/hhhapz/giokit/gio/app"
)

// Open returns the Store for the app called name: a directory named name
// in the platform's app data directory (Gio's app.DataDir). That is
// os.UserConfigDir on desktops (~/.config/name on Linux), the app's
// private files directory on Android, and its Documents directory on iOS.
// In a browser it is localStorage, each entry under the key "name/entry".
// Name is usually the app's short name ("myapp"); keep it once released,
// or saved data is left behind.
//
// On Android, call Open from main or later, never from a package init.
func Open(name string) (Store, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	dir, err := app.DataDir()
	if err != nil {
		return nil, err
	}
	return Dir(filepath.Join(dir, name)), nil
}
