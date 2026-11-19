package fonts

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	gtfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/hhhapz/giokit/gio/font"
	"github.com/hhhapz/giokit/gio/font/opentype"
)

// System fallback: giokit draws with bundled fonts only, so text looks the
// same everywhere. Scripts no bundled set covers (Chinese, Japanese,
// Korean) would draw as boxes; SystemFallback borrows one installed font
// for them. Add its faces to a set (Fonts.Faces) after the bundled ones:
// the shaper takes each rune from the theme's own face when it has it, so
// Latin text is unchanged and only the missing script comes from the
// system.
//
//	cjk, err := fonts.SystemCJK() // slow the first time: run it off the UI goroutine
//	f := fonts.Go()
//	f.Faces = append(f.Faces, cjk...)
//	th.UseFonts(f)

// ErrNoSystemFont is returned when no installed font covers the runes.
var ErrNoSystemFont = errors.New("fonts: no installed font covers the script")

// cjkSample is a rune from each CJK script: Han, Hiragana, Hangul.
var cjkSample = []rune{'中', 'あ', '한'}

// cjkPrefer are CJK families in order of preference, normalised (lower
// case, no spaces), as the system index stores them. Simplified Chinese
// forms first: the most common CJK UI text.
var cjkPrefer = []string{
	"notosanscjksc", "sourcehansanssc", "notosanssc", "notosanscjk", "sourcehansans",
	"pingfangsc", "microsoftyahei", "hiraginosansgb", "wenquanyimicrohei", "droidsansfallback",
}

var (
	cjkOnce  sync.Once
	cjkFaces []font.FontFace
	cjkErr   error
)

// SystemCJK finds an installed font covering Chinese, Japanese and Korean
// and returns its regular face. It runs once per process; later calls
// return the same faces. The first run on a machine indexes the system
// fonts (seconds; the index is cached in the user cache directory), so
// call it off the UI goroutine.
func SystemCJK() ([]font.FontFace, error) {
	cjkOnce.Do(func() { cjkFaces, cjkErr = SystemFallback(cjkSample, cjkPrefer...) })
	return cjkFaces, cjkErr
}

// SystemFallback finds the installed font that covers every rune of
// sample and returns its regular face. prefer lists family names, best
// first, matched without case or spaces; among fonts that match none, a
// sans-serif regular face wins. The returned face is renamed so it can
// never be chosen by typeface name, only to fill in runes.
func SystemFallback(sample []rune, prefer ...string) ([]font.FontFace, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	fps, err := fontscan.SystemFonts(log.New(io.Discard, "", 0), filepath.Join(cache, "giokit", "fontscan"))
	if err != nil {
		return nil, fmt.Errorf("fonts: index system fonts: %w", err)
	}
	best, bestScore := -1, -1
	for i, fp := range fps {
		if !covers(fp, sample) {
			continue
		}
		if s := score(fp, prefer); s > bestScore {
			best, bestScore = i, s
		}
	}
	if best < 0 {
		return nil, ErrNoSystemFont
	}
	loc := fps[best].Location
	src, err := os.ReadFile(loc.File)
	if err != nil {
		return nil, fmt.Errorf("fonts: read %s: %w", loc.File, err)
	}
	faces, err := opentype.ParseCollection(src)
	if err != nil {
		return nil, fmt.Errorf("fonts: parse %s: %w", loc.File, err)
	}
	if int(loc.Index) >= len(faces) {
		return nil, fmt.Errorf("fonts: %s has no face %d", loc.File, loc.Index)
	}
	f := faces[loc.Index]
	f.Font.Typeface = "giokit system fallback" // matched by coverage, never by name
	return []font.FontFace{f}, nil
}

// covers reports whether fp has every rune of sample.
func covers(fp fontscan.Footprint, sample []rune) bool {
	for _, r := range sample {
		if !fp.Runes.Contains(r) {
			return false
		}
	}
	return true
}

// score ranks a candidate: a preferred family (earlier is better), then
// sans-serif, then a regular upright face.
func score(fp fontscan.Footprint, prefer []string) int {
	s := 0
	fam := strings.ReplaceAll(strings.ToLower(fp.Family), " ", "")
	if i := slices.IndexFunc(prefer, func(p string) bool { return strings.HasPrefix(fam, p) }); i >= 0 {
		s += 1000 - 10*i
	}
	if strings.Contains(fam, "sans") && !strings.Contains(fam, "mono") {
		s += 50
	}
	if fp.Aspect.Style == gtfont.StyleNormal {
		s += 20
	}
	if w := fp.Aspect.Weight; w >= 380 && w <= 420 {
		s += 30
	}
	return s
}
