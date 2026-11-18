//go:build linux && !android

package desktop

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// busConfig is a private session bus that lets every client own names and
// call every method.
const busConfig = `<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:dir=%DIR%</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>
`

// testBus starts a private dbus-daemon for the test and points the session
// bus address at it. It skips the test where dbus-daemon is not installed.
func testBus(t *testing.T) {
	t.Helper()
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon not installed")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "bus.conf")
	if err := os.WriteFile(cfg, []byte(strings.ReplaceAll(busConfig, "%DIR%", dir)), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(daemon, "--config-file="+cfg, "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("dbus-daemon address: %v", err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", strings.TrimSpace(addr))
}

// fakePortal serves the OpenURI portal on the test bus. Like a portal
// older than 0.9, when legacy is set it ignores handle_token, picks its own
// Request path, and sends the response before replying to the call.
type fakePortal struct {
	conn   *dbus.Conn
	legacy bool
	uris   chan string // OpenURI's uri argument
	files  chan string // the contents of OpenFile's descriptor
}

// startPortal owns the portal's name on the test bus.
func startPortal(t *testing.T, legacy bool) *fakePortal {
	t.Helper()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	p := &fakePortal{conn: conn, legacy: legacy, uris: make(chan string, 1), files: make(chan string, 1)}
	if err := conn.Export(p, portalPath, "org.freedesktop.portal.OpenURI"); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(portalService, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: %v, %v", portalService, reply, err)
	}
	return p
}

// respond answers a call from sender with a successful Response on the
// Request object the portal picks, and returns that object's path.
func (p *fakePortal) respond(sender dbus.Sender, opts map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	token := "legacy_1"
	if !p.legacy {
		if err := opts["handle_token"].Store(&token); err != nil {
			return "", dbus.MakeFailedError(err)
		}
	}
	elem := strings.ReplaceAll(strings.TrimPrefix(string(sender), ":"), ".", "_")
	handle := dbus.ObjectPath(portalRequestPath + elem + "/" + token)
	if err := p.conn.Emit(handle, portalRequestIface+"."+portalResponse, uint32(responseSuccess), map[string]dbus.Variant{}); err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return handle, nil
}

// OpenURI implements org.freedesktop.portal.OpenURI.OpenURI.
func (p *fakePortal) OpenURI(sender dbus.Sender, _ string, uri string, opts map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	p.uris <- uri
	return p.respond(sender, opts)
}

// OpenFile implements org.freedesktop.portal.OpenURI.OpenFile.
func (p *fakePortal) OpenFile(sender dbus.Sender, _ string, fd dbus.UnixFD, opts map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	f := os.NewFile(uintptr(fd), "portal-fd")
	b, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	p.files <- string(b)
	return p.respond(sender, opts)
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestPortalOpenURL(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		testBus(t)
		p := startPortal(t, legacy)
		if err := (system{}).OpenURL(testContext(t), "https://example.com/"); err != nil {
			t.Fatalf("legacy %v: OpenURL: %v", legacy, err)
		}
		if got := <-p.uris; got != "https://example.com/" {
			t.Errorf("legacy %v: portal got %q", legacy, got)
		}
	}
}

// OpenURI.OpenURI rejects file: URIs; a local file goes to OpenFile as a
// descriptor.
func TestPortalOpenURLFile(t *testing.T) {
	testBus(t)
	p := startPortal(t, false)
	path := filepath.Join(t.TempDir(), "a b.txt")
	if err := os.WriteFile(path, []byte("contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := "file://" + strings.ReplaceAll(path, " ", "%20")
	if err := (system{}).OpenURL(testContext(t), uri); err != nil {
		t.Fatalf("OpenURL(%s): %v", uri, err)
	}
	select {
	case got := <-p.files:
		if got != "contents" {
			t.Errorf("portal read %q through the descriptor", got)
		}
	case uri := <-p.uris:
		t.Fatalf("file: URL sent to OpenURI as %q", uri)
	}
	if err := (system{}).OpenURL(testContext(t), "file:///no/such/file"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file: %v, want ErrNotExist", err)
	}
	if err := (system{}).OpenURL(testContext(t), "file://elsewhere/tmp/x"); err == nil {
		t.Error("remote file: URL: no error")
	}
}
