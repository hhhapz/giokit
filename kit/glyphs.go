package kit

import (
	"unicode"

	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

// MissingGlyphs returns the runes in s that the theme's fonts can't draw
// (they would render as tofu boxes). Whitespace and control runes are ignored.
func (t *Theme) MissingGlyphs(s string) []rune {
	var missing []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if seen[r] || unicode.IsSpace(r) || unicode.IsControl(r) {
			continue
		}
		seen[r] = true
		t.Shaper.LayoutString(text.Parameters{PxPerEm: fixed.I(16), MaxWidth: 1 << 20}, string(r))
		for g, ok := t.Shaper.NextGlyph(); ok; g, ok = t.Shaper.NextGlyph() {
			if uint32(g.ID) == 0 { // low bits hold the font glyph id; 0 is .notdef
				missing = append(missing, r)
			}
		}
	}
	return missing
}
