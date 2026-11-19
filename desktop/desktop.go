// Package desktop is giokit's bridge to the operating system: native file
// choosers and opening links.
//
//   - Linux uses the XDG desktop portals over D-Bus. Without a FileChooser
//     portal the choosers fall back to zenity, then kdialog (kdialog first
//     on KDE Plasma); with neither installed they return ErrUnsupported.
//   - Windows uses the Common Item Dialog (IFileOpenDialog,
//     IFileSaveDialog) through COM, without cgo, and ShellExecute for links.
//   - macOS uses NSOpenPanel and NSSavePanel through cgo, and open(1) for
//     links. Built without cgo, the choosers return ErrUnsupported.
//   - Other systems return ErrUnsupported.
//
// Calls block until the user answers: run them off the UI goroutine with
// kit.Spawn. On macOS this is required, not just advised: the panel runs on
// the main thread, which may be waiting for the UI goroutine's frame. The
// choosers have no parent window (Platform takes none), so they are not
// modal to the app's windows and the app keeps drawing while one is open.
// When ctx ends, a chooser returns ctx's error at once and asks the
// native dialog to close as if cancelled (see each system's notes in its
// source: on Windows a nested prompt such as "replace the file?" stays
// until answered, and its answer is discarded).
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
// "*.csv"). macOS panels take one set of extensions rather than a list of
// filters: there every filter's "*.ext" patterns are merged, and any other
// pattern ("*", "data-*.csv") or a filter without patterns shows every
// file.
type Filter struct {
	Name     string   // shown in the chooser
	Patterns []string // globs matched against file names, such as "*.csv"
}

// FileOptions configure OpenFile.
type FileOptions struct {
	Title     string   // the chooser's title; "" lets the platform choose
	Filters   []Filter // offered in order; none shows every file
	Multiple  bool     // allow choosing several
	Directory bool     // choose folders instead of files; Filters do not apply
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
// answers or ctx is done, and may be called from any goroutine but the UI
// goroutine. They return ErrCancelled when the user dismisses the dialog,
// ErrUnsupported (possibly wrapped) where the system has no
// implementation, the context's error when ctx ends first, and a
// descriptive error when the system service fails. Every system returns
// the same errors for the same outcomes.
type Platform interface {
	// OpenFile shows the native file chooser and returns the chosen local
	// paths: at least one on success, and only one unless o.Multiple.
	OpenFile(ctx context.Context, o FileOptions) ([]string, error)
	// SaveFile shows the native save dialog and returns the chosen local
	// path. The dialog asks before replacing an existing file.
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
