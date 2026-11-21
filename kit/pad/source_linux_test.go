//go:build linux && !android

package pad

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// uinputPad is a virtual gamepad made through /dev/uinput, shaped like the
// xpad driver's Xbox 360 pad.
type uinputPad struct{ f *os.File }

const (
	uiSetEvBit   = 1<<30 | 4<<16 | 'U'<<8 | 100
	uiSetKeyBit  = 1<<30 | 4<<16 | 'U'<<8 | 101
	uiSetAbsBit  = 1<<30 | 4<<16 | 'U'<<8 | 103
	uiDevCreate  = 'U'<<8 | 1
	uiDevDestroy = 'U'<<8 | 2
)

func newUinputPad(t *testing.T, name string, vendor, product uint16) *uinputPad {
	t.Helper()
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("no uinput: %v", err)
	}
	fd := f.Fd()
	ioctl := func(req, arg uintptr) {
		if _, _, e := unix.Syscall(unix.SYS_IOCTL, fd, req, arg); e != 0 {
			t.Fatalf("uinput ioctl %#x %d: %v", req, arg, e)
		}
	}
	ioctl(uiSetEvBit, evKey)
	for _, k := range []uintptr{btnSouth, btnEast, btnNorth, btnWest, btnTL, btnTR, btnSelect, btnStart, btnMode, btnThumbL, btnThumbR} {
		ioctl(uiSetKeyBit, k)
	}
	ioctl(uiSetEvBit, evAbs)
	abs := []uintptr{absX, absY, absZ, absRX, absRY, absRZ, absHat0X, absHat0Y}
	for _, a := range abs {
		ioctl(uiSetAbsBit, a)
	}
	// struct uinput_user_dev: name[80], input_id, ff_effects_max, then
	// absmax, absmin, absfuzz, absflat [64]int32.
	dev := make([]byte, 80+8+4+4*64*4)
	copy(dev, name)
	binary.NativeEndian.PutUint16(dev[80:], unix.BUS_USB)
	binary.NativeEndian.PutUint16(dev[82:], vendor)
	binary.NativeEndian.PutUint16(dev[84:], product)
	put := func(table, axis int, v int32) {
		binary.NativeEndian.PutUint32(dev[92+table*256+axis*4:], uint32(v))
	}
	for _, a := range abs {
		lo, hi := int32(-32768), int32(32767)
		switch a {
		case absZ, absRZ:
			lo, hi = 0, 255
		case absHat0X, absHat0Y:
			lo, hi = -1, 1
		}
		put(0, int(a), hi)
		put(1, int(a), lo)
	}
	if _, err := f.Write(dev); err != nil {
		t.Fatal(err)
	}
	ioctl(uiDevCreate, 0)
	p := &uinputPad{f}
	t.Cleanup(func() {
		unix.Syscall(unix.SYS_IOCTL, fd, uiDevDestroy, 0)
		f.Close()
	})
	return p
}

func (p *uinputPad) emit(t *testing.T, typ, code uint16, v int32) {
	t.Helper()
	for _, ev := range []inputEvent{{Type: typ, Code: code, Value: v}, {Type: evSyn}} {
		b := (*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:]
		if _, err := p.f.Write(b); err != nil {
			t.Fatal(err)
		}
	}
}

// waitPad polls src until want reports true for a pad, or times out.
func waitPad(t *testing.T, src Source, what string, want func(Pad) bool) Pad {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, p := range src.Poll(nil) {
			if want(p) {
				return p
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; pads: %+v", what, src.Poll(nil))
	return Pad{}
}

// TestEvdev plugs in a virtual Xbox 360 pad and checks the source finds it
// through hot-plug, maps xpad's BTN_X to West, the hat to the d-pad and the
// axes to their ranges, and drops the pad when it is unplugged.
func TestEvdev(t *testing.T) {
	const name = "giokit test X-Box 360 pad"
	src := platformSource()
	woke := make(chan struct{}, 100)
	if err := src.Start(func() {
		select {
		case woke <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	u := newUinputPad(t, name, 0x045e, 0x028e)
	ours := func(p Pad) bool { return p.Name == name }
	p := waitPad(t, src, "the pad to appear", ours)
	if p.Family != Xbox {
		t.Errorf("family %d, want Xbox", p.Family)
	}

	u.emit(t, evKey, btnNorth, 1) // xpad's BTN_X: the left face button
	u.emit(t, evAbs, absHat0Y, -1)
	u.emit(t, evAbs, absX, 32767)
	u.emit(t, evAbs, absRZ, 255)
	p = waitPad(t, src, "the presses", func(p Pad) bool {
		return ours(p) && p.Buttons.Has(West) && p.Buttons.Has(Up) && p.Axes[LeftX] > 0.99 && p.Axes[RightTriggerAxis] > 0.99
	})
	if p.Buttons.Has(North) || p.Buttons.Has(Down) {
		t.Errorf("buttons %b: North or Down held too", p.Buttons)
	}
	u.emit(t, evKey, btnNorth, 0)
	u.emit(t, evAbs, absHat0Y, 0)
	waitPad(t, src, "the releases", func(p Pad) bool { return ours(p) && p.Buttons == 0 })
	if len(woke) == 0 {
		t.Error("never woke the app")
	}

	u.f.Close()
	unix.Syscall(unix.SYS_IOCTL, u.f.Fd(), uiDevDestroy, 0)
	deadline := time.Now().Add(3 * time.Second)
	for {
		gone := true
		for _, p := range src.Poll(nil) {
			gone = gone && !ours(p)
		}
		if gone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pad still there after unplugging")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
