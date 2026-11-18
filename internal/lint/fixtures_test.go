package lint

import (
	"go/parser"
	"go/token"
	"testing"
)

// ruleFixture is one statement and the number of findings a rule must
// report for it: positive fixtures show what the rule catches, zero-count
// fixtures what it deliberately lets through or cannot see.
type ruleFixture struct {
	name string
	stmt string
	want int
}

// checkFixtures runs r over each fixture, wrapped in a function body.
func checkFixtures(t *testing.T, r rule, fixtures []ruleFixture) {
	t.Helper()
	for _, fx := range fixtures {
		src := "package kit\nfunc f() {\n\t" + fx.stmt + "\n}\n"
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "fixture.go", src, 0)
		if err != nil {
			t.Fatalf("%s: %v", fx.name, err)
		}
		if got := r(fset, f); len(got) != fx.want {
			t.Errorf("%s: %s: %d findings, want %d: %v", fx.name, fx.stmt, len(got), fx.want, got)
		}
	}
}

func TestColorLiteralsFixtures(t *testing.T) {
	checkFixtures(t, colorLiterals, []ruleFixture{
		{name: "literal", stmt: `_ = color.NRGBA{R: 1}`, want: 1},
		{name: "token", stmt: `_ = t.Accent`, want: 0},
	})
}

func TestStrokesFixtures(t *testing.T) {
	checkFixtures(t, strokes, []ruleFixture{
		{name: "stroke", stmt: `_ = clip.Stroke{}`, want: 1},
		{name: "border", stmt: `Border(gtx, r, 4, 1, t.Border)`, want: 0},
	})
}

func TestGeometryFixtures(t *testing.T) {
	checkFixtures(t, dpLiterals, []ruleFixture{
		// caught
		{name: "Dp literal", stmt: `_ = gtx.Dp(22)`, want: 1},
		{name: "unit conversion", stmt: `_ = unit.Dp(12)`, want: 1},
		{name: "literal arithmetic", stmt: `_ = gtx.Dp(14 + 8)`, want: 1},
		{name: "off the scale", stmt: `_ = gtx.Dp(Space1 - 1)`, want: 1},
		{name: "off the scale, qualified", stmt: `_ = gtx.Dp(kit.Space1 - 1)`, want: 1},
		{name: "scaled step", stmt: `_ = t.Space(2 * kit.Space2)`, want: 1},
		{name: "negative", stmt: `_ = gtx.Dp(-4)`, want: 1},
		{name: "parenthesised", stmt: `_ = gtx.Dp((4))`, want: 1},
		{name: "Space literal", stmt: `_ = t.Space(8)`, want: 1},
		{name: "control padding", stmt: `_ = t.ControlPad(ControlDefault, 7)`, want: 1},
		{name: "private control padding", stmt: `_ = t.control(5)`, want: 1},
		{name: "icon size", stmt: `IconChevronDown(gtx, 12, t.Muted)`, want: 1},
		{name: "qualified icon size", stmt: `kit.IconCheck(gtx, 12, th.Bg)`, want: 1},
		{name: "Pad", stmt: `_ = Pad(6, 10, w)`, want: 2},
		{name: "Box", stmt: `Box(gtx, 18, s, w)`, want: 1},
		{name: "Fixed", stmt: `_ = Fixed(240, w)`, want: 1},
		{name: "Gap", stmt: `_ = Gap(12, 0)`, want: 1},
		{name: "spinner size", stmt: `t.Spinner(gtx, 18)`, want: 1},
		{name: "radius field", stmt: `_ = Surface{Radius: 4}`, want: 1},
		{name: "gap field", stmt: `_ = Flex{Gap: 6}`, want: 1},
		{name: "inset arithmetic", stmt: `_ = layout.Inset{Top: Space2 + 2}`, want: 1},
		// let through
		{name: "zero", stmt: `_ = gtx.Dp(0)`, want: 0},
		{name: "scale step", stmt: `_ = t.Space(Space2)`, want: 0},
		{name: "scale field", stmt: `_ = Flex{Pad: Space4}`, want: 0},
		{name: "metric", stmt: `kit.IconCheck(gtx, th.IconSize, th.Bg)`, want: 0},
		{name: "scale times a variable", stmt: `_ = th.Space(kit.Space4 * unit.Dp(depth))`, want: 0},
		{name: "animation progress", stmt: `_ = gtx.Dp(transitionSlide * unit.Dp(1-p))`, want: 0},
		{name: "measured value", stmt: `_ = gtx.Dp(size * 2)`, want: 0},
		{name: "not geometry", stmt: `_ = strconv.Itoa(12)`, want: 0},
		{name: "not an icon helper", stmt: `Icons(gtx, 12)`, want: 0},
		// out of reach: the number reaches the size through a name
		{name: "named constant", stmt: `_ = gtx.Dp(fieldPadY)`, want: 0},
	})
}

func TestEnglishFixtures(t *testing.T) {
	checkFixtures(t, english, []ruleFixture{
		// caught
		{name: "sentence", stmt: `t.Text(gtx, "No log lines yet", 0, t.Fg, 0)`, want: 1},
		{name: "concatenated label", stmt: `semantic.LabelOp("Decrease " + id).Add(gtx.Ops)`, want: 1},
		{name: "capitalised word", stmt: `label := "Name"`, want: 1},
		{name: "appended words", stmt: `label += " (asc)"`, want: 1},
		{name: "en dash range", stmt: `title = strconv.Itoa(d) + "–" + strconv.Itoa(d+9)`, want: 1},
		{name: "ellipsis", stmt: `_ = name + "…"`, want: 1},
		{name: "month name", stmt: `_ = m.Month().String()[:3]`, want: 1},
		{name: "weekday name", stmt: `_ = time.Weekday(i % 7).String()[:2]`, want: 1},
		{name: "month conversion", stmt: `_ = time.Month(i).String()`, want: 1},
		{name: "date layout", stmt: `_ = d.Format("January 2006")`, want: 1},
		// let through
		{name: "id part", stmt: `Tag(gtx, Part(id, "Sort Key"))`, want: 0},
		{name: "developer panic", stmt: `panic("kit: Bad state")`, want: 0},
		{name: "error", stmt: `_ = fmt.Errorf("ui: bad index %d", i)`, want: 0},
		{name: "prefixed message", stmt: `_ = fmt.Sprintf("video: Decoder closed %d", n)`, want: 0},
		{name: "context name", stmt: `KeyContext(gtx, "Menu Bar", w)`, want: 0},
		{name: "identifier-like", stmt: `_ = "row-1"`, want: 0},
		{name: "number format", stmt: `_ = fmt.Sprintf("%d:%02d", m, s)`, want: 0},
		{name: "other String", stmt: `_ = b.String()`, want: 0},
		{name: "level String", stmt: `_ = l.Level.String()`, want: 0},
		// out of reach: English that arrives through a name
		{name: "variable", stmt: `t.Text(gtx, label, 0, t.Fg, 0)`, want: 0},
	})
}
