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
	"slices"
)

// Filter is a named set of glob patterns for a file chooser ("CSV files",
// "*.csv").
type Filter struct {
	Name     string   // shown in the chooser
	Patterns []string // globs matched against file names, such as "*.csv"
}

// FileOptions configure OpenFile.
type FileOptions struct {
	Title     string   // the chooser's title; "" lets the platform choose
	Filters   []Filter // offered in order; none shows every file
	Multiple  bool     // allow choosing several
	Directory bool     // choose folders instead of files
}

// SaveOptions configure SaveFile.
type SaveOptions struct {
	Title   string   // the chooser's title; "" lets the platform choose
	Name    string   // suggested file name
	Folder  string   // suggested folder (a local path)
	Filters []Filter // offered in order; none shows every file
}

var (
	// ErrCancelled is returned when the user dismisses a chooser.
	ErrCancelled = errors.New("desktop: cancelled")
	// ErrUnsupported is returned where the system has no implementation.
	ErrUnsupported = errors.New("desktop: not supported on this system")
)

// Platform is the operating system's services. Methods block until the user
// answers or ctx is done. They return ErrCancelled when the user dismisses
// the dialog, ErrUnsupported where the system has no implementation, the
// context's error when ctx ends first, and a descriptive error when the
// system service fails.
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
// values you set. The zero value answers every call with zero results and
// no error. A Fake is not safe for concurrent use: inspect it after the
// calls under test have finished (kittest's RunTasks waits for kit.Spawn).
type Fake struct {
	Open    []string // OpenFile's answer; each call returns a copy
	Save    string   // SaveFile's answer
	Err     error    // returned by every call when set, alongside the answer
	Opened  []string // URLs passed to OpenURL, in order
	Choices []Choice // the chooser calls received, in order
}

// Choice is one chooser call a Fake received. Exactly one of Open and
// Save is set, to the options passed (their Filters slice is shared with
// the caller).
type Choice struct {
	Open *FileOptions // the options of an OpenFile call
	Save *SaveOptions // the options of a SaveFile call
}

// OpenFile records o and returns a copy of f.Open and f.Err.
func (f *Fake) OpenFile(_ context.Context, o FileOptions) ([]string, error) {
	f.Choices = append(f.Choices, Choice{Open: &o})
	return slices.Clone(f.Open), f.Err
}

// SaveFile records o and returns f.Save and f.Err.
func (f *Fake) SaveFile(_ context.Context, o SaveOptions) (string, error) {
	f.Choices = append(f.Choices, Choice{Save: &o})
	return f.Save, f.Err
}

// OpenURL records url and returns f.Err.
func (f *Fake) OpenURL(_ context.Context, url string) error {
	f.Opened = append(f.Opened, url)
	return f.Err
}
