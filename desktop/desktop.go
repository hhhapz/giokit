// Package desktop is giokit's bridge to the operating system: native file
// choosers and opening links. Linux uses the XDG desktop portals over
// D-Bus; other systems return ErrUnsupported until they get an
// implementation. Calls block; run them off the UI goroutine with
// kit.Spawn.
//
// Code against Platform (System() in the app, a *Fake in tests) so the
// calls can be checked without a desktop.
package desktop

import (
	"context"
	"errors"
)

// Filter is a named set of glob patterns for a file chooser ("CSV files",
// "*.csv").
type Filter struct {
	Name     string
	Patterns []string
}

// FileOptions configure OpenFile.
type FileOptions struct {
	Title     string
	Filters   []Filter
	Multiple  bool
	Directory bool // choose folders instead of files
}

// SaveOptions configure SaveFile.
type SaveOptions struct {
	Title   string
	Name    string // suggested file name
	Folder  string // suggested folder (a local path)
	Filters []Filter
}

var (
	// ErrCancelled is returned when the user dismisses a chooser.
	ErrCancelled = errors.New("desktop: cancelled")
	// ErrUnsupported is returned where the system has no implementation.
	ErrUnsupported = errors.New("desktop: not supported on this system")
)

// Platform is the operating system's services. Methods block and may return
// ErrCancelled or ErrUnsupported.
type Platform interface {
	// OpenFile shows the native file chooser and returns the chosen paths.
	OpenFile(ctx context.Context, o FileOptions) ([]string, error)
	// SaveFile shows the native save dialog and returns the chosen path.
	SaveFile(ctx context.Context, o SaveOptions) (string, error)
	// OpenURL opens a link (https:, mailto:, file:) in the user's default
	// application.
	OpenURL(ctx context.Context, url string) error
}

// System is this operating system's Platform.
func System() Platform { return system{} }

// OpenFile is System().OpenFile.
func OpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	return System().OpenFile(ctx, o)
}

// SaveFile is System().SaveFile.
func SaveFile(ctx context.Context, o SaveOptions) (string, error) {
	return System().SaveFile(ctx, o)
}

// OpenURL is System().OpenURL.
func OpenURL(ctx context.Context, url string) error { return System().OpenURL(ctx, url) }

// Fake is a Platform for tests: it records calls and answers with the
// values you set.
type Fake struct {
	Open    []string // OpenFile's answer
	Save    string   // SaveFile's answer
	Err     error    // returned by every call when set
	Opened  []string // URLs passed to OpenURL
	Choices []any    // the FileOptions and SaveOptions passed, in order
}

func (f *Fake) OpenFile(_ context.Context, o FileOptions) ([]string, error) {
	f.Choices = append(f.Choices, o)
	return f.Open, f.Err
}

func (f *Fake) SaveFile(_ context.Context, o SaveOptions) (string, error) {
	f.Choices = append(f.Choices, o)
	return f.Save, f.Err
}

func (f *Fake) OpenURL(_ context.Context, url string) error {
	f.Opened = append(f.Opened, url)
	return f.Err
}
