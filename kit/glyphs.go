package kit

import (
	"unicode"

	"github.com/hhhapz/giokit/gio/text"
	"golang.org/x/image/math/fixed"
)

// glyphProbeSize is the text size, in pixels, MissingGlyphs shapes at; any
// size finds the same glyphs.
const glyphProbeSize = 16

// MissingGlyphs returns the runes in s that the theme's fonts can't draw
// (they would render as tofu boxes), each once, in order of first
// appearance. Whitespace and control runes are ignored.
//
// It shapes each rune on its own, so it judges runes, not text: a glyph
// that only exists in context (a ligature, a combining mark drawn with its
// base) is not checked. It reads the font glyph index from the low 32 bits
// of Gio's text.GlyphID, where 0 is .notdef; that layout is Gio's internal
// encoding, and a Gio upgrade could change it.
func (t *Theme) MissingGlyphs(s string) []rune {
	var missing []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if seen[r] || unicode.IsSpace(r) || unicode.IsControl(r) {
			continue
		}
		seen[r] = true
		t.Shaper.LayoutString(text.Parameters{PxPerEm: fixed.I(glyphProbeSize), MaxWidth: unbounded}, string(r))
		notdef := false
		for g, ok := t.Shaper.NextGlyph(); ok; g, ok = t.Shaper.NextGlyph() {
			if uint32(g.ID) == 0 { // the font glyph index; 0 is .notdef
				notdef = true
			}
		}
		if notdef {
			missing = append(missing, r)
		}
	}
	return missing
}
