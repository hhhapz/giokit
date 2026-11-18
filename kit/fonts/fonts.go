// Package fonts holds giokit's curated font sets, bundled so text renders
// the same on every machine. Each set is a UI face plus a code face.
//
//	th.UseFonts(fonts.Inter())
//
// The font files are embedded in the binary. A program that calls only
// specific set constructors links only those sets' files; ByName refers to
// every set, so calling it links them all. Each family is parsed once, on
// first use (a parsed family costs a few MB of heap, several times its
// file size), and the parsed faces are shared by every set that includes
// it. A picker lists Names and loads only the set the user chooses.
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

// The bundled families, each parsed on first use.
var (
	interFamily     = family{fsys: interFS, dir: "inter"}
	jetbrainsFamily = family{fsys: jetbrainsFS, dir: "jetbrainsmono"}
	plexSansFamily  = family{fsys: plexSansFS, dir: "plexsans"}
	plexMonoFamily  = family{fsys: plexMonoFS, dir: "plexmono"}
	atkinsonFamily  = family{fsys: atkinsonFS, dir: "atkinson"}
)

// Go is kit's default: Go and Go Mono (no extra files).
func Go() kit.Fonts { return kit.Fonts{Name: "Go", UI: "Go", Mono: "Go Mono"} }

// Inter is a neutral, highly legible UI sans with JetBrains Mono for code.
func Inter() kit.Fonts {
	return kit.Fonts{Name: "Inter", UI: "Inter", Mono: "JetBrains Mono", Faces: faces(&interFamily, &jetbrainsFamily)}
}

// Plex is IBM Plex Sans with IBM Plex Mono: technical, slightly condensed.
func Plex() kit.Fonts {
	return kit.Fonts{Name: "Plex", UI: "IBM Plex Sans", Mono: "IBM Plex Mono", Faces: faces(&plexSansFamily, &plexMonoFamily)}
}

// Atkinson is Atkinson Hyperlegible, designed for low-vision readers
// (distinct letterforms, regular and bold only), with JetBrains Mono.
func Atkinson() kit.Fonts {
	return kit.Fonts{Name: "Atkinson", UI: "Atkinson Hyperlegible", Mono: "JetBrains Mono", Faces: faces(&atkinsonFamily, &jetbrainsFamily)}
}

// Names lists every set's name, Go first, for a picker. It loads nothing;
// pass the chosen name to ByName.
func Names() []string { return []string{"Go", "Inter", "Plex", "Atkinson"} }

// ByName returns the set called name, as stored in Theme.Fonts and tokens.
// It loads only the families of that set.
func ByName(name string) (kit.Fonts, bool) {
	switch name {
	case "Go":
		return Go(), true
	case "Inter":
		return Inter(), true
	case "Plex":
		return Plex(), true
	case "Atkinson":
		return Atkinson(), true
	}
	return kit.Fonts{}, false
}

// family is one directory of embedded font files, parsed once. It is
// shared, immutable after loading and safe for concurrent use.
type family struct {
	fsys embed.FS
	dir  string

	once  sync.Once
	faces []font.FontFace
}

// load returns the family's faces, parsing them on the first call. The
// files are bundled, so a read or parse failure is a build defect and
// panics.
func (f *family) load() []font.FontFace {
	f.once.Do(func() {
		entries, err := f.fsys.ReadDir(f.dir)
		if err != nil {
			panic(fmt.Errorf("fonts: %s: %w", f.dir, err))
		}
		for _, e := range entries {
			name := path.Join(f.dir, e.Name())
			data, err := f.fsys.ReadFile(name)
			if err != nil {
				panic(fmt.Errorf("fonts: %s: %w", name, err))
			}
			fs, err := opentype.ParseCollection(data)
			if err != nil {
				panic(fmt.Errorf("fonts: %s: %w", name, err))
			}
			f.faces = append(f.faces, fs...)
		}
	})
	return f.faces
}

// faces returns a new slice of the UI family's faces followed by the mono
// family's.
func faces(ui, mono *family) []font.FontFace {
	return append(append([]font.FontFace(nil), ui.load()...), mono.load()...)
}
