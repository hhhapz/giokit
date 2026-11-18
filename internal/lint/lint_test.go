// Package lint turns the structural rules in AGENTS.md into tests over the
// source of kit, kit/ui and kit/x. Rules that hold everywhere are strict
// (zero allowed); rules the code does not meet yet are ratchets: a per-file
// count that may only go down. When a count drops, lower its baseline in the
// same change.
//
// The rules are syntactic: they read the parsed source without type
// information, so each one documents the expression forms it sees and
// fixtures_test.go pins down what it catches and what it misses. A ratchet
// shows the count is not growing; it does not prove the code is free of the
// pattern.
//
// Go runs tests in the package directory, so sources are found relative to
// internal/lint ("../../kit"). Findings and baselines name files relative
// to kit: "menu.go", "ui/combobox.go".
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
	"unicode"
)

// kitRoot is the kit source tree, relative to this package's directory.
const kitRoot = "../../kit/"

// kitDirs are the directories under kitRoot that the rules read.
var kitDirs = []string{"", "ui/", "x/", "x/video/"}

// tokenFiles may define colours: they are the palette and token tables.
var tokenFiles = []string{"color.go", "palette.go", "theme.go", "tokens.go"}

// dpBaseline is the number of geometry literals per file today. They move
// into Theme metrics.
var dpBaseline = map[string]int{
	"menu.go":       2,
	"ui/reorder.go": 2,
}

// englishBaseline is the number of English strings shown or announced per
// file today. They move into Theme strings.
var englishBaseline = map[string]int{}

// finding is one rule violation.
type finding struct {
	pos token.Position
	msg string
}

// rule inspects one file and reports findings.
type rule func(fset *token.FileSet, f *ast.File) []finding

// colorLiterals reports colour literals: components get colour only from
// palette tokens.
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

// strokes reports uses of clip.Stroke: Gio re-flattens strokes on the CPU
// every frame (SKILL.md, "Outlines are fills").
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

// geometryCalls take sizes, radii or insets as arguments.
var geometryCalls = []string{"Dp", "Box", "Fixed", "Gap", "Pad", "UniformInset", "Space", "ControlPad", "control", "Spinner"}

// geometryFields are struct fields that hold sizes, radii or insets.
var geometryFields = []string{"Radius", "Top", "Bottom", "Left", "Right", "Gap", "Pad", "PadX", "PadY"}

// dpLiterals reports fixed sizes, radii and insets written as numbers
// instead of theme tokens. It counts arguments of geometryCalls and of icon
// helpers (kit.IconChevronDown(gtx, 12, c)), and values of geometryFields,
// when the value is a nonzero number or arithmetic that adds one to the
// Space scale or to other numbers (gtx.Dp(14+8), kit.Space1-1, -4). Spacing
// uses the scale: th.Space(kit.Space2), kit.Flex{Gap: kit.Space2}.
//
// It does not see numbers that reach a size through a variable or a
// constant declared elsewhere, or arithmetic with a measured value
// (unit.Dp(1-p) scales by an animation progress).
func dpLiterals(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	add := func(n ast.Node, what string) {
		out = append(out, finding{pos: fset.Position(n.Pos()), msg: "geometry literal in " + what + ": use a theme metric"})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			name := callName(n.Fun)
			if !slices.Contains(geometryCalls, name) && !isIconHelper(name) {
				return true
			}
			for _, a := range n.Args {
				if literalGeometry(a) {
					add(a, name)
				}
			}
		case *ast.KeyValueExpr:
			if k, ok := n.Key.(*ast.Ident); ok && slices.Contains(geometryFields, k.Name) && literalGeometry(n.Value) {
				add(n.Value, k.Name)
			}
		}
		return true
	})
	return out
}

// isIconHelper reports whether name is one of kit's IconX(gtx, size, colour)
// helpers.
func isIconHelper(name string) bool {
	rest, ok := strings.CutPrefix(name, "Icon")
	return ok && rest != "" && unicode.IsUpper(rune(rest[0]))
}

// literalGeometry reports whether e writes a size as a number: a nonzero
// literal, or arithmetic that combines one with numbers or the Space scale.
func literalGeometry(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return isNumber(e) && !isZero(e)
	case *ast.ParenExpr:
		return literalGeometry(e.X)
	case *ast.UnaryExpr:
		return (e.Op == token.SUB || e.Op == token.ADD) && literalGeometry(e.X)
	case *ast.BinaryExpr:
		return literalGeometry(e.X) && constOperand(e.Y) || literalGeometry(e.Y) && constOperand(e.X)
	}
	return false
}

// constOperand reports whether e is a number, a Space scale step, or
// arithmetic on them.
func constOperand(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return isNumber(e)
	case *ast.ParenExpr:
		return constOperand(e.X)
	case *ast.UnaryExpr:
		return constOperand(e.X)
	case *ast.BinaryExpr:
		return constOperand(e.X) && constOperand(e.Y)
	}
	return strings.HasPrefix(callName(e), "Space")
}

// english reports user-visible text written into the toolkit. Counted:
//   - string literals that read as words (a space between letters, or a
//     capitalised word);
//   - literals holding typographic punctuation shown to users, such as the
//     en dash of a range ("1–10") or an ellipsis;
//   - English computed at run time: the String method of time.Month and
//     time.Weekday (m.Month().String(), time.Weekday(i).String()).
//
// Exempt: ids and context names (Part, Tag, KeyContext, Scope, Path),
// developer messages (errors, panics, and any literal that starts with a
// package prefix such as "kit: "), and imports. It does not see English
// that arrives through a variable, or other formatters (time.Time.Format
// layouts are literals, and caught as words).
func english(fset *token.FileSet, f *ast.File) []finding {
	var out []finding
	add := func(n ast.Node) {
		out = append(out, finding{pos: fset.Position(n.Pos()), msg: "English in the toolkit: take it from the caller or Theme strings"})
	}
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
			case "String":
				if calendarName(n) {
					add(n)
				}
			}
		case *ast.BasicLit:
			if !skip[n] && n.Kind == token.STRING && userText(n.Value) {
				add(n)
			}
		}
		return true
	})
	return out
}

// calendarName reports whether call is x.String() on a time.Month or
// time.Weekday: the receiver is a Month() or Weekday() call, or a
// time.Month(...) or time.Weekday(...) conversion.
func calendarName(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	recv, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fn := recv.Fun.(type) {
	case *ast.SelectorExpr:
		if isSel(fn, "time", "Month") || isSel(fn, "time", "Weekday") {
			return true
		}
		return len(recv.Args) == 0 && (fn.Sel.Name == "Month" || fn.Sel.Name == "Weekday")
	}
	return false
}

// typographic is punctuation that only appears in text for people.
const typographic = "–—…‹›«»“”‘’·•"

// devPrefix matches a developer message's package prefix: "kit: ",
// "video: ".
func devPrefix(s string) bool {
	pkg, _, ok := strings.Cut(s, ": ")
	if !ok || pkg == "" {
		return false
	}
	for _, r := range pkg {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '/') {
			return false
		}
	}
	return true
}

// userText reports whether the source literal lit is text for people: it
// reads as words or holds typographic punctuation, and is not a developer
// message.
func userText(lit string) bool {
	s := strings.Trim(lit, "`\"")
	if devPrefix(s) {
		return false
	}
	return readsAsWords(s) || strings.ContainsAny(s, typographic)
}

// readsAsWords reports whether s reads as words: a space between letters,
// or a capitalised word.
func readsAsWords(s string) bool {
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

// callName is the name an identifier or selector refers to: "Dp" for
// gtx.Dp and unit.Dp.
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

// parseKit parses the non-test sources in kitDirs, keyed by their path
// relative to kitRoot.
func parseKit(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, dir := range kitDirs {
		paths, err := filepath.Glob(kitRoot + dir + "*.go")
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
	t.Helper()
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
	t.Helper()
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
		t.Logf("%s baseline:\n%s", name, baselineLines(got))
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
		t.Errorf("%s went down in %v: lower the baseline to\n%s", name, changed, baselineLines(got))
	}
}

// baselineLines formats counts as sorted map entries to paste into a
// baseline.
func baselineLines(counts map[string]int) string {
	var lines []string
	for file, n := range counts {
		lines = append(lines, fmt.Sprintf("\t%q: %d,", file, n))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
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
// diagnostics; strings.go is the default English Strings; the token files
// name tokens and presets.
var englishExempt = append([]string{"keymap.go", "trace.go", "strings.go"}, tokenFiles...)

func TestEnglishRatchet(t *testing.T) {
	ratchet(t, english, englishBaseline, "English strings", englishExempt)
}

// coverageExempt are kit/ui components without a conformance row, and why.
var coverageExempt = map[string]string{
	// display only: no input of their own
	"Artwork": "display", "Badge": "display", "Banner": "display", "DialogContent": "display",
	"EmptyState": "display", "FieldError": "display", "Form": "layout of other controls",
	"Kbd": "display", "Stat": "display", "ToastHost": "host; its close buttons are IconButtons",
	"Tooltip": "no input of its own: it shows on its Content's hover or focus (TestTooltipKeyboard)",
	// interactive, covered by gallery tests until their contract exists
	"Combobox":       "its trigger is an editor: typing opens the list",
	"ContextMenu":    "opens on a right click",
	"CommandPalette": "a modal",
	"Split":          "a drag control",
}

// Every kit/ui component (an exported type with a Layout method) has a
// gallery demo, and a conformance row unless coverageExempt says why.
func TestComponentCoverage(t *testing.T) {
	fset := token.NewFileSet()
	paths, err := filepath.Glob(kitRoot + "ui/*.go")
	if err != nil {
		t.Fatal(err)
	}
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
		ps, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range ps {
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			b.Write(src)
		}
		return b.String()
	}
	gallery := read("../../examples/gallery/*.go")
	conform := read(kitRoot + "ui/conform_test.go")
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
