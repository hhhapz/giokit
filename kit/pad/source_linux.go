//go:build linux && !android

package pad

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The Linux source reads evdev devices (/dev/input/event*) that have a
// gamepad's south button, one goroutine each, and watches /dev/input with
// inotify for pads plugged in later. The desktop session usually grants
// the logged-in user access to joysticks (udev's uaccess tag); without it,
// pads are simply not seen.

func platformSource() Source { return &evdevSource{dir: "/dev/input"} }

func debugf(format string, args ...any) {
	if Debug {
		log.Printf("pad: "+format, args...)
	}
}

type evdevSource struct {
	dir  string
	mu   sync.Mutex
	devs map[string]*evdev // by path
	wake func()
	next int
	ino  *os.File // inotify, nil when none
	done chan struct{}
}

func (s *evdevSource) Start(wake func()) error {
	s.wake, s.devs, s.done = wake, map[string]*evdev{}, make(chan struct{})
	// Non-blocking, so the runtime poller owns it and Close interrupts the
	// watcher's Read: closing a descriptor doesn't wake a read(2) blocked
	// on it, and a new inotify could reuse the number.
	if fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK); err == nil {
		if _, err := unix.InotifyAddWatch(fd, s.dir, unix.IN_CREATE|unix.IN_ATTRIB); err == nil {
			s.ino = os.NewFile(uintptr(fd), "inotify")
			go s.watch(s.ino)
		} else {
			unix.Close(fd)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(s.dir, "event*"))
	for _, p := range paths {
		s.open(p)
	}
	return nil
}

func (s *evdevSource) Poll(dst []Pad) []Pad {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devs {
		d.mu.Lock()
		dst = append(dst, d.pad)
		d.mu.Unlock()
	}
	return dst
}

func (s *evdevSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.done)
	if s.ino != nil {
		s.ino.Close() // ends watch
	}
	for p, d := range s.devs {
		d.f.Close() // ends read
		delete(s.devs, p)
	}
	return nil
}

// watch opens event devices as they appear. udev sets their permissions
// just after creating them, so a create that can't be opened yet is
// retried on the attribute change that follows.
func (s *evdevSource) watch(f *os.File) {
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if err != nil || n <= 0 {
			return
		}
		for off := 0; off+unix.SizeofInotifyEvent <= n; {
			ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off]))
			name := buf[off+unix.SizeofInotifyEvent : off+unix.SizeofInotifyEvent+int(ev.Len)]
			off += unix.SizeofInotifyEvent + int(ev.Len)
			if n := string(bytes.TrimRight(name, "\x00")); strings.HasPrefix(n, "event") {
				s.open(filepath.Join(s.dir, n))
			}
		}
	}
}

// open starts reading path if it is a gamepad not already open.
func (s *evdevSource) open(path string) {
	s.mu.Lock()
	if _, ok := s.devs[path]; ok {
		s.mu.Unlock()
		return
	}
	select {
	case <-s.done:
		s.mu.Unlock()
		return
	default:
	}
	s.mu.Unlock()

	if !sysfsGamepad(filepath.Base(path)) {
		return // keyboards, mice and the like, which the user may not even be allowed to open
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		debugf("%s: %v", path, err)
		return
	}
	d, ok := probe(fd)
	if !ok {
		debugf("%s: not a gamepad (no south button)", path)
		unix.Close(fd)
		return
	}
	debugf("%s: %q, family %d", path, d.pad.Name, d.pad.Family)
	d.f = os.NewFile(uintptr(fd), path) // non-blocking: Close interrupts Read

	s.mu.Lock()
	if _, dup := s.devs[path]; dup {
		s.mu.Unlock()
		d.f.Close()
		return
	}
	s.next++
	d.pad.ID = s.next
	s.devs[path] = d
	s.mu.Unlock()
	go s.read(path, d)
	s.wake()
}

func (s *evdevSource) read(path string, d *evdev) {
	defer func() {
		s.mu.Lock()
		if s.devs[path] == d {
			delete(s.devs, path)
			d.f.Close()
		}
		s.mu.Unlock()
		debugf("%s: gone", path)
		s.wake()
	}()
	var ev inputEvent
	buf := make([]byte, 64*unsafe.Sizeof(ev))
	for {
		n, err := d.f.Read(buf)
		if err != nil {
			return // unplugged (ENODEV) or closed
		}
		changed := false
		d.mu.Lock()
		for off := 0; off+int(unsafe.Sizeof(ev)) <= n; off += int(unsafe.Sizeof(ev)) {
			ev = *(*inputEvent)(unsafe.Pointer(&buf[off]))
			if Debug && ev.Type != evSyn {
				debugf("%s: type %#x code %#x value %d", path, ev.Type, ev.Code, ev.Value)
			}
			if ev.Type == evSyn && ev.Code == synDropped {
				d.sync()
			}
			changed = d.apply(ev) || changed
		}
		d.mu.Unlock()
		if changed {
			s.wake()
		}
	}
}

// evdev is one open gamepad.
type evdev struct {
	f      *os.File
	fd     int
	mu     sync.Mutex
	pad    Pad
	keys   map[uint16]Button
	axes   map[uint16]axisMap
	absMin map[uint16]int32
	absMax map[uint16]int32
}

// axisMap says where an evdev axis goes: a standard axis, or (hat) a pair
// of d-pad buttons.
type axisMap struct {
	axis     Axis
	trigger  bool // 0..1 rather than -1..1
	hat      bool
	neg, pos Button
}

// Linux input constants (linux/input-event-codes.h).
const (
	evSyn      = 0x00
	evKey      = 0x01
	evAbs      = 0x03
	synDropped = 3

	btnSouth  = 0x130
	btnEast   = 0x131
	btnNorth  = 0x133 // also BTN_X
	btnWest   = 0x134 // also BTN_Y
	btnTL     = 0x136
	btnTR     = 0x137
	btnTL2    = 0x138
	btnTR2    = 0x139
	btnSelect = 0x13a
	btnStart  = 0x13b
	btnMode   = 0x13c
	btnThumbL = 0x13d
	btnThumbR = 0x13e
	btnDUp    = 0x220
	btnDDown  = 0x221
	btnDLeft  = 0x222
	btnDRight = 0x223
	keyMax    = 0x2ff

	absX     = 0x00
	absY     = 0x01
	absZ     = 0x02
	absRX    = 0x03
	absRY    = 0x04
	absRZ    = 0x05
	absGas   = 0x09
	absBrake = 0x0a
	absHat0X = 0x10
	absHat0Y = 0x11
	absMax   = 0x3f
)

// inputEvent is struct input_event; unix.Timeval sizes it per architecture.
type inputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

func ioc(dir, nr, size uintptr) uintptr { return dir<<30 | size<<16 | 'E'<<8 | nr }

const iocRead = 2

func ioctlBuf(fd int, req uintptr, buf []byte) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

func bit(bits []byte, n int) bool { return n/8 < len(bits) && bits[n/8]&(1<<(n%8)) != 0 }

// probe reads fd's capabilities and returns it as a pad if it has a
// gamepad's south button.
func probe(fd int) (*evdev, bool) {
	keys := make([]byte, keyMax/8+1)
	if ioctlBuf(fd, ioc(iocRead, 0x20+evKey, uintptr(len(keys))), keys) != nil || !bit(keys, btnSouth) {
		return nil, false
	}
	abs := make([]byte, absMax/8+1)
	_ = ioctlBuf(fd, ioc(iocRead, 0x20+evAbs, uintptr(len(abs))), abs)
	name := make([]byte, 256)
	_ = ioctlBuf(fd, ioc(iocRead, 0x06, uintptr(len(name))), name)
	id := make([]byte, 8)
	_ = ioctlBuf(fd, ioc(iocRead, 0x02, uintptr(len(id))), id)
	vendor, product := binary.NativeEndian.Uint16(id[2:]), binary.NativeEndian.Uint16(id[4:])

	d := &evdev{fd: fd, keys: map[uint16]Button{}, axes: map[uint16]axisMap{}, absMin: map[uint16]int32{}, absMax: map[uint16]int32{}}
	d.pad.Name = string(bytes.TrimRight(name, "\x00"))
	d.pad.Family = familyOf(vendor, product, d.pad.Name)

	// Xbox drivers (xpad and kin, and Steam Input's virtual pad) report the
	// left face button as BTN_X, whose code is BTN_NORTH, and the top one
	// as BTN_Y (BTN_WEST). Drivers that follow the kernel's gamepad spec
	// (hid-playstation, hid-nintendo) mean the codes by position.
	north, west := Button(North), Button(West)
	if d.pad.Family == Xbox || d.pad.Family == Steam {
		north, west = West, North
	}
	for code, b := range map[uint16]Button{
		btnSouth: South, btnEast: East, btnNorth: north, btnWest: west,
		btnTL: LeftShoulder, btnTR: RightShoulder, btnTL2: LeftTrigger, btnTR2: RightTrigger,
		btnSelect: Back, btnStart: Start, btnMode: Guide, btnThumbL: LeftStick, btnThumbR: RightStick,
		btnDUp: Up, btnDDown: Down, btnDLeft: Left, btnDRight: Right,
	} {
		if bit(keys, int(code)) {
			d.keys[code] = b
		}
	}

	has := func(c int) bool { return bit(abs, c) }
	d.axes[absX] = axisMap{axis: LeftX}
	d.axes[absY] = axisMap{axis: LeftY}
	switch {
	case has(absRX) && has(absRY): // xpad, hid-playstation: triggers on Z/RZ
		d.axes[absRX] = axisMap{axis: RightX}
		d.axes[absRY] = axisMap{axis: RightY}
		d.axes[absZ] = axisMap{axis: LeftTriggerAxis, trigger: true}
		d.axes[absRZ] = axisMap{axis: RightTriggerAxis, trigger: true}
	case has(absZ) && has(absRZ): // many generic HID pads: right stick on Z/RZ
		d.axes[absZ] = axisMap{axis: RightX}
		d.axes[absRZ] = axisMap{axis: RightY}
	}
	if has(absBrake) && has(absGas) {
		d.axes[absBrake] = axisMap{axis: LeftTriggerAxis, trigger: true}
		d.axes[absGas] = axisMap{axis: RightTriggerAxis, trigger: true}
	}
	d.axes[absHat0X] = axisMap{hat: true, neg: Left, pos: Right}
	d.axes[absHat0Y] = axisMap{hat: true, neg: Up, pos: Down}
	for code := range d.axes {
		if !has(int(code)) {
			delete(d.axes, code)
			continue
		}
		info := make([]byte, 24) // struct input_absinfo
		if ioctlBuf(fd, ioc(iocRead, 0x40+uintptr(code), 24), info) == nil {
			d.absMin[code] = int32(binary.NativeEndian.Uint32(info[4:]))
			d.absMax[code] = int32(binary.NativeEndian.Uint32(info[8:]))
		}
	}
	d.sync()
	return d, true
}

// sync reads the current key and axis state, at open and after the kernel
// dropped events.
func (d *evdev) sync() {
	keys := make([]byte, keyMax/8+1)
	if ioctlBuf(d.fd, ioc(iocRead, 0x18, uintptr(len(keys))), keys) == nil {
		for code := range d.keys {
			var v int32
			if bit(keys, int(code)) {
				v = 1
			}
			d.apply(inputEvent{Type: evKey, Code: code, Value: v})
		}
	}
	for code := range d.axes {
		info := make([]byte, 24)
		if ioctlBuf(d.fd, ioc(iocRead, 0x40+uintptr(code), 24), info) == nil {
			d.apply(inputEvent{Type: evAbs, Code: code, Value: int32(binary.NativeEndian.Uint32(info[0:]))})
		}
	}
}

// apply updates the pad from one event, reporting whether it changed.
func (d *evdev) apply(ev inputEvent) bool {
	old := d.pad
	switch ev.Type {
	case evKey:
		if b, ok := d.keys[ev.Code]; ok {
			d.pad.Buttons = setButton(d.pad.Buttons, b, ev.Value != 0)
		}
	case evAbs:
		m, ok := d.axes[ev.Code]
		if !ok {
			return false
		}
		if m.hat {
			d.pad.Buttons = setButton(d.pad.Buttons, m.neg, ev.Value < 0)
			d.pad.Buttons = setButton(d.pad.Buttons, m.pos, ev.Value > 0)
			break
		}
		lo, hi := d.absMin[ev.Code], d.absMax[ev.Code]
		if hi <= lo {
			break
		}
		d.pad.Axes[m.axis] = steadyAxis(float32(ev.Value-lo)/float32(hi-lo), m.trigger)
	}
	return d.pad != old
}

func setButton(s Buttons, b Button, down bool) Buttons {
	if down {
		return s | 1<<b
	}
	return s &^ (1 << b)
}

// sysfsGamepad reports whether input device name (event7) has a gamepad's
// south button, from its capabilities in sysfs, which anyone may read. It
// says yes when sysfs can't tell, leaving probe to decide.
func sysfsGamepad(name string) bool {
	b, err := os.ReadFile("/sys/class/input/" + name + "/device/capabilities/key")
	if err != nil {
		return true
	}
	// Space-separated hex words, the highest bits first.
	words := strings.Fields(string(b))
	bitsPerWord := 8 * int(unsafe.Sizeof(uintptr(0)))
	i := len(words) - 1 - btnSouth/bitsPerWord
	if i < 0 {
		return false
	}
	w, err := strconv.ParseUint(words[i], 16, 64)
	return err != nil || w&(1<<(btnSouth%bitsPerWord)) != 0
}
