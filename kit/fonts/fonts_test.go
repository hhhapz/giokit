package fonts

import (
	"testing"

	"gioui.org/font"
)

// Every set's UI and Mono typefaces must exist in its faces under exactly
// those names, in the weights components use, or text silently falls back
// to Go.
func TestSetsResolve(t *testing.T) {
	for _, name := range Names()[1:] {
		set, _ := ByName(name)
		have := map[font.Typeface]map[font.Weight]bool{}
		for _, f := range set.Faces {
			if have[f.Font.Typeface] == nil {
				have[f.Font.Typeface] = map[font.Weight]bool{}
			}
			have[f.Font.Typeface][f.Font.Weight] = true
		}
		for _, tf := range []font.Typeface{set.UI, set.Mono} {
			ws := have[tf]
			if ws == nil {
				t.Errorf("%s: no faces named %q; have %v", set.Name, tf, keys(have))
				continue
			}
			for _, w := range []font.Weight{font.Normal, font.Bold} {
				if !ws[w] {
					t.Errorf("%s: %q lacks weight %v (has %v)", set.Name, tf, w, ws)
				}
			}
		}
	}
}

// ByName must find every set Names lists, under that name.
func TestByName(t *testing.T) {
	for _, name := range Names() {
		if got, ok := ByName(name); !ok || got.Name != name {
			t.Errorf("ByName(%q) = set %q, %v", name, got.Name, ok)
		}
	}
	if _, ok := ByName("Comic Sans"); ok {
		t.Error("ByName found an unknown set")
	}
}

func keys(m map[font.Typeface]map[font.Weight]bool) []font.Typeface {
	var out []font.Typeface
	for k := range m {
		out = append(out, k)
	}
	return out
}
