// SPDX-License-Identifier: Unlicense OR MIT

package keylayout

import "testing"

// TestTables checks each platform table names every code exactly once.
func TestTables(t *testing.T) {
	for name, table := range map[string][]string{"evdev": evdev[:], "mac": mac[:]} {
		seen := map[string]bool{}
		for _, c := range table {
			if c == "" {
				continue
			}
			if _, ok := US[c]; !ok || seen[c] {
				t.Errorf("%s: unknown or repeated code %q", name, c)
			}
			seen[c] = true
		}
		if len(seen) != len(US) {
			t.Errorf("%s names %d of %d codes", name, len(seen), len(US))
		}
	}
	if CodeFor("Q") != "KeyQ" || CodeFor("Space") != "Space" || CodeFor(`\`) != "Backslash" {
		t.Error("CodeFor guesses wrong")
	}
}

// TestLearn relabels every key from one press that shows the layout.
func TestLearn(t *testing.T) {
	defer Reset()
	Learn("KeyA", "A") // the same on US and Dvorak: nothing to go on
	if Name("KeyS") != "S" {
		t.Fatal("an unrevealing press changed the layout")
	}
	Learn("KeyS", "O") // only Dvorak types O there
	if Name("KeyQ") != "'" || Name("KeyJ") != "H" || Name("Slash") != "Z" {
		t.Fatalf("after a Dvorak press: KeyQ %q, KeyJ %q, Slash %q", Name("KeyQ"), Name("KeyJ"), Name("Slash"))
	}
	Learn("KeyQ", "A") // AZERTY now
	if Name("KeyW") != "Z" || Name("KeyA") != "Q" {
		t.Fatalf("after an AZERTY press: KeyW %q, KeyA %q", Name("KeyW"), Name("KeyA"))
	}
	Learn("KeyS", "S") // AZERTY types S there too: still AZERTY
	Learn("KeyQ", "Q") // back to US
	if Name("KeyQ") != "Q" || Name("KeyW") != "W" {
		t.Fatalf("after a US press: KeyQ %q, KeyW %q", Name("KeyQ"), Name("KeyW"))
	}
	Learn("KeyQ", "Ж") // a layout it doesn't know: just that key
	if Name("KeyQ") != "Ж" || Name("KeyW") != "W" {
		t.Fatalf("unknown layout: KeyQ %q, KeyW %q", Name("KeyQ"), Name("KeyW"))
	}
	for i, l := range common {
		for c := range l {
			if _, ok := US[c]; !ok {
				t.Errorf("layout %d has unknown code %q", i, c)
			}
		}
	}
}
