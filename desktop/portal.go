// Package desktop is giokit's bridge to the Linux desktop over D-Bus (XDG
// desktop portals): native file choosers now, more as apps need them. Calls
// block; run them off the UI goroutine with kit.Spawn.
package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/godbus/dbus/v5"
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

// ErrCancelled is returned when the user dismisses a chooser.
var ErrCancelled = errors.New("desktop: cancelled")

// OpenFile shows the desktop's native file chooser (the FileChooser portal)
// and returns the chosen local paths. It blocks until the user answers.
func OpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	opts := map[string]dbus.Variant{
		"multiple":  dbus.MakeVariant(o.Multiple),
		"directory": dbus.MakeVariant(o.Directory),
	}
	if len(o.Filters) > 0 {
		type pattern struct {
			Kind uint32
			Glob string
		}
		type filter struct {
			Name     string
			Patterns []pattern
		}
		var fs []filter
		for _, f := range o.Filters {
			ff := filter{Name: f.Name}
			for _, p := range f.Patterns {
				ff.Patterns = append(ff.Patterns, pattern{0, p})
			}
			fs = append(fs, ff)
		}
		opts["filters"] = dbus.MakeVariant(fs)
	}
	res, err := request(ctx, "org.freedesktop.portal.FileChooser.OpenFile", opts, "", o.Title)
	if err != nil {
		return nil, err
	}
	var uris []string
	if v, ok := res["uris"]; ok {
		_ = v.Store(&uris)
	}
	var paths []string
	for _, u := range uris {
		if p, err := url.Parse(u); err == nil && p.Scheme == "file" {
			paths = append(paths, p.Path)
		}
	}
	if len(paths) == 0 {
		return nil, ErrCancelled
	}
	return paths, nil
}

// request calls a portal method that answers asynchronously through a
// Request object, and waits for its Response signal.
func request(ctx context.Context, method string, opts map[string]dbus.Variant, args ...any) (map[string]dbus.Variant, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	b := make([]byte, 8)
	rand.Read(b)
	token := "giokit" + hex.EncodeToString(b)
	opts["handle_token"] = dbus.MakeVariant(token)
	// Subscribe before calling: the response may arrive before the call returns.
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	handle := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(handle), dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response")); err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	if err := obj.CallWithContext(ctx, method, 0, append(args, opts)...).Err; err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case s := <-signals:
			if s.Path != handle || len(s.Body) < 2 {
				continue
			}
			code, _ := s.Body[0].(uint32)
			res, _ := s.Body[1].(map[string]dbus.Variant)
			if code != 0 { // 1: cancelled, 2: other
				return nil, ErrCancelled
			}
			return res, nil
		}
	}
}
