//go:build windows

package desktop

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

// OpenURL opens the link with ShellExecute, in the user's default
// browser, mail program or file viewer.
func (system) OpenURL(_ context.Context, uri string) error {
	if err := checkLink(uri); err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return fmt.Errorf("desktop: open %q: %w", uri, err)
	}
	if err := windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("desktop: open %q: %w", uri, err)
	}
	return nil
}
