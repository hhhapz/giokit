//go:build linux && !android

package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/godbus/dbus/v5"
)

// system is the Linux desktop through the XDG portals.
type system struct{}

// portalFilters converts filters to the portal's a(sa(us)) form.
func portalFilters(filters []Filter) any {
	type pattern struct {
		Kind uint32
		Glob string
	}
	type filter struct {
		Name     string
		Patterns []pattern
	}
	var fs []filter
	for _, f := range filters {
		ff := filter{Name: f.Name}
		for _, p := range f.Patterns {
			ff.Patterns = append(ff.Patterns, pattern{0, p})
		}
		fs = append(fs, ff)
	}
	return fs
}

// paths keeps the local paths of a portal response's uris.
func paths(res map[string]dbus.Variant) []string {
	var uris []string
	if v, ok := res["uris"]; ok {
		_ = v.Store(&uris)
	}
	var out []string
	for _, u := range uris {
		if p, err := url.Parse(u); err == nil && p.Scheme == "file" {
			out = append(out, p.Path)
		}
	}
	return out
}

// OpenFile uses the FileChooser portal's OpenFile.
func (system) OpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	opts := map[string]dbus.Variant{
		"multiple":  dbus.MakeVariant(o.Multiple),
		"directory": dbus.MakeVariant(o.Directory),
	}
	if len(o.Filters) > 0 {
		opts["filters"] = dbus.MakeVariant(portalFilters(o.Filters))
	}
	res, err := request(ctx, "org.freedesktop.portal.FileChooser.OpenFile", opts, "", o.Title)
	if err != nil {
		return nil, err
	}
	ps := paths(res)
	if len(ps) == 0 {
		return nil, ErrCancelled
	}
	return ps, nil
}

// SaveFile uses the FileChooser portal's SaveFile.
func (system) SaveFile(ctx context.Context, o SaveOptions) (string, error) {
	opts := map[string]dbus.Variant{}
	if o.Name != "" {
		opts["current_name"] = dbus.MakeVariant(o.Name)
	}
	if o.Folder != "" {
		opts["current_folder"] = dbus.MakeVariant(append([]byte(o.Folder), 0)) // ay, NUL-terminated
	}
	if len(o.Filters) > 0 {
		opts["filters"] = dbus.MakeVariant(portalFilters(o.Filters))
	}
	res, err := request(ctx, "org.freedesktop.portal.FileChooser.SaveFile", opts, "", o.Title)
	if err != nil {
		return "", err
	}
	if ps := paths(res); len(ps) > 0 {
		return ps[0], nil
	}
	return "", ErrCancelled
}

// OpenURL uses the OpenURI portal.
func (system) OpenURL(ctx context.Context, uri string) error {
	_, err := request(ctx, "org.freedesktop.portal.OpenURI.OpenURI", map[string]dbus.Variant{}, "", uri)
	return err
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
