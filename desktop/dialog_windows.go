//go:build windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// system is Windows: the Common Item Dialog (IFileOpenDialog,
// IFileSaveDialog) through COM, and ShellExecute for links.
//
// Each chooser runs on its own locked OS thread in a single-threaded COM
// apartment, which the dialog's modal message loop then owns, so a chooser
// never touches the UI goroutine's thread. The dialog has no owner window:
// Platform takes no window, so it is not modal to the app's windows and
// the app keeps drawing while it is open.
//
// Cancellation: when ctx ends, the call returns ctx's error at once and
// posts WM_CLOSE (the same as Esc) to the dialog's windows until the dialog
// has closed. A nested prompt that has no Cancel button, such as the save
// dialog's "replace the file?" question, ignores WM_CLOSE and stays until
// the user answers it; whatever they choose is then discarded.
type system struct{}

// OpenFile shows IFileOpenDialog. Directory sets FOS_PICKFOLDERS (Filters
// do not apply to folders); Multiple sets FOS_ALLOWMULTISELECT.
func (system) OpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	return runDialog(ctx, "IFileOpenDialog", func() ([]string, error) { return showOpen(o, false) })
}

// SaveFile shows IFileSaveDialog, which asks before replacing a file. The
// first filter's first "*.ext" pattern is the extension added to a name
// typed without one.
func (system) SaveFile(ctx context.Context, o SaveOptions) (string, error) {
	ps, err := runDialog(ctx, "IFileSaveDialog", func() ([]string, error) { return showSave(o, false) })
	if err != nil {
		return "", err
	}
	return ps[0], nil
}

// dialogResult is what a dialog thread hands back.
type dialogResult struct {
	paths []string
	err   error
}

// runDialog runs show on a new OS thread inside an apartment-threaded COM
// apartment, and waits for its answer or for ctx to end (see system).
func runDialog(ctx context.Context, name string, show func() ([]string, error)) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tid := make(chan uint32, 1)
	done := make(chan dialogResult, 1)
	go func() {
		// The goroutine never unlocks: the thread exits with it, taking
		// whatever COM state was left on it.
		runtime.LockOSThread()
		tid <- windows.GetCurrentThreadId()
		done <- comThread(name, show)
	}()
	thread := <-tid
	select {
	case r := <-done:
		return r.paths, r.err
	case <-ctx.Done():
		go func() {
			tick := time.NewTicker(50 * time.Millisecond)
			defer tick.Stop()
			for {
				closeDialogs(thread)
				select {
				case <-done:
					return
				case <-tick.C:
				}
			}
		}()
		return nil, ctx.Err()
	}
}

// comThread initialises COM on the current, locked thread, runs show, and
// maps a user cancel to ErrCancelled.
func comThread(name string, show func() ([]string, error)) dialogResult {
	const coinit = windows.COINIT_APARTMENTTHREADED | windows.COINIT_DISABLE_OLE1DDE
	r, _, _ := procCoInitializeEx.Call(0, coinit)
	if h := hresult(int32(r)); h.failed() {
		return dialogResult{err: fmt.Errorf("desktop: %s: CoInitializeEx: %w", name, h)}
	}
	defer procCoUninitialize.Call()
	ps, err := show()
	if h := hresult(0); errors.As(err, &h) && h == errCancelled {
		return dialogResult{err: ErrCancelled}
	}
	if err != nil {
		return dialogResult{err: fmt.Errorf("desktop: %s: %w", name, err)}
	}
	return dialogResult{paths: ps}
}

// dialogSetup is the configuration shared by both dialogs.
type dialogSetup struct {
	title   string
	name    string
	folder  string
	ext     string
	filters []Filter
	options uint32 // FOS_ flags added to the dialog's defaults
}

// configure applies s to the IFileDialog d. The returned value holds the
// Go memory the dialog was given; keep it alive until the dialog is
// released.
func (s dialogSetup) configure(d *comObject) (keep any, err error) {
	var opts uint32
	if h := d.call(slotGetOptions, uintptr(unsafe.Pointer(&opts))); h.failed() {
		return nil, fmt.Errorf("GetOptions: %w", h)
	}
	if h := d.call(slotSetOptions, uintptr(opts|s.options)); h.failed() {
		return nil, fmt.Errorf("SetOptions: %w", h)
	}
	var strs []*uint16
	setString := func(slot int, method, v string) error {
		if v == "" {
			return nil
		}
		p, err := windows.UTF16PtrFromString(v)
		if err != nil {
			return fmt.Errorf("%s: %w", method, err)
		}
		strs = append(strs, p)
		if h := d.call(slot, uintptr(unsafe.Pointer(p))); h.failed() {
			return fmt.Errorf("%s: %w", method, h)
		}
		return nil
	}
	if err := setString(slotSetTitle, "SetTitle", s.title); err != nil {
		return nil, err
	}
	if err := setString(slotSetFileName, "SetFileName", s.name); err != nil {
		return nil, err
	}
	if err := setString(slotSetDefaultExtension, "SetDefaultExtension", s.ext); err != nil {
		return nil, err
	}
	var specs []filterSpec
	if len(s.filters) > 0 && s.options&fosPickFolders == 0 {
		if specs, err = filterSpecs(s.filters); err != nil {
			return nil, fmt.Errorf("SetFileTypes: %w", err)
		}
		if h := d.call(slotSetFileTypes, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0]))); h.failed() {
			return nil, fmt.Errorf("SetFileTypes: %w", h)
		}
		if h := d.call(slotSetFileTypeIndex, 1); h.failed() { // 1-based
			return nil, fmt.Errorf("SetFileTypeIndex: %w", h)
		}
	}
	if s.folder != "" {
		item, err := shellItem(s.folder)
		if err != nil {
			return nil, fmt.Errorf("folder %q: %w", s.folder, err)
		}
		h := d.call(slotSetFolder, uintptr(unsafe.Pointer(item)))
		item.release()
		if h.failed() {
			return nil, fmt.Errorf("SetFolder: %w", h)
		}
	}
	return []any{strs, specs}, nil
}

// openSetup is the IFileOpenDialog configuration for o.
func openSetup(o FileOptions) dialogSetup {
	s := dialogSetup{title: o.Title, filters: o.Filters, options: fosForceFileSystem | fosNoChangeDir | fosPathMustExist | fosFileMustExist}
	if o.Directory {
		s.options |= fosPickFolders
	}
	if o.Multiple {
		s.options |= fosAllowMultiSelect
	}
	return s
}

// saveSetup is the IFileSaveDialog configuration for o.
func saveSetup(o SaveOptions) dialogSetup {
	return dialogSetup{
		title: o.Title, name: o.Name, folder: o.Folder, filters: o.Filters, ext: defaultExt(o.Filters),
		options: fosForceFileSystem | fosNoChangeDir | fosPathMustExist | fosOverwritePrompt,
	}
}

// showOpen creates, configures and (unless dryRun) shows IFileOpenDialog.
// It must run on a thread in an apartment-threaded COM apartment.
func showOpen(o FileOptions, dryRun bool) ([]string, error) {
	d, err := coCreate(&clsidFileOpenDialog, &iidFileOpenDialog)
	if err != nil {
		return nil, fmt.Errorf("CoCreateInstance: %w", err)
	}
	defer d.release()
	keep, err := openSetup(o).configure(d)
	defer runtime.KeepAlive(keep)
	if err != nil || dryRun {
		return nil, err
	}
	if h := d.call(slotShow, 0); h.failed() {
		return nil, h
	}
	var items *comObject
	if h := d.call(slotGetResults, uintptr(unsafe.Pointer(&items))); h.failed() {
		return nil, fmt.Errorf("GetResults: %w", h)
	}
	defer items.release()
	var n uint32
	if h := items.call(slotGetCount, uintptr(unsafe.Pointer(&n))); h.failed() {
		return nil, fmt.Errorf("GetCount: %w", h)
	}
	ps := make([]string, 0, n)
	for i := range n {
		var item *comObject
		if h := items.call(slotGetItemAt, uintptr(i), uintptr(unsafe.Pointer(&item))); h.failed() {
			return nil, fmt.Errorf("GetItemAt: %w", h)
		}
		p, err := itemPath(item)
		item.release()
		if err != nil {
			return nil, fmt.Errorf("GetDisplayName: %w", err)
		}
		ps = append(ps, p)
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("GetResults: no items")
	}
	return ps, nil
}

// showSave creates, configures and (unless dryRun) shows IFileSaveDialog.
// It must run on a thread in an apartment-threaded COM apartment.
func showSave(o SaveOptions, dryRun bool) ([]string, error) {
	d, err := coCreate(&clsidFileSaveDialog, &iidFileSaveDialog)
	if err != nil {
		return nil, fmt.Errorf("CoCreateInstance: %w", err)
	}
	defer d.release()
	keep, err := saveSetup(o).configure(d)
	defer runtime.KeepAlive(keep)
	if err != nil || dryRun {
		return nil, err
	}
	if h := d.call(slotShow, 0); h.failed() {
		return nil, h
	}
	var item *comObject
	if h := d.call(slotGetResult, uintptr(unsafe.Pointer(&item))); h.failed() {
		return nil, fmt.Errorf("GetResult: %w", h)
	}
	defer item.release()
	p, err := itemPath(item)
	if err != nil {
		return nil, fmt.Errorf("GetDisplayName: %w", err)
	}
	return []string{p}, nil
}
