// Command padtest prints what kit/pad sees, for checking a controller
// without an app: pads connecting, button presses and releases (by the
// standard layout and as the pad labels them), and stick and trigger
// movement.
//
//	go run ./examples/padtest
//
// On Linux it also logs each evdev device it considers and every raw
// event (GIOKIT_PAD_DEBUG), and -hidraw /dev/hidrawN dumps a pad's raw HID
// reports, decoding Nintendo pads' buttons, to tell whether presses reach
// the computer at all when nothing else sees them.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hhhapz/giokit/gio/io/input"
	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/kit/pad"
)

var buttonNames = [pad.ButtonCount]string{
	"South", "East", "West", "North", "LeftShoulder", "RightShoulder", "LeftTrigger", "RightTrigger",
	"Back", "Start", "LeftStick", "RightStick", "Up", "Down", "Left", "Right", "Guide",
}

var axisNames = [pad.AxisCount]string{"LeftX", "LeftY", "RightX", "RightY", "LeftTrigger", "RightTrigger"}

var familyNames = []string{"generic", "xbox", "playstation", "nintendo", "steam"}

func main() {
	hidraw := flag.String("hidraw", "", "also dump raw HID reports from this device (Linux), e.g. /dev/hidraw9")
	raw := flag.Bool("raw", true, "log raw evdev events (Linux)")
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	pad.Debug = *raw
	if *hidraw != "" {
		go dumpHIDRaw(*hidraw)
	}

	pads := pad.New(nil)
	var r input.Router
	known := map[int]pad.Pad{}
	log.Print("waiting for controllers; press buttons, move sticks. Ctrl+C quits.")
	for range time.Tick(time.Second / 60) {
		pads.Frame(layout.Context{Now: time.Now(), Source: r.Source()})
		if err := pads.Err(); err != nil {
			log.Fatalf("source: %v", err)
		}
		seen := map[int]bool{}
		for _, p := range pads.Connected() {
			seen[p.ID] = true
			old, ok := known[p.ID]
			if !ok {
				log.Printf("pad %d connected: %q (%s)", p.ID, p.Name, familyNames[p.Family])
			}
			for a := range pad.AxisCount {
				if ok && p.Axes[a] != old.Axes[a] {
					log.Printf("pad %d %s %+.3f", p.ID, axisNames[a], p.Axes[a])
				}
			}
			known[p.ID] = p
		}
		for id := range known {
			if !seen[id] {
				log.Printf("pad %d disconnected", id)
				delete(known, id)
			}
		}
		for _, e := range pads.Events() {
			if e.Kind == pad.Repeat {
				continue
			}
			kind := "pressed"
			if e.Kind == pad.Release {
				kind = "released"
			}
			log.Printf("pad %d %s %s (%s)", e.Pad, buttonNames[e.Button], kind, pads.Label(e.Button))
		}
	}
}

// dumpHIDRaw prints a HID device's reports when they change. Nintendo full
// reports (id 0x30) are decoded to button names; anything else is printed
// as hex, without the timer byte.
func dumpHIDRaw(path string) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("hidraw: %v", err)
		return
	}
	defer f.Close()
	log.Printf("hidraw: reading %s", path)
	buf := make([]byte, 512)
	var last string
	for {
		n, err := f.Read(buf)
		if err != nil {
			log.Printf("hidraw: %v", err)
			return
		}
		b := buf[:n]
		var s string
		if n >= 12 && b[0] == 0x30 {
			s = "nintendo buttons: " + nintendoButtons(b[3], b[4], b[5]) +
				fmt.Sprintf(" sticks % x", b[6:12])
		} else if n > 2 {
			s = fmt.Sprintf("report %#02x: % x", b[0], b[2:min(n, 20)])
		}
		if s != last {
			log.Printf("hidraw: %s", s)
			last = s
		}
	}
}

func nintendoButtons(right, shared, left byte) string {
	names := []struct {
		b    byte
		bit  byte
		name string
	}{
		{right, 0x01, "Y"}, {right, 0x02, "X"}, {right, 0x04, "B"}, {right, 0x08, "A"},
		{right, 0x40, "R"}, {right, 0x80, "ZR"},
		{shared, 0x01, "-"}, {shared, 0x02, "+"}, {shared, 0x04, "RS"}, {shared, 0x08, "LS"},
		{shared, 0x10, "HOME"}, {shared, 0x20, "CAPTURE"},
		{left, 0x01, "DOWN"}, {left, 0x02, "UP"}, {left, 0x04, "RIGHT"}, {left, 0x08, "LEFT"},
		{left, 0x40, "L"}, {left, 0x80, "ZL"},
	}
	s := ""
	for _, n := range names {
		if n.b&n.bit != 0 {
			s += n.name + " "
		}
	}
	if s == "" {
		return "(none)"
	}
	return s
}
