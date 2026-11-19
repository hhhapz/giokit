//go:build windows

package desktop

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file is the small slice of COM that the file dialogs need, called
// through vtables with syscall.SyscallN: no cgo.

var (
	ole32   = windows.NewLazySystemDLL("ole32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	user32  = windows.NewLazySystemDLL("user32.dll")

	procCoInitializeEx              = ole32.NewProc("CoInitializeEx")
	procCoUninitialize              = ole32.NewProc("CoUninitialize")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
	procEnumThreadWindows           = user32.NewProc("EnumThreadWindows")
	procPostMessageW                = user32.NewProc("PostMessageW")
)

var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xdc1c5a9c, Data2: 0xe88a, Data3: 0x4dde, Data4: [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	clsidFileSaveDialog = windows.GUID{Data1: 0xc0b4e2f3, Data2: 0xba21, Data3: 0x4773, Data4: [8]byte{0x8d, 0xba, 0x33, 0x5e, 0xc9, 0x46, 0xeb, 0x8b}}
	iidFileOpenDialog   = windows.GUID{Data1: 0xd57c7288, Data2: 0xd4ad, Data3: 0x4768, Data4: [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
	iidFileSaveDialog   = windows.GUID{Data1: 0x84bccd23, Data2: 0x5fde, Data3: 0x4cdb, Data4: [8]byte{0xae, 0xa4, 0xaf, 0x64, 0xb8, 0x3d, 0x78, 0xab}}
	iidShellItem        = windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
)

// hresult is a COM status code.
type hresult int32

const (
	sOK    hresult = 0
	sFalse hresult = 1
	// errCancelled is HRESULT_FROM_WIN32(ERROR_CANCELLED): the user
	// dismissed the dialog.
	errCancelled hresult = -0x7ff8fb39 // 0x800704C7
)

func (h hresult) failed() bool { return h < 0 }

func (h hresult) Error() string {
	return fmt.Sprintf("HRESULT 0x%08X: %v", uint32(h), syscall.Errno(uint32(h)&0xffff))
}

// Vtable slots, counted from IUnknown's QueryInterface at 0.
const (
	slotRelease = 2

	// IModalWindow
	slotShow = 3
	// IFileDialog
	slotSetFileTypes        = 4
	slotSetFileTypeIndex    = 5
	slotSetOptions          = 9
	slotGetOptions          = 10
	slotSetFolder           = 12
	slotSetFileName         = 15
	slotSetTitle            = 17
	slotGetResult           = 20
	slotSetDefaultExtension = 22
	// IFileOpenDialog
	slotGetResults = 27

	// IShellItem
	slotGetDisplayName = 5
	// IShellItemArray
	slotGetCount  = 7
	slotGetItemAt = 8
)

// FILEOPENDIALOGOPTIONS flags.
const (
	fosOverwritePrompt  = 0x2
	fosNoChangeDir      = 0x8
	fosPickFolders      = 0x20
	fosForceFileSystem  = 0x40
	fosAllowMultiSelect = 0x200
	fosPathMustExist    = 0x800
	fosFileMustExist    = 0x1000
)

// sigdnFileSysPath asks IShellItem::GetDisplayName for a file system path.
const sigdnFileSysPath = 0x80058000

// comObject is a COM interface pointer: its first word points at the
// vtable. Values live in COM's memory, never Go's.
type comObject struct {
	vtbl unsafe.Pointer
}

// call invokes vtable slot with o as the this pointer.
//
//go:uintptrescapes
func (o *comObject) call(slot int, args ...uintptr) hresult {
	fn := *(*uintptr)(unsafe.Add(o.vtbl, slot*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return hresult(int32(r))
}

// release drops o's reference. It accepts nil.
func (o *comObject) release() {
	if o != nil {
		o.call(slotRelease)
	}
}

// coCreate creates an in-process instance of clsid and returns its iid
// interface.
func coCreate(clsid, iid *windows.GUID) (*comObject, error) {
	const clsctxInprocServer = 0x1
	var o *comObject
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxInprocServer, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&o)))
	if h := hresult(int32(r)); h.failed() {
		return nil, h
	}
	return o, nil
}

// shellItem returns the IShellItem of a local path.
func shellItem(path string) (*comObject, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var o *comObject
	r, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&o)))
	if h := hresult(int32(r)); h.failed() {
		return nil, h
	}
	return o, nil
}

// itemPath returns the file system path of an IShellItem.
func itemPath(item *comObject) (string, error) {
	var s *uint16
	if h := item.call(slotGetDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&s))); h.failed() {
		return "", h
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(s))
	return windows.UTF16PtrToString(s), nil
}

// filterSpec is COMDLG_FILTERSPEC.
type filterSpec struct {
	name *uint16
	spec *uint16
}

// filterSpecs converts filters to COMDLG_FILTERSPECs. The strings are Go
// memory reachable from the result: keep it alive while the dialog may read
// them.
func filterSpecs(filters []Filter) ([]filterSpec, error) {
	var out []filterSpec
	for _, f := range winFilters(filters) {
		name, err := windows.UTF16PtrFromString(f.Name)
		if err != nil {
			return nil, err
		}
		spec, err := windows.UTF16PtrFromString(f.Spec)
		if err != nil {
			return nil, err
		}
		out = append(out, filterSpec{name: name, spec: spec})
	}
	return out, nil
}

// enumThreadWindows is the EnumThreadWindows callback shared by every
// closeDialogs call: syscall.NewCallback slots are never freed, so it is
// made once.
var enumThreadWindows = syscall.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
	var class [16]uint16
	n, _ := windows.GetClassName(hwnd, &class[0], int32(len(class)))
	if windows.UTF16ToString(class[:n]) == "#32770" { // the dialog box class
		const wmClose = 0x0010
		procPostMessageW.Call(uintptr(hwnd), wmClose, 0, 0)
	}
	return 1 // continue
})

// closeDialogs posts WM_CLOSE, which a dialog treats as Cancel, to every
// top-level dialog window of thread tid. PostMessage may be called from any
// thread.
func closeDialogs(tid uint32) {
	procEnumThreadWindows.Call(uintptr(tid), enumThreadWindows, 0)
}
