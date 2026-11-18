package kit

import (
	"math"
	"strings"
	"testing"
)

func TestStringsValidate(t *testing.T) {
	if err := English().Validate(); err != nil {
		t.Fatalf("English: %v", err)
	}
	good := English()
	good.NumberRange = "Zwischen %[1]g und %[2]g"
	good.Expand = "%s ausklappen (100%%)"
	if err := good.Validate(); err != nil {
		t.Errorf("valid translation: %v", err)
	}
	reordered := English()
	reordered.NumberRange = "%[2]g bis %[1]g"
	if err := reordered.Validate(); err != nil {
		t.Errorf("reordered arguments: %v", err)
	}
	for field, f := range map[string]func(*Strings){
		"Expand":      func(s *Strings) { s.Expand = "Ausklappen" },
		"Collapse":    func(s *Strings) { s.Collapse = "Einklappen %d" },
		"MoveUp":      func(s *Strings) { s.MoveUp = "%s nach %s" },
		"Reorder":     func(s *Strings) { s.Reorder = "Ordnen %" },
		"Increase":    func(s *Strings) { s.Increase = "%[2]s erhöhen" },
		"NumberRange": func(s *Strings) { s.NumberRange = "Ab %g" },
		"Decrease":    func(s *Strings) { s.Decrease = "%[x]s" },
	} {
		s := English()
		f(&s)
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), "Strings."+field) {
			t.Errorf("%s broken: err = %v, want one naming the field", field, err)
		}
	}
}

// Tint and Highlight clamp their 0..1 inputs instead of wrapping.
func TestTintClamps(t *testing.T) {
	th := Dark()
	for _, v := range []float32{-1, 2, 300, float32(math.NaN()), float32(math.Inf(1))} {
		want := uint8(0)
		if v > 1 {
			want = 255
		}
		if got := th.Highlight(v).A; got != want {
			t.Errorf("Highlight(%v).A = %d, want %d", v, got, want)
		}
		if got := th.Tint(none, v).A; got != want {
			t.Errorf("Tint(transparent, %v).A = %d, want %d", v, got, want)
		}
	}
	if got := th.Highlight(0.5).A; got != 127 {
		t.Errorf("Highlight(0.5).A = %d, want 127", got)
	}
}
