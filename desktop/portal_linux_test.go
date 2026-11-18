//go:build linux && !android

package desktop

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestResponseCodes(t *testing.T) {
	results := map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///tmp/a"})}
	res, err := response(methodOpenFile, []any{uint32(responseSuccess), results})
	if err != nil || len(res) != 1 {
		t.Fatalf("success: %v, %v", res, err)
	}
	if _, err := response(methodOpenFile, []any{uint32(responseCancelled), results}); !errors.Is(err, ErrCancelled) {
		t.Errorf("cancelled: %v, want ErrCancelled", err)
	}
	for _, code := range []uint32{uint32(responseEnded), 7} {
		_, err := response(methodOpenFile, []any{code, results})
		if err == nil || errors.Is(err, ErrCancelled) {
			t.Errorf("code %d: %v, want a non-cancel error", code, err)
		}
	}
}

func TestResponseMalformed(t *testing.T) {
	results := map[string]dbus.Variant{}
	for name, body := range map[string][]any{
		"short":       {uint32(0)},
		"long":        {uint32(0), results, "extra"},
		"code type":   {int32(0), results},
		"result type": {uint32(0), "not a map"},
	} {
		if _, err := response(methodOpenFile, body); err == nil || errors.Is(err, ErrCancelled) {
			t.Errorf("%s: %v, want an error", name, err)
		}
	}
}

func TestPaths(t *testing.T) {
	res := map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///tmp/a%20b.csv", "https://example.com/x", "file:///tmp/c"})}
	got, err := paths(methodOpenFile, res)
	if want := []string{"/tmp/a b.csv", "/tmp/c"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("paths = %q, %v; want %q", got, err, want)
	}
	for name, res := range map[string]map[string]dbus.Variant{
		"missing":    {},
		"wrong type": {"uris": dbus.MakeVariant(uint32(1))},
		"no files":   {"uris": dbus.MakeVariant([]string{"https://example.com"})},
		"bad uri":    {"uris": dbus.MakeVariant([]string{"file://%zz"})},
	} {
		if _, err := paths(methodOpenFile, res); err == nil || errors.Is(err, ErrCancelled) {
			t.Errorf("%s: %v, want an error", name, err)
		}
	}
}

func TestPortalFilters(t *testing.T) {
	got := portalFilters([]Filter{{Name: "CSV", Patterns: []string{"*.csv", "*.tsv"}}})
	if len(got) != 1 || got[0].Name != "CSV" || len(got[0].Patterns) != 2 || got[0].Patterns[1] != (portalPattern{Kind: patternGlob, Pattern: "*.tsv"}) {
		t.Fatalf("portalFilters = %+v", got)
	}
	if sig := dbus.SignatureOf(got).String(); sig != "a(sa(us))" {
		t.Fatalf("signature %s, want a(sa(us))", sig)
	}
}

// A portal older than 0.9 picks its own Request path: the wait follows the
// handle the call returned, not the one handle_token predicted.
func TestAwaitResponseFollowsHandle(t *testing.T) {
	const predicted, returned = portalRequestPath + "1_42/giokit_X", portalRequestPath + "1_42/t7"
	name := portalRequestIface + "." + portalResponse
	ok := []any{uint32(responseSuccess), map[string]dbus.Variant{}}
	signals := make(chan *dbus.Signal, 3)
	signals <- &dbus.Signal{Path: predicted, Name: name, Body: []any{uint32(responseCancelled), map[string]dbus.Variant{}}}
	signals <- &dbus.Signal{Path: returned, Name: "org.example.Other", Body: ok}
	signals <- &dbus.Signal{Path: returned, Name: name, Body: ok}
	if _, err := awaitResponse(context.Background(), methodOpenURI, returned, signals); err != nil {
		t.Fatalf("awaitResponse: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := awaitResponse(ctx, methodOpenURI, returned, signals); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestInNamespace(t *testing.T) {
	const ns = portalRequestPath + "1_42"
	for p, want := range map[dbus.ObjectPath]bool{
		ns:                              true,
		ns + "/t7":                      true,
		portalRequestPath + "1_420/t7":  false,
		"/org/freedesktop/portal/other": false,
	} {
		if got := inNamespace(p, ns); got != want {
			t.Errorf("inNamespace(%s) = %v, want %v", p, got, want)
		}
	}
}
