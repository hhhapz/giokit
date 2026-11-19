//go:build darwin && !ios && cgo

package desktop

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UniformTypeIdentifiers

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#define GIOKIT_PANEL_OK 0
#define GIOKIT_PANEL_CANCELLED 1
#define GIOKIT_PANEL_MAIN_THREAD 2

// giokitPanels maps a request number to its showing panel, or to NSNull
// when the request was cancelled before its panel was created. It is
// touched only on the main thread, whose queue orders creation and
// cancellation: whichever runs first decides.
static NSMutableDictionary<NSNumber *, id> *giokitPanels;

static NSMutableDictionary<NSNumber *, id> *giokitPanelState(void) {
	if (giokitPanels == nil) {
		giokitPanels = [NSMutableDictionary dictionary];
	}
	return giokitPanels;
}

static NSString *giokitString(const char *s) {
	if (s == NULL || s[0] == '\0') {
		return nil;
	}
	return [NSString stringWithUTF8String:s];
}

// giokitRunPanel shows an NSSavePanel (save != 0) or NSOpenPanel on the
// main queue and blocks the calling thread until the user answers. It must
// not run on the main thread, which has to be running the application's
// run loop: it returns GIOKIT_PANEL_MAIN_THREAD there instead of
// deadlocking. On GIOKIT_PANEL_OK, *paths is a malloc'd array of *n
// malloc'd UTF-8 paths that the caller frees.
static int giokitRunPanel(uintptr_t req, int save, const char *title, const char *name, const char *folder,
		const char **exts, int nexts, int multiple, int directory, char ***paths, int *n) {
	*paths = NULL;
	*n = 0;
	if ([NSThread isMainThread]) {
		return GIOKIT_PANEL_MAIN_THREAD;
	}
	__block int status = GIOKIT_PANEL_CANCELLED;
	__block char **out = NULL;
	__block int count = 0;
	@autoreleasepool {
		NSString *t = giokitString(title);
		NSString *nm = giokitString(name);
		NSString *dir = giokitString(folder);
		NSMutableArray<NSString *> *extensions = [NSMutableArray arrayWithCapacity:nexts];
		for (int i = 0; i < nexts; i++) {
			NSString *e = giokitString(exts[i]);
			if (e != nil) {
				[extensions addObject:e];
			}
		}
		dispatch_semaphore_t done = dispatch_semaphore_create(0);
		dispatch_async(dispatch_get_main_queue(), ^{
			if (giokitPanelState()[@(req)] == [NSNull null]) {
				// Cancelled before it was shown: never show it.
				[giokitPanels removeObjectForKey:@(req)];
				dispatch_semaphore_signal(done);
				return;
			}
			NSSavePanel *panel;
			if (save) {
				panel = [NSSavePanel savePanel];
			} else {
				NSOpenPanel *openPanel = [NSOpenPanel openPanel];
				openPanel.canChooseFiles = !directory;
				openPanel.canChooseDirectories = directory != 0;
				openPanel.allowsMultipleSelection = multiple != 0;
				panel = openPanel;
			}
			panel.canCreateDirectories = YES;
			if (t != nil) {
				// Panels have no title bar since macOS 10.11; the message
				// is what the user sees.
				panel.title = t;
				panel.message = t;
			}
			if (nm != nil) {
				panel.nameFieldStringValue = nm;
			}
			if (dir != nil) {
				panel.directoryURL = [NSURL fileURLWithPath:dir isDirectory:YES];
			}
			if (extensions.count > 0 && !directory) {
				if (@available(macOS 11.0, *)) {
					NSMutableArray<UTType *> *types = [NSMutableArray arrayWithCapacity:extensions.count];
					for (NSString *e in extensions) {
						UTType *type = [UTType typeWithFilenameExtension:e];
						if (type != nil) {
							[types addObject:type];
						}
					}
					panel.allowedContentTypes = types;
				}
			}
			giokitPanelState()[@(req)] = panel;
			[panel beginWithCompletionHandler:^(NSModalResponse response) {
				[giokitPanels removeObjectForKey:@(req)];
				if (response == NSModalResponseOK) {
					NSArray<NSURL *> *urls;
					if (save) {
						urls = panel.URL != nil ? @[panel.URL] : @[];
					} else {
						urls = ((NSOpenPanel *)panel).URLs;
					}
					out = calloc(urls.count + 1, sizeof(char *));
					for (NSURL *u in urls) {
						if (u.isFileURL) {
							out[count++] = strdup(u.fileSystemRepresentation);
						}
					}
					status = GIOKIT_PANEL_OK;
				}
				dispatch_semaphore_signal(done);
			}];
		});
		dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	}
	*paths = out;
	*n = count;
	return status;
}

// giokitCancelPanel cancels request req: a showing panel closes as if the
// user had pressed Cancel, and a panel not created yet is never shown. It
// may be called from any thread, at most once per request, and only while
// giokitRunPanel for req has not returned.
static void giokitCancelPanel(uintptr_t req) {
	dispatch_async(dispatch_get_main_queue(), ^{
		id p = giokitPanelState()[@(req)];
		if (p == nil) {
			giokitPanels[@(req)] = [NSNull null];
		} else if (p != [NSNull null]) {
			[(NSSavePanel *)p cancel:nil];
		}
	});
}

// giokitForgetPanel drops a cancellation marker giokitCancelPanel left for
// req, once giokitRunPanel for req has returned. Queued after the cancel,
// it runs after it.
static void giokitForgetPanel(uintptr_t req) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (giokitPanels[@(req)] == [NSNull null]) {
			[giokitPanels removeObjectForKey:@(req)];
		}
	});
}
*/
import "C"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"
)

// The panels are modeless (beginWithCompletionHandler, not runModal): the
// app keeps drawing while one is open. They have no parent window, because
// Platform takes none.
//
// Threading contract: the calls dispatch the panel to the main queue and
// block the calling goroutine until the user answers, so the main thread
// must be running the application's run loop (Gio's app.Main does) and the
// caller must not hold it up: call from a goroutine such as kit.Spawn's,
// never from the UI goroutine, whose frame the main thread may be waiting
// for. Called on the main thread itself they return an error at once
// instead of deadlocking.
//
// Cancellation: when ctx ends, the call returns ctx's error at once and
// asks the main queue to cancel the panel, which closes it as the Cancel
// button would; a panel not shown yet is never shown.

// panelSeq numbers panel requests, for giokitCancelPanel.
var panelSeq atomic.Uint64

// errMainThread is returned when a chooser is called on the main thread.
var errMainThread = errors.New("desktop: file panel called on the main thread; call it from another goroutine")

// panelRequest is one panel to show.
type panelRequest struct {
	save      bool
	title     string
	name      string
	folder    string
	exts      []string // allowed extensions; none allows every file
	multiple  bool
	directory bool
}

// OpenFile shows NSOpenPanel. Directory chooses folders instead of files;
// Filters are merged into one set of allowed extensions (see macExtensions).
func (system) OpenFile(ctx context.Context, o FileOptions) ([]string, error) {
	return runPanel(ctx, panelRequest{title: o.Title, exts: macExtensions(o.Filters), multiple: o.Multiple, directory: o.Directory})
}

// SaveFile shows NSSavePanel, which asks before replacing a file. Filters
// are merged into one set of allowed extensions; the first is added to a
// name typed without one.
func (system) SaveFile(ctx context.Context, o SaveOptions) (string, error) {
	ps, err := runPanel(ctx, panelRequest{save: true, title: o.Title, name: o.Name, folder: o.Folder, exts: macExtensions(o.Filters)})
	if err != nil {
		return "", err
	}
	return ps[0], nil
}

// panelResult is a finished panel's answer.
type panelResult struct {
	paths []string
	err   error
}

// runPanel shows req's panel and waits for its answer or for ctx to end.
func runPanel(ctx context.Context, req panelRequest) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id := panelSeq.Add(1)
	// mu orders the cancel before the worker's cleanup: the cancel is sent
	// only while run is outstanding, and the cleanup only after a cancel,
	// so a cancellation marker never outlives its request.
	var (
		mu                   sync.Mutex
		finished, cancelSent bool
	)
	done := make(chan panelResult, 1)
	go func() {
		r := req.run(id)
		mu.Lock()
		finished = true
		if cancelSent {
			C.giokitForgetPanel(C.uintptr_t(id))
		}
		mu.Unlock()
		done <- r
	}()
	select {
	case r := <-done:
		return r.paths, r.err
	case <-ctx.Done():
		mu.Lock()
		if !finished {
			cancelSent = true
			C.giokitCancelPanel(C.uintptr_t(id))
		}
		mu.Unlock()
		return nil, ctx.Err()
	}
}

// run shows the panel through giokitRunPanel, blocking until it closes.
func (req panelRequest) run(id uint64) panelResult {
	cstr := func(s string) *C.char {
		if s == "" {
			return nil
		}
		return C.CString(s)
	}
	title, name, folder := cstr(req.title), cstr(req.name), cstr(req.folder)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(name))
	defer C.free(unsafe.Pointer(folder))
	exts := make([]*C.char, len(req.exts)+1) // never empty, so &exts[0] is valid
	for i, e := range req.exts {
		exts[i] = C.CString(e)
		defer C.free(unsafe.Pointer(exts[i]))
	}
	var out **C.char
	var n C.int
	status := C.giokitRunPanel(C.uintptr_t(id), cbool(req.save), title, name, folder,
		&exts[0], C.int(len(req.exts)), cbool(req.multiple), cbool(req.directory), &out, &n)
	switch status {
	case C.GIOKIT_PANEL_MAIN_THREAD:
		return panelResult{err: errMainThread}
	case C.GIOKIT_PANEL_CANCELLED:
		return panelResult{err: ErrCancelled}
	}
	defer C.free(unsafe.Pointer(out))
	var ps []string
	for _, p := range unsafe.Slice(out, int(n)) {
		ps = append(ps, C.GoString(p))
		C.free(unsafe.Pointer(p))
	}
	if len(ps) == 0 {
		return panelResult{err: errors.New("desktop: file panel: no local files chosen")}
	}
	return panelResult{paths: ps}
}

func cbool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
