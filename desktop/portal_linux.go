//go:build linux && !android

package desktop

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"
)

// system is the Linux desktop through the XDG portals. Where the session
// has no FileChooser portal (no session bus, or no portal service
// implementing it), the choosers fall back to the zenity or kdialog
// programs (helper_linux.go). The zero value runs real processes.
type system struct {
	cmds commands // the fallback's process seam
}

// errNoPortal marks an error that means the portal is missing, not that it
// failed: the session bus is unreachable, or nothing on it implements the
// method.
var errNoPortal = errors.New("portal unavailable")

// missingNames are the D-Bus errors that mean nothing serves a method.
var missingNames = []string{
	"org.freedesktop.DBus.Error.ServiceUnknown",
	"org.freedesktop.DBus.Error.NameHasNoOwner",
	"org.freedesktop.DBus.Error.UnknownMethod",
	"org.freedesktop.DBus.Error.UnknownInterface",
	"org.freedesktop.DBus.Error.UnknownObject",
}

// missing reports whether err, from a method call, means nothing on the
// bus serves the method, including a service that failed to activate.
func missing(err error) bool {
	var e dbus.Error
	if !errors.As(err, &e) {
		var p *dbus.Error
		if !errors.As(err, &p) || p == nil {
			return false
		}
		e = *p
	}
	return slices.Contains(missingNames, e.Name) || strings.HasPrefix(e.Name, "org.freedesktop.DBus.Error.Spawn.")
}

// D-Bus names of the portal service and the methods giokit calls.
const (
	portalService      = "org.freedesktop.portal.Desktop"
	portalPath         = "/org/freedesktop/portal/desktop"
	portalRequestPath  = portalPath + "/request/"
	portalRequestIface = "org.freedesktop.portal.Request"
	portalResponse     = "Response"
	methodOpenFile     = "org.freedesktop.portal.FileChooser.OpenFile"
	methodSaveFile     = "org.freedesktop.portal.FileChooser.SaveFile"
	methodOpenURI      = "org.freedesktop.portal.OpenURI.OpenURI"
	methodOpenURIFile  = "org.freedesktop.portal.OpenURI.OpenFile"
)

// responseCode is the first value of a Request's Response signal.
type responseCode uint32

const (
	responseSuccess   responseCode = 0 // the interaction completed
	responseCancelled responseCode = 1 // the user cancelled it
	responseEnded     responseCode = 2 // it ended some other way, such as an error in the portal
)

// patternKind says how a FileChooser filter pattern matches files.
type patternKind uint32

// patternGlob is a shell glob such as "*.csv", the only kind giokit sends.
const patternGlob patternKind = 0

// portalPattern is one (us) element of a FileChooser filter.
type portalPattern struct {
	Kind    patternKind
	Pattern string
}

// portalFilter is one (sa(us)) element of the FileChooser "filters" option:
// a user-visible name and the patterns it matches.
type portalFilter struct {
	Name     string
	Patterns []portalPattern
}

// portalFilters converts filters to the FileChooser's a(sa(us)) form. Every
// pattern is a glob.
func portalFilters(filters []Filter) []portalFilter {
	fs := make([]portalFilter, 0, len(filters))
	for _, f := range filters {
		pf := portalFilter{Name: f.Name}
		for _, p := range f.Patterns {
			pf.Patterns = append(pf.Patterns, portalPattern{Kind: patternGlob, Pattern: p})
		}
		fs = append(fs, pf)
	}
	return fs
}

// paths returns the local paths of a FileChooser result's "uris" (as).
// A result without uris, or with no file: uris among them, is an error: the
// portal reports a dismissed chooser with responseCancelled, not with an
// empty success.
func paths(method string, res map[string]dbus.Variant) ([]string, error) {
	v, ok := res["uris"]
	if !ok {
		return nil, fmt.Errorf("desktop: %s: result has no uris", method)
	}
	var uris []string
	if err := v.Store(&uris); err != nil {
		return nil, fmt.Errorf("desktop: %s: uris: %w", method, err)
	}
	var out []string
	for _, u := range uris {
		p, err := url.Parse(u)
		if err != nil {
			return nil, fmt.Errorf("desktop: %s: %w", method, err)
		}
		if p.Scheme == "file" {
			out = append(out, p.Path)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("desktop: %s: no local files among %q", method, uris)
	}
	return out, nil
}

// OpenFile uses the FileChooser portal's OpenFile, or zenity or kdialog
// without the portal.
func (s system) OpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	ps, err := portalOpenFile(ctx, o)
	if errors.Is(err, errNoPortal) {
		return s.cmds.choose(ctx, err, o.Multiple, func(h helper) []string { return h.open(o) })
	}
	return ps, err
}

// SaveFile uses the FileChooser portal's SaveFile, or zenity or kdialog
// without the portal.
func (s system) SaveFile(ctx context.Context, o SaveOptions) (string, error) {
	p, err := portalSaveFile(ctx, o)
	if errors.Is(err, errNoPortal) {
		var ps []string
		if ps, err = s.cmds.choose(ctx, err, false, func(h helper) []string { return h.save(o) }); err != nil {
			return "", err
		}
		return ps[0], nil
	}
	return p, err
}

// portalOpenFile calls the FileChooser portal's OpenFile.
func portalOpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	opts := map[string]dbus.Variant{
		"multiple":  dbus.MakeVariant(o.Multiple),
		"directory": dbus.MakeVariant(o.Directory),
	}
	if len(o.Filters) > 0 {
		opts["filters"] = dbus.MakeVariant(portalFilters(o.Filters))
	}
	res, err := request(ctx, methodOpenFile, opts, "", o.Title)
	if err != nil {
		return nil, err
	}
	return paths(methodOpenFile, res)
}

// portalSaveFile calls the FileChooser portal's SaveFile.
func portalSaveFile(ctx context.Context, o SaveOptions) (string, error) {
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
	res, err := request(ctx, methodSaveFile, opts, "", o.Title)
	if err != nil {
		return "", err
	}
	ps, err := paths(methodSaveFile, res)
	if err != nil {
		return "", err
	}
	return ps[0], nil
}

// OpenURL uses the OpenURI portal. OpenURI.OpenURI rejects file: URIs, so a
// local file or folder goes to OpenURI.OpenFile as an open descriptor.
func (system) OpenURL(ctx context.Context, uri string) error {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		return openLocal(ctx, u)
	}
	_, err := request(ctx, methodOpenURI, map[string]dbus.Variant{}, "", uri)
	return err
}

// openLocal opens the file or folder that a file: URL names with
// OpenURI.OpenFile, which takes a read-only descriptor of it.
func openLocal(ctx context.Context, u *url.URL) error {
	if u.Host != "" && u.Host != "localhost" {
		return fmt.Errorf("desktop: %s: %q is not on this machine", methodOpenURIFile, u)
	}
	// os.Open opens with O_RDONLY|O_CLOEXEC. The bus hands the portal a
	// copy of the descriptor, so f may close once the call has returned.
	f, err := os.Open(u.Path)
	if err != nil {
		return fmt.Errorf("desktop: %s: %w", methodOpenURIFile, err)
	}
	defer f.Close()
	_, err = request(ctx, methodOpenURIFile, map[string]dbus.Variant{}, "", dbus.UnixFD(f.Fd()))
	return err
}

// request calls a portal method that answers asynchronously through a
// Request object, and waits for its Response signal. args precede opts in
// the call; request adds opts' "handle_token". Its error wraps errNoPortal
// when there is no session bus or nothing on it serves method.
func request(ctx context.Context, method string, opts map[string]dbus.Variant, args ...any) (map[string]dbus.Variant, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("desktop: %s: %w: %w", method, errNoPortal, err)
	}
	// Closing is best effort: by then the call has succeeded or failed, and
	// a close error leaves the caller nothing to act on.
	defer conn.Close()

	// The portal names the Request object
	// /org/freedesktop/portal/desktop/request/SENDER/TOKEN, where SENDER is
	// the caller's unique bus name without its ':' and with '.' as '_', and
	// TOKEN is handle_token. Both must be valid object path elements
	// ([A-Za-z0-9_]); rand.Text is base32 ([A-Z2-7]), so it qualifies.
	// Portals older than 0.9 ignore handle_token and pick TOKEN themselves,
	// so request subscribes to every Request under SENDER and then waits for
	// the handle the call returns.
	token := "giokit_" + rand.Text()
	opts["handle_token"] = dbus.MakeVariant(token)
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	requests := dbus.ObjectPath(portalRequestPath + sender)

	// Subscribe before calling: the response may arrive before the call
	// returns. The connection is this request's own, so no other request's
	// response arrives on it.
	if err := subscribe(conn, dbus.WithMatchPathNamespace(requests)); err != nil {
		return nil, fmt.Errorf("desktop: %s: subscribe: %w", method, err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	obj := conn.Object(portalService, portalPath)
	var handle dbus.ObjectPath
	if err := obj.CallWithContext(ctx, method, 0, append(args, opts)...).Store(&handle); err != nil {
		if missing(err) {
			return nil, fmt.Errorf("desktop: %s: %w: %w", method, errNoPortal, err)
		}
		return nil, fmt.Errorf("desktop: %s: %w", method, err)
	}
	if !inNamespace(handle, requests) {
		// A response sent before this subscription is lost, but the portal
		// answers once the user has, so it almost always comes later.
		if err := subscribe(conn, dbus.WithMatchObjectPath(handle)); err != nil {
			return nil, fmt.Errorf("desktop: %s: subscribe: %w", method, err)
		}
	}
	return awaitResponse(ctx, method, handle, signals)
}

// subscribe asks the bus for the Response signals of the Request objects
// that path matches.
func subscribe(conn *dbus.Conn, path dbus.MatchOption) error {
	return conn.AddMatchSignal(path, dbus.WithMatchInterface(portalRequestIface), dbus.WithMatchMember(portalResponse))
}

// inNamespace reports whether p is ns or below it, as D-Bus's
// path_namespace match rule decides.
func inNamespace(p, ns dbus.ObjectPath) bool {
	return p == ns || strings.HasPrefix(string(p), string(ns)+"/")
}

// awaitResponse waits among signals for the Response of the Request object
// at handle, skipping every other signal, and interprets it. It returns
// ctx's error if ctx ends first.
func awaitResponse(ctx context.Context, method string, handle dbus.ObjectPath, signals <-chan *dbus.Signal) (map[string]dbus.Variant, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case s := <-signals:
			if s.Path != handle || s.Name != portalRequestIface+"."+portalResponse {
				continue
			}
			return response(method, s.Body)
		}
	}
}

// response interprets a Response signal's body (ua{sv}): the results on
// success, ErrCancelled if the user cancelled, and an error otherwise.
func response(method string, body []any) (map[string]dbus.Variant, error) {
	if len(body) != 2 {
		return nil, fmt.Errorf("desktop: %s: response has %d values, want 2", method, len(body))
	}
	code, ok := body[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("desktop: %s: response code is %T, want uint32", method, body[0])
	}
	results, ok := body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, fmt.Errorf("desktop: %s: response results are %T, want a{sv}", method, body[1])
	}
	switch responseCode(code) {
	case responseSuccess:
		return results, nil
	case responseCancelled:
		return nil, ErrCancelled
	case responseEnded:
		return nil, fmt.Errorf("desktop: %s: the portal ended the interaction", method)
	default:
		return nil, fmt.Errorf("desktop: %s: unknown response code %d", method, code)
	}
}
