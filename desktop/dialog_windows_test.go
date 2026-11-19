//go:build windows

package desktop

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// lockedCOM runs comThread on a new locked OS thread, as runDialog does.
func lockedCOM(name string, show func() ([]string, error)) dialogResult {
	done := make(chan dialogResult)
	go func() {
		runtime.LockOSThread()
		done <- comThread(name, show)
	}()
	return <-done
}

// The dialogs' COM objects are created and configured with every option,
// then released without being shown.
func TestDialogConfigure(t *testing.T) {
	filters := []Filter{{Name: "Tables", Patterns: []string{"*.csv", "*.tsv"}}, {Name: "All"}}
	for name, show := range map[string]func() ([]string, error){
		"open": func() ([]string, error) {
			return showOpen(FileOptions{Title: "Import", Filters: filters, Multiple: true}, true)
		},
		"directory": func() ([]string, error) {
			return showOpen(FileOptions{Title: "Folder", Filters: filters, Directory: true}, true)
		},
		"save": func() ([]string, error) {
			return showSave(SaveOptions{Title: "Export", Name: "out.csv", Folder: os.TempDir(), Filters: filters}, true)
		},
	} {
		r := lockedCOM(name, show)
		if r.err != nil {
			t.Errorf("%s: %v", name, r.err)
		}
	}
	r := lockedCOM("save", func() ([]string, error) {
		return showSave(SaveOptions{Folder: `C:\no\such\folder`}, true)
	})
	if r.err == nil {
		t.Error("missing folder: no error")
	}
}

// A dialog that is showing closes when ctx ends. It needs a desktop, so it
// runs only when DESKTOP_SHOW_DIALOG is set (under wine, for example).
func TestDialogCancel(t *testing.T) {
	if os.Getenv("DESKTOP_SHOW_DIALOG") == "" {
		t.Skip("set DESKTOP_SHOW_DIALOG=1 to show a dialog")
	}
	for name, call := range map[string]func(context.Context) error{
		"open": func(ctx context.Context) error {
			_, err := system{}.OpenFile(ctx, FileOptions{Title: "giokit cancel test", Filters: []Filter{{Name: "Text", Patterns: []string{"*.txt"}}}})
			return err
		},
		"save": func(ctx context.Context) error {
			_, err := system{}.SaveFile(ctx, SaveOptions{Title: "giokit cancel test", Name: "x.txt"})
			return err
		},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		start := time.Now()
		err := call(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%s: %v, want DeadlineExceeded", name, err)
		}
		t.Logf("%s: returned after %v", name, time.Since(start).Round(time.Millisecond))
	}

	// WM_CLOSE dismisses the dialog the way the user cancelling does.
	tid := make(chan uint32, 1)
	done := make(chan dialogResult, 1)
	go func() {
		runtime.LockOSThread()
		tid <- windows.GetCurrentThreadId()
		done <- comThread("open", func() ([]string, error) { return showOpen(FileOptions{Title: "giokit close test"}, false) })
	}()
	thread := <-tid
	deadline := time.After(10 * time.Second)
	for {
		select {
		case r := <-done:
			if !errors.Is(r.err, ErrCancelled) {
				t.Fatalf("closed dialog: %v, want ErrCancelled", r.err)
			}
			return
		case <-deadline:
			t.Fatal("dialog still open 10s after WM_CLOSE")
		case <-time.After(200 * time.Millisecond):
			closeDialogs(thread)
		}
	}
}
