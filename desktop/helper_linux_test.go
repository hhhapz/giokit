//go:build linux && !android

package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/godbus/dbus/v5"
)

var testFilters = []Filter{{Name: "Tables", Patterns: []string{"*.csv", "*.tsv"}}, {Name: "A|B", Patterns: []string{"*.ab"}}, {}}

func TestZenityArgs(t *testing.T) {
	for _, c := range []struct {
		got, want []string
	}{
		{zenityOpenArgs(FileOptions{}), []string{"--file-selection", "--separator=//"}},
		{
			zenityOpenArgs(FileOptions{Title: "Import", Multiple: true, Filters: testFilters}),
			[]string{"--file-selection", "--separator=//", "--title=Import", "--multiple",
				"--file-filter=Tables | *.csv *.tsv", "--file-filter=A B | *.ab", "--file-filter=* | *"},
		},
		{
			zenityOpenArgs(FileOptions{Directory: true, Filters: testFilters}),
			[]string{"--file-selection", "--separator=//", "--directory"},
		},
		{zenitySaveArgs(SaveOptions{}), []string{"--file-selection", "--save", "--confirm-overwrite"}},
		{
			zenitySaveArgs(SaveOptions{Title: "Export", Name: "out.csv", Folder: "/tmp/x", Filters: testFilters[:1]}),
			[]string{"--file-selection", "--save", "--confirm-overwrite", "--title=Export", "--filename=/tmp/x/out.csv", "--file-filter=Tables | *.csv *.tsv"},
		},
		{zenitySaveArgs(SaveOptions{Folder: "/tmp/x/"}), []string{"--file-selection", "--save", "--confirm-overwrite", "--filename=/tmp/x/"}},
		{zenitySaveArgs(SaveOptions{Name: "out.csv"}), []string{"--file-selection", "--save", "--confirm-overwrite", "--filename=out.csv"}},
	} {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("got  %q\nwant %q", c.got, c.want)
		}
	}
}

func TestKdialogArgs(t *testing.T) {
	for _, c := range []struct {
		got, want []string
	}{
		{kdialogOpenArgs(FileOptions{}), []string{"--getopenfilename", "."}},
		{
			kdialogOpenArgs(FileOptions{Title: "Import", Multiple: true, Filters: testFilters}),
			[]string{"--title", "Import", "--multiple", "--separate-output", "--getopenfilename", ".", "*.csv *.tsv|Tables\n*.ab|A|B\n*|*"},
		},
		{kdialogOpenArgs(FileOptions{Directory: true, Multiple: true, Filters: testFilters}), []string{"--getexistingdirectory", "."}},
		{kdialogSaveArgs(SaveOptions{}), []string{"--getsavefilename", "."}},
		{
			kdialogSaveArgs(SaveOptions{Title: "Export", Name: "out.csv", Folder: "/tmp/x", Filters: testFilters[:1]}),
			[]string{"--title", "Export", "--getsavefilename", "/tmp/x/out.csv", "*.csv *.tsv|Tables"},
		},
	} {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("got  %q\nwant %q", c.got, c.want)
		}
	}
}

func TestHelperPaths(t *testing.T) {
	dir := t.TempDir()
	a, c := filepath.Join(dir, "a b.csv"), filepath.Join(dir, "c")
	for _, p := range []string{a, c} {
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name     string
		h        helper
		out      string
		multiple bool
		want     []string
	}{
		// A single name keeps its newlines and carriage returns: they are
		// legal in file names (audit 16, CP-05).
		{"one", zenity, "/tmp/quarter\nreport.csv\n", false, []string{"/tmp/quarter\nreport.csv"}},
		{"one cr", kdialog, "/tmp/a\r\n", false, []string{"/tmp/a\r"}},
		{"zenity many", zenity, "/tmp/a\nb//" + "/tmp/c\n", true, []string{"/tmp/a\nb", "/tmp/c"}},
		{"kdialog many", kdialog, a + "\n" + c + "\n", true, []string{a, c}},
	} {
		got, err := helperPaths(tc.h, commandResult{stdout: []byte(tc.out)}, tc.multiple)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: paths = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := helperPaths(zenity, commandResult{code: helperCancelled}, false); !errors.Is(err, ErrCancelled) {
		t.Errorf("cancelled: %v, want ErrCancelled", err)
	}
	for name, tc := range map[string]struct {
		h        helper
		res      commandResult
		multiple bool
	}{
		"failed": {zenity, commandResult{code: 255, stderr: []byte("cannot open display")}, false},
		"empty":  {zenity, commandResult{stdout: []byte("\n")}, false},
		// kdialog's newline list is ambiguous: a name holding a newline
		// splits into paths that do not exist, or are not absolute.
		"kdialog split":    {kdialog, commandResult{stdout: []byte(dir + "/quarter\n" + dir + "/report.csv\n")}, true},
		"kdialog relative": {kdialog, commandResult{stdout: []byte(a + "\nreport.csv\n")}, true},
	} {
		if _, err := helperPaths(tc.h, tc.res, tc.multiple); err == nil || errors.Is(err, ErrCancelled) {
			t.Errorf("%s: %v, want an error", name, err)
		}
	}
}

// fakeCommands is a commands whose installed programs and answer are set
// by the test, recording the command lines it receives.
type fakeCommands struct {
	installed []string
	desktop   string
	answer    commandResult
	ran       [][]string // program then arguments
}

func (f *fakeCommands) commands() commands {
	return commands{
		look: func(name string) (string, error) {
			if slices.Contains(f.installed, name) {
				return "/usr/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		},
		run: func(_ context.Context, path string, args []string) (commandResult, error) {
			f.ran = append(f.ran, append([]string{filepath.Base(path)}, args...))
			return f.answer, nil
		},
		getenv: func(key string) string {
			if key == "XDG_CURRENT_DESKTOP" {
				return f.desktop
			}
			return ""
		},
	}
}

func TestChooseOrder(t *testing.T) {
	ctx := context.Background()
	answer := commandResult{stdout: []byte("/tmp/a\n")}
	for _, c := range []struct {
		installed []string
		desktop   string
		want      string
	}{
		{[]string{"zenity", "kdialog"}, "GNOME", "zenity"},
		{[]string{"zenity", "kdialog"}, "KDE", "kdialog"},
		{[]string{"zenity", "kdialog"}, "ubuntu:kde", "kdialog"},
		{[]string{"kdialog"}, "", "kdialog"},
		{[]string{"zenity"}, "KDE", "zenity"},
	} {
		f := &fakeCommands{installed: c.installed, desktop: c.desktop, answer: answer}
		ps, err := f.commands().choose(ctx, errNoPortal, false, func(h helper) []string { return h.open(FileOptions{}) })
		if err != nil || !slices.Equal(ps, []string{"/tmp/a"}) || len(f.ran) != 1 || f.ran[0][0] != c.want {
			t.Errorf("%v on %q: ran %q, got %q, %v; want %s", c.installed, c.desktop, f.ran, ps, err, c.want)
		}
	}
	f := &fakeCommands{}
	if _, err := f.commands().choose(ctx, errNoPortal, false, func(h helper) []string { return nil }); !errors.Is(err, ErrUnsupported) || !errors.Is(err, errNoPortal) {
		t.Errorf("nothing installed: %v, want ErrUnsupported wrapping the portal error", err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	f = &fakeCommands{installed: []string{"zenity"}, answer: answer}
	if _, err := f.commands().choose(cctx, errNoPortal, false, func(h helper) []string { return nil }); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled ctx: %v", err)
	}
}

// A real process's exit code reaches helperPaths; ctx kills it.
func TestRunCommand(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	res, err := runCommand(context.Background(), sh, []string{"-c", "echo /tmp/a; echo oops >&2; exit 1"})
	if err != nil || res.code != 1 || string(res.stdout) != "/tmp/a\n" || string(res.stderr) != "oops\n" {
		t.Fatalf("runCommand = %+v, %v", res, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runCommand(ctx, sh, []string{"-c", "sleep 5"}); err == nil {
		t.Fatal("cancelled ctx: no error")
	}
}

// Without the FileChooser portal, the choosers fall back to a helper:
// when the bus is unreachable, when nothing owns the portal's name, and
// when the portal does not implement FileChooser.
func TestPortalFallback(t *testing.T) {
	check := func(name string) {
		t.Helper()
		f := &fakeCommands{installed: []string{"zenity"}, answer: commandResult{stdout: []byte("/tmp/out.csv\n")}}
		s := system{cmds: f.commands()}
		ps, err := s.OpenFile(testContext(t), FileOptions{Title: "Import"})
		if err != nil || !slices.Equal(ps, []string{"/tmp/out.csv"}) {
			t.Errorf("%s: OpenFile = %q, %v", name, ps, err)
		}
		p, err := s.SaveFile(testContext(t), SaveOptions{Name: "out.csv"})
		if err != nil || p != "/tmp/out.csv" {
			t.Errorf("%s: SaveFile = %q, %v", name, p, err)
		}
		if len(f.ran) != 2 || !slices.Contains(f.ran[1], "--save") {
			t.Errorf("%s: ran %q", name, f.ran)
		}
		f.answer = commandResult{code: helperCancelled}
		if _, err := s.OpenFile(testContext(t), FileOptions{}); !errors.Is(err, ErrCancelled) {
			t.Errorf("%s: cancelled helper: %v, want ErrCancelled", name, err)
		}
	}

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	check("no bus")
	testBus(t)
	check("no portal")
	startPortal(t, false) // serves OpenURI only
	check("no FileChooser")
}

// Only errors that say nothing serves the method fall back; a portal that
// fails is reported.
func TestMissing(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}, true},
		{&dbus.Error{Name: "org.freedesktop.DBus.Error.UnknownMethod"}, true},
		{dbus.Error{Name: "org.freedesktop.DBus.Error.Spawn.ChildExited"}, true},
		{dbus.Error{Name: "org.freedesktop.DBus.Error.Failed"}, false},
		{dbus.Error{Name: "org.freedesktop.DBus.Error.AccessDenied"}, false},
		{errors.New("org.freedesktop.DBus.Error.ServiceUnknown"), false},
	} {
		if got := missing(c.err); got != c.want {
			t.Errorf("missing(%#v) = %v, want %v", c.err, got, c.want)
		}
	}
}
