// Package pad reads game controllers on every platform giokit runs on and
// turns them into per-frame button events.
//
// Every controller is presented in one standard layout, the one Xbox,
// PlayStation, Nintendo and Steam pads share: four face buttons named by
// where they sit (South, East, West, North), shoulders, analog triggers,
// two sticks, a d-pad, and Back/Start/Guide. A game binds positions, not
// letters, so South is "A" on an Xbox pad, "✕" on a PlayStation pad and
// "B" on a Nintendo pad (Pads.NintendoConfirm instead makes South the A
// button there, so it confirms on every pad); Label names a button the way the player's pad
// prints it.
//
// Sources:
//   - Linux: evdev (/dev/input/event*), with hot-plug. The kernel drivers
//     for Xbox, PlayStation and Switch pads already report this layout.
//   - Windows: XInput (Xbox pads, and anything Steam Input presents as one).
//   - macOS: the GameController framework (needs cgo).
//   - Web: the Gamepad API, with the "standard" mapping.
//
// Steam Input needs nothing extra: a game launched through Steam sees any
// controller as a virtual Xbox pad, which every source reads.
//
// Use one Pads per window. Call Frame at the top of every frame, then read
// Events or run a Map:
//
//	pads := pad.New(app) // once; app wakes the UI when a pad changes
//
//	func (v *view) Frame(gtx layout.Context) {
//		v.pads.Frame(gtx)
//		v.padKeys.Run(gtx, v.pads)
//		...
//	}
package pad

import (
	"os"
	"time"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op"
)

// Button is a control in the standard layout.
type Button uint8

const (
	South Button = iota // bottom face button: A, ✕, B (Nintendo)
	East                // right face button: B, ○, A (Nintendo)
	West                // left face button: X, □, Y (Nintendo)
	North               // top face button: Y, △, X (Nintendo)
	LeftShoulder
	RightShoulder
	LeftTrigger // also an axis; pressed past half travel
	RightTrigger
	Back  // View, Share/Create, −
	Start // Menu, Options, +
	LeftStick
	RightStick
	Up // d-pad; the left stick also presses these (see Pads.StickAsDPad)
	Down
	Left
	Right
	Guide // Xbox, PS, Home; often taken by the system
	ButtonCount
)

// Buttons is a set of buttons.
type Buttons uint32

// Has reports whether b is in the set.
func (s Buttons) Has(b Button) bool { return s&(1<<b) != 0 }

// With returns the set with b added.
func (s Buttons) With(b Button) Buttons { return s | 1<<b }

// Axis is an analog control.
type Axis uint8

const (
	LeftX            Axis = iota // -1 left .. 1 right
	LeftY                        // -1 up .. 1 down
	RightX                       // -1 left .. 1 right
	RightY                       // -1 up .. 1 down
	LeftTriggerAxis              // 0 released .. 1 fully pressed
	RightTriggerAxis             // 0 released .. 1 fully pressed
	AxisCount
)

// Family is the kind of controller, for labels and glyphs.
type Family uint8

const (
	Generic Family = iota // unknown; labelled like an Xbox pad
	Xbox
	PlayStation
	Nintendo
	Steam
)

// Pad is one connected controller's state.
type Pad struct {
	// ID identifies the pad while it stays connected; a reconnected pad
	// may get a new one.
	ID      int
	Name    string
	Family  Family
	Buttons Buttons
	Axes    [AxisCount]float32
}

// EventKind says what happened to a button.
type EventKind uint8

const (
	Press   EventKind = iota // the button went down
	Release                  // the button came up
	Repeat                   // the button is still held: auto-repeat, like a held key
)

// Event is a button change on one pad.
type Event struct {
	Pad    int // Pad.ID
	Family Family
	Button Button
	Kind   EventKind
}

// Source reads controllers. Platform sources come from New; Fake is one for
// tests. A Source is used from the UI goroutine except for wake.
type Source interface {
	// Start begins reading. The source calls wake, from any goroutine,
	// when a pad connects, disconnects or changes, so the app draws a frame.
	Start(wake func()) error
	// Poll appends every connected pad's current state to dst.
	Poll(dst []Pad) []Pad
	// Close stops reading and releases the devices.
	Close() error
}

// continuous is a Source that can't wake the app on input (the web's
// Gamepad API has no button events), so Pads draws frames while a pad is
// connected.
type continuous interface{ Continuous() bool }

// ownFocus is a Source that already sees no input while the app is in the
// background, so Pads leaves the host's focus out of it. The web's is one:
// browsers hold back gamepad data from hidden pages, and there Focused
// tracks the text input's focus, not the page's.
type ownFocus interface{ OwnFocus() bool }

// Host is what Pads needs from the app: kit.App is one. Invalidate requests
// a frame from any goroutine. If the host also has Focused() bool, input is
// ignored while it reports false, so a background window doesn't react.
type Host interface{ Invalidate() }

type focuser interface{ Focused() bool }

// Debug makes sources log the devices they consider and every raw event
// to stderr (Linux), for working out why a pad doesn't respond. Set it
// before the first Frame; GIOKIT_PAD_DEBUG=1 sets it too.
var Debug = os.Getenv("GIOKIT_PAD_DEBUG") != ""

// Repeat timing for held buttons, like a held key.
const (
	RepeatDelay    = 350 * time.Millisecond
	RepeatInterval = 90 * time.Millisecond
)

// Stick thresholds for StickAsDPad, with hysteresis so a stick resting
// near the edge doesn't chatter.
const (
	stickPress   = 0.5
	stickRelease = 0.35
)

// Pads tracks the controllers of one window and turns their state into
// events, once per frame. Create it with New (platform source) or NewSource;
// the zero value is not usable. It belongs to the UI goroutine.
type Pads struct {
	// StickAsDPad makes the left stick press Up/Down/Left/Right too, so
	// menus and grids move with either. On by default.
	StickAsDPad bool
	// NintendoConfirm swaps A and B on Nintendo pads, so South is the A
	// button (which sits east there) and East is B, for players who expect
	// A to confirm as on a Switch. West and North stay by position (Y and
	// X on a Nintendo pad). Off by default: every button goes by position,
	// as on PC, so South (B on a Nintendo pad) confirms. Pads.Label
	// follows it.
	NintendoConfirm bool

	src     Source
	last    Family // family of the pad last pressed
	host    Host
	started bool
	err     error

	pads   []Pad
	prev   map[int]Buttons       // last frame's buttons by pad id
	held   map[heldKey]time.Time // next repeat
	events []Event
	used   []bool // events consumed by a Map
	buf    []Pad
}

type heldKey struct {
	pad int
	b   Button
}

// New returns Pads reading the platform's controllers, waking host on
// input. host may be nil when something else draws frames.
func New(host Host) *Pads { return NewSource(platformSource(), host) }

// NewSource returns Pads reading src.
func NewSource(src Source, host Host) *Pads {
	return &Pads{StickAsDPad: true, src: src, host: host, prev: map[int]Buttons{}, held: map[heldKey]time.Time{}}
}

// Err reports why the source failed to start, if it did. Pads without a
// working source just report no controllers.
func (p *Pads) Err() error { return p.err }

func (p *Pads) srcOwnsFocus() bool {
	o, ok := p.src.(ownFocus)
	return ok && o.OwnFocus()
}

// Frame reads the controllers and computes this frame's events, dropping
// the last frame's. Call it once per frame, before anything reads Events.
// It schedules the frames that auto-repeat and the web source need.
func (p *Pads) Frame(gtx layout.Context) {
	if !p.started {
		p.started = true
		wake := func() {}
		if p.host != nil {
			wake = p.host.Invalidate
		}
		if p.src != nil {
			p.err = p.src.Start(wake)
		}
	}
	p.events, p.used = p.events[:0], p.used[:0]
	p.buf = p.buf[:0]
	if p.src != nil && p.err == nil {
		p.buf = p.src.Poll(p.buf)
	}
	if f, ok := p.host.(focuser); ok && !f.Focused() && !p.srcOwnsFocus() {
		p.buf = p.buf[:0] // as if every button came up
	}
	p.pads = append(p.pads[:0], p.buf...)

	now := gtx.Now
	var next time.Time
	seen := map[int]bool{}
	for i := range p.pads {
		pd := &p.pads[i]
		seen[pd.ID] = true
		old := p.prev[pd.ID]
		if p.NintendoConfirm && pd.Family == Nintendo {
			pd.Buttons = swapConfirm(pd.Buttons)
		}
		if p.StickAsDPad {
			pd.Buttons = stickDPad(pd.Buttons, pd.Axes, old)
		}
		pd.Buttons = triggerButtons(pd.Buttons, pd.Axes, old)
		for b := range ButtonCount {
			was, is := old.Has(b), pd.Buttons.Has(b)
			k := heldKey{pd.ID, b}
			switch {
			case is && !was:
				p.add(Event{pd.ID, pd.Family, b, Press})
				p.last = pd.Family
				p.held[k] = now.Add(RepeatDelay)
			case !is && was:
				p.add(Event{pd.ID, pd.Family, b, Release})
				delete(p.held, k)
			case is:
				if at := p.held[k]; !now.Before(at) {
					p.add(Event{pd.ID, pd.Family, b, Repeat})
					p.held[k] = now.Add(RepeatInterval)
				}
			}
			if at, ok := p.held[k]; ok && (next.IsZero() || at.Before(next)) {
				next = at
			}
		}
		p.prev[pd.ID] = pd.Buttons
	}
	// Disconnected pads release what they held.
	for id, old := range p.prev {
		if seen[id] {
			continue
		}
		for b := range ButtonCount {
			if old.Has(b) {
				p.add(Event{id, Generic, b, Release})
				delete(p.held, heldKey{id, b})
			}
		}
		delete(p.prev, id)
	}
	if !next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	if c, ok := p.src.(continuous); ok && c.Continuous() && len(p.pads) > 0 {
		gtx.Execute(op.InvalidateCmd{})
	}
}

func (p *Pads) add(e Event) {
	p.events = append(p.events, e)
	p.used = append(p.used, false)
}

// Events returns this frame's button events, consumed ones included. The
// slice is reused by the next Frame.
func (p *Pads) Events() []Event { return p.events }

// Connected returns the connected pads as of this frame. The slice is
// reused by the next Frame.
func (p *Pads) Connected() []Pad { return p.pads }

// Family is the family of the pad last pressed, or else of the first
// connected pad: the one to label hints for.
func (p *Pads) Family() Family {
	for _, pd := range p.pads {
		if pd.Family == p.last {
			return p.last
		}
	}
	if len(p.pads) > 0 {
		return p.pads[0].Family
	}
	return Generic
}

// Label is how the pad to label hints for (Family) prints b, following
// NintendoConfirm: with it, South is "A" on a Nintendo pad too.
func (p *Pads) Label(b Button) string { return Label(p.Family(), p.physical(b)) }

// ASCIILabel is Label without symbols (see the package's ASCIILabel).
func (p *Pads) ASCIILabel(b Button) string { return ASCIILabel(p.Family(), p.physical(b)) }

// physical is the button in position that reports b, for labels.
func (p *Pads) physical(b Button) Button {
	if p.NintendoConfirm && p.Family() == Nintendo {
		switch b {
		case South:
			return East
		case East:
			return South
		}
	}
	return b
}

// Close stops the source.
func (p *Pads) Close() error {
	if p.src == nil || !p.started || p.err != nil {
		return nil
	}
	return p.src.Close()
}

// swapConfirm swaps South and East.
func swapConfirm(b Buttons) Buttons {
	out := b &^ (1<<South | 1<<East)
	if b.Has(South) {
		out = out.With(East)
	}
	if b.Has(East) {
		out = out.With(South)
	}
	return out
}

// stickDPad adds d-pad directions for a deflected left stick. old is last
// frame's buttons, which already include the stick's directions, for
// hysteresis.
func stickDPad(b Buttons, ax [AxisCount]float32, old Buttons) Buttons {
	dir := func(v float32, neg, pos Button) {
		if v <= -stickPress || v <= -stickRelease && old.Has(neg) {
			b = b.With(neg)
		}
		if v >= stickPress || v >= stickRelease && old.Has(pos) {
			b = b.With(pos)
		}
	}
	dir(ax[LeftX], Left, Right)
	dir(ax[LeftY], Up, Down)
	return b
}

// triggerButtons presses the trigger buttons for triggers past half
// travel, for sources that only report them as axes.
func triggerButtons(b Buttons, ax [AxisCount]float32, old Buttons) Buttons {
	for _, t := range [...]struct {
		a Axis
		b Button
	}{{LeftTriggerAxis, LeftTrigger}, {RightTriggerAxis, RightTrigger}} {
		if v := ax[t.a]; v >= stickPress || v >= stickRelease && old.Has(t.b) {
			b = b.With(t.b)
		}
	}
	return b
}
