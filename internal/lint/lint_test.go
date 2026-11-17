// Package lint turns the structural rules in AGENTS.md into tests over the
// source of kit/. Rules that hold everywhere are strict (zero allowed);
// rules the code does not meet yet are ratchets: a per-file count that may
// only go down. When a count drops, lower its baseline in the same change.
package lint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// tokenFiles may define colours: they are the palette and token tables.
var tokenFiles = []string{"color.go", "theme.go", "tokens.go"}

// dpBaseline is the number of geometry literals per file today. They move
// into Theme metrics.
var dpBaseline = map[string]int{
	"draw.go": 2, "feedback.go": 1, "menu.go": 1, "overlay.go": 6, "scroll.go": 8,
	"ui/collections.go": 2, "ui/datefield.go": 3, "ui/display.go": 6, "ui/navigation.go": 1, "ui/overlays.go": 3,
	"widgets.go": 4, "x/logview.go": 4,
}

// englishBaseline is the number of English string literals shown or
// announced per file today. They move into Theme strings.
var englishBaseline = map[string]int{}

type finding struct {
	file string
	pos  token.Position
	msg  string
}

// rule inspects one file and reports findings.
type rule func(fset *token.FileSet, f *ast.File) []finding

// colorLiterals: components get colour only from palette tokens.
func colorLiterals(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	ast.Inspect(f, func(n ast.Node) bool {
		if cl, ok := n.(*ast.CompositeLit); ok && isSel(cl.Type, "color", "NRGBA") {
			out = append(out, finding{pos: fset.Position(cl.Pos()), msg: "colour literal: use a palette token"})
		}
		return true
	})
	return out
}

// strokes: Gio re-flattens strokes on the CPU every frame (SKILL rule 14).
func strokes(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && isSel(s, "clip", "Stroke") {
			out = append(out, finding{pos: fset.Position(s.Pos()), msg: "clip.Stroke: use kit.Border or fills"})
		}
		return true
	})
	return out
}

// dpLiterals: fixed sizes, radii and insets written as numbers instead of
// theme tokens. Counted: numeric arguments of Dp/Box/Fixed/Gap/
// UniformInset/Space, and numeric Radius/Top/Bottom/Left/Right/Gap/Pad
// fields. Spacing uses the scale: t.Space(kit.Space2), kit.Flex{Gap: kit.Space2}.
func dpLiterals(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	add := func(n ast.Node, what string) {
		out = append(out, finding{pos: fset.Position(n.Pos()), msg: "geometry literal in " + what + ": use a theme metric"})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			name := callName(n.Fun)
			switch name {
			case "Dp", "Box", "Fixed", "Gap", "UniformInset", "Space":
				for _, a := range n.Args {
					if isNumber(a) && !isZero(a) {
						add(a, name)
					}
				}
			}
		case *ast.KeyValueExpr:
			if k, ok := n.Key.(*ast.Ident); ok && isNumber(n.Value) && !isZero(n.Value) {
				switch k.Name {
				case "Radius", "Top", "Bottom", "Left", "Right", "Gap", "Pad", "PadX", "PadY":
					add(n.Value, k.Name)
				}
			}
		}
		return true
	})
	return out
}

// english: user-visible text written into the toolkit. Counted: string
// literals that read as words (a space, or a capitalised word), wherever
// they are, except ids and context names (Part, Tag, KeyContext, Scope),
// developer messages (errors, panics) and imports.
func english(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	skip := map[ast.Node]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.CallExpr:
			switch callName(n.Fun) {
			case "Part", "Tag", "KeyContext", "Scope", "Path", "Errorf", "New", "panic", "Sprint":
				for _, a := range n.Args {
					skip[a] = true
				}
			}
		case *ast.BasicLit:
			if !skip[n] && n.Kind == token.STRING && readsAsWords(n.Value) {
				out = append(out, finding{pos: fset.Position(n.Pos()), msg: "English in the toolkit: take it from the caller or Theme strings"})
			}
		}
		return true
	})
	return out
}

// readsAsWords: a space between letters, or a capitalised word.
func readsAsWords(lit string) bool {
	s := strings.Trim(lit, "`\"")
	if strings.HasPrefix(s, "kit: ") { // an error message, for developers
		return false
	}
	letters := 0
	for i, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			letters++
		}
		if r >= 'A' && r <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
			return true
		}
	}
	return letters >= 2 && strings.Contains(s, " ")
}

func isSel(e ast.Expr, pkg, name string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == pkg && s.Sel.Name == name
}

func callName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

func isNumber(e ast.Expr) bool {
	b, ok := e.(*ast.BasicLit)
	return ok && (b.Kind == token.INT || b.Kind == token.FLOAT)
}

func isZero(e ast.Expr) bool {
	b, _ := e.(*ast.BasicLit)
	return b != nil && (b.Value == "0" || b.Value == "0.0")
}

// parseKit parses kit/*.go, kit/ui/*.go and kit/x/*.go, tests excluded. Keys are paths
// relative to kit: "menu.go", "ui/combobox.go".
func parseKit(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, dir := range []string{"", "ui/", "x/", "x/video/"} {
		paths, err := filepath.Glob("../../kit/" + dir + "*.go")
		if err != nil || len(paths) == 0 {
			t.Fatalf("no kit/%s sources: %v", dir, err)
		}
		for _, p := range paths {
			if strings.HasSuffix(p, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			files[dir+filepath.Base(p)] = f
		}
	}
	return fset, files
}

func strict(t *testing.T, r rule, skip []string) {
	fset, files := parseKit(t)
	for name, f := range files {
		if slices.Contains(skip, name) {
			continue
		}
		for _, x := range r(fset, f) {
			t.Errorf("%s: %s", x.pos, x.msg)
		}
	}
}

// ratchet fails when a file's count rises above its baseline, and when it
// falls (so the baseline follows it down). LINT_BASELINE=1 prints the
// current counts to paste in.
func ratchet(t *testing.T, r rule, baseline map[string]int, name string, skip []string) {
	fset, files := parseKit(t)
	got := map[string]int{}
	for file, f := range files {
		if slices.Contains(skip, file) {
			continue
		}
		if n := len(r(fset, f)); n > 0 {
			got[file] = n
		}
	}
	if os.Getenv("LINT_BASELINE") != "" {
		var lines []string
		for file, n := range got {
			lines = append(lines, fmt.Sprintf("\t%q: %d,", file, n))
		}
		sort.Strings(lines)
		t.Logf("%s baseline:\n%s", name, strings.Join(lines, "\n"))
		return
	}
	var changed []string
	for file := range union(got, baseline) {
		switch g, b := got[file], baseline[file]; {
		case g > b:
			t.Errorf("%s: %d %s, baseline %d. New ones:", file, g, name, b)
			for _, x := range r(fset, files[file]) {
				t.Logf("  %s: %s", x.pos, x.msg)
			}
		case g < b:
			changed = append(changed, file)
		}
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		var lines []string
		for file := range union(got, baseline) {
			if got[file] > 0 {
				lines = append(lines, fmt.Sprintf("\t%q: %d,", file, got[file]))
			}
		}
		sort.Strings(lines)
		t.Errorf("%s went down in %v: lower the baseline to\n%s", name, changed, strings.Join(lines, "\n"))
	}
}

func union(a, b map[string]int) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

func TestNoColorLiterals(t *testing.T) { strict(t, colorLiterals, tokenFiles) }
func TestNoStrokes(t *testing.T)       { strict(t, strokes, nil) }
func TestGeometryRatchet(t *testing.T) {
	ratchet(t, dpLiterals, dpBaseline, "geometry literals", tokenFiles)
}

// keymap.go names keys for chord parsing and display; trace.go writes
// diagnostics; the token files name tokens and presets.
var englishExempt = append([]string{"keymap.go", "trace.go", "metrics.go"}, tokenFiles...)

func TestEnglishRatchet(t *testing.T) {
	ratchet(t, english, englishBaseline, "English strings", englishExempt)
}

// Each rule must catch the bug it exists for.
func TestRulesCatch(t *testing.T) {
	src := `package kit
func bad(gtx C, t *Theme, id string) {
	_ = color.NRGBA{R: 1}
	_ = clip.Stroke{}
	_ = Surface{Radius: 4}
	_ = gtx.Dp(22)
	t.Text(gtx, "No log lines yet", 0, t.Fg, 0)
	semantic.LabelOp("Decrease " + id).Add(gtx.Ops)
	label := "Name"
	label += " (asc)"
	Tag(gtx, Part(id, "Sort Key")) // ids: fine
	panic("kit: Bad state")       // developer message: fine
	_ = t.Space(8)       // off the scale
	_ = Flex{Gap: 6}     // off the scale
	_ = t.Space(Space2)  // the scale: fine
	_ = Flex{Pad: Space4} // fine
	_ = gtx.Dp(0)        // zero: fine
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "bad.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		r    rule
		want int
	}{"colour": {colorLiterals, 1}, "stroke": {strokes, 1}, "geometry": {dpLiterals, 4}, "english": {english, 4}} {
		if got := len(c.r(fset, f)); got != c.want {
			t.Errorf("%s rule found %d, want %d: %v", name, got, c.want, c.r(fset, f))
		}
	}
}

// coverageExempt are kit/ui components without a conformance row, and why.
var coverageExempt = map[string]string{
	// display only: no input of their own
	"Artwork": "display", "Badge": "display", "Banner": "display", "DialogContent": "display",
	"EmptyState": "display", "FieldError": "display", "Form": "layout of other controls",
	"Kbd": "display", "Stat": "display", "ToastHost": "host; its close buttons are IconButtons",
	"Tooltip": "display; hover-only",
	// interactive, covered by gallery tests until their contract exists
	"Combobox":       "its trigger is an editor: typing opens the list",
	"ContextMenu":    "opens on a right click",
	"CommandPalette": "a modal",
	"Split":          "a drag control",
	"ReorderList":    "its buttons are IconButtons; the drag handle needs a drag contract",
}

// Every kit/ui component (an exported type with a Layout method) has a
// gallery demo, and a conformance row unless coverageExempt says why.
func TestComponentCoverage(t *testing.T) {
	fset := token.NewFileSet()
	paths, _ := filepath.Glob("../../kit/ui/*.go")
	var comps []string
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Layout" || fn.Recv == nil {
				continue
			}
			if id, ok := fn.Recv.List[0].Type.(*ast.Ident); ok && id.IsExported() {
				comps = append(comps, id.Name)
			}
		}
	}
	read := func(glob string) string {
		var b strings.Builder
		ps, _ := filepath.Glob(glob)
		for _, p := range ps {
			src, _ := os.ReadFile(p)
			b.Write(src)
		}
		return b.String()
	}
	gallery := read("../../examples/gallery/*.go")
	conform := read("../../kit/ui/conform_test.go")
	for _, c := range comps {
		if !strings.Contains(gallery, "ui."+c+"{") {
			t.Errorf("ui.%s has no gallery demo (examples/gallery)", c)
		}
		_, exempt := coverageExempt[c]
		if !exempt && !strings.Contains(conform, "ui."+c+"{") {
			t.Errorf("ui.%s has no conformance row (kit/ui/conform_test.go) and no coverageExempt reason", c)
		}
		if exempt && strings.Contains(conform, "ui."+c+"{") {
			t.Errorf("ui.%s has a conformance row now: drop it from coverageExempt", c)
		}
	}
	if len(comps) < 30 {
		t.Fatalf("found only %d components: the scan is broken", len(comps))
	}
}
