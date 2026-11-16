// Package fonts holds giokit's curated font sets, bundled so text renders
// the same on every machine. Each set is a UI face plus a code face; the
// files are embedded, and only sets an app calls are linked into it.
//
//	th.UseFonts(fonts.Inter())
//
// All fonts are under the SIL Open Font License 1.1 (OFL.txt beside each).
package fonts

import (
	"embed"
	"fmt"
	"path"
	"sync"

	"gioui.org/font"
	"gioui.org/font/opentype"

	"github.com/hhhapz/giokit/kit"
)

//go:embed inter/*.ttf
var interFS embed.FS

//go:embed jetbrainsmono/*.ttf
var jetbrainsFS embed.FS

//go:embed plexsans/*.ttf
var plexSansFS embed.FS

//go:embed plexmono/*.ttf
var plexMonoFS embed.FS

//go:embed atkinson/*.ttf
var atkinsonFS embed.FS

// Go is kit's default: Go and Go Mono (no extra files).
func Go() kit.Fonts { return kit.Fonts{Name: "Go", UI: "Go", Mono: "Go Mono"} }

// Inter is a neutral, highly legible UI sans with JetBrains Mono for code.
func Inter() kit.Fonts {
	return kit.Fonts{Name: "Inter", UI: "Inter", Mono: "JetBrains Mono", Faces: faces(&interOnce, interFS, "inter", &jetbrainsOnce, jetbrainsFS, "jetbrainsmono")}
}

// Plex is IBM Plex Sans with IBM Plex Mono: technical, slightly condensed.
func Plex() kit.Fonts {
	return kit.Fonts{Name: "Plex", UI: "IBM Plex Sans", Mono: "IBM Plex Mono", Faces: faces(&plexSansOnce, plexSansFS, "plexsans", &plexMonoOnce, plexMonoFS, "plexmono")}
}

// Atkinson is Atkinson Hyperlegible, designed for low-vision readers
// (distinct letterforms, regular and bold only), with JetBrains Mono.
func Atkinson() kit.Fonts {
	return kit.Fonts{Name: "Atkinson", UI: "Atkinson Hyperlegible", Mono: "JetBrains Mono", Faces: faces(&atkinsonOnce, atkinsonFS, "atkinson", &jetbrainsOnce, jetbrainsFS, "jetbrainsmono")}
}

// All lists every set, Go first, for a picker.
func All() []kit.Fonts { return []kit.Fonts{Go(), Inter(), Plex(), Atkinson()} }

// ByName returns the set called name, as stored in Theme.Fonts and tokens.
func ByName(name string) (kit.Fonts, bool) {
	for _, f := range All() {
		if f.Name == name {
			return f, true
		}
	}
	return kit.Fonts{}, false
}

var (
	interOnce, jetbrainsOnce, plexSansOnce, plexMonoOnce, atkinsonOnce family
)

// family parses a directory of font files once.
type family struct {
	once  sync.Once
	faces []font.FontFace
}

func (f *family) load(fsys embed.FS, dir string) []font.FontFace {
	f.once.Do(func() {
		entries, err := fsys.ReadDir(dir)
		if err != nil {
			panic(err)
		}
		for _, e := range entries {
			data, err := fsys.ReadFile(path.Join(dir, e.Name()))
			if err != nil {
				panic(err)
			}
			fs, err := opentype.ParseCollection(data)
			if err != nil {
				panic(fmt.Sprintf("fonts: %s: %v", e.Name(), err)) // bundled files: a bug, not input
			}
			f.faces = append(f.faces, fs...)
		}
	})
	return f.faces
}

func faces(ui *family, uiFS embed.FS, uiDir string, mono *family, monoFS embed.FS, monoDir string) []font.FontFace {
	return append(append([]font.FontFace(nil), ui.load(uiFS, uiDir)...), mono.load(monoFS, monoDir)...)
}
