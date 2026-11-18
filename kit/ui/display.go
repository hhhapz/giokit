package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/hhhapz/giokit/kit"
)

// Badge is a pill coloured by status.
type Badge struct {
	Status kit.Status // picks the pill's text and fill colours
	Text   string     // the pill's text, in the user's language
}

// Layout draws the pill at its natural size. It takes no input.
func (b Badge) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	fg, bg, _ := th.StatusColors(b.Status)
	return th.Pill(gtx, b.Text, fg, bg)
}

// statusIconNudge lowers a status icon (a banner's, a toast's) to sit on
// its title's first line.
const statusIconNudge unit.Dp = 1

// Banner is an inline alert: status icon, bold title, optional body.
type Banner struct {
	Status      kit.Status // picks the icon, tint and border
	Title, Body string     // a bold line, then an optional wrapped paragraph
}

// bannerBorderAlpha is the opacity of a Banner's border in its status
// colour: enough to edge the tinted fill without competing with the icon.
const bannerBorderAlpha = 60

// Layout draws the banner stretched to the available width. It takes no
// input; put actions beside it.
func (b Banner) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	fg, bg, icon := th.StatusColors(b.Status)
	return th.Styled(kit.Surface{Bg: bg, Border: kit.WithAlpha(fg, bannerBorderAlpha), Radius: th.Radius}).Layout(gtx, kit.Stretch(kit.Pad(th.Space(kit.Space2p5), th.Space(kit.Space3), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: statusIconNudge, Right: th.Space(kit.Space2p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return icon(gtx, th.IconSizeLg, fg) })
			}),
			kit.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Col(gtx,
					kit.Rigid(func(gtx layout.Context) layout.Dimensions {
						return th.Text(gtx, b.Title, th.Type.Body, th.Fg, kit.SemiBold)
					}),
					kit.Rigid(func(gtx layout.Context) layout.Dimensions {
						if b.Body == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Top: th.Space(kit.Space0p5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return th.Paragraph(gtx, b.Body, th.Muted) })
					}),
				)
			}),
		)
	})))
}

// Kbd renders a key chord as key caps: "Ctrl+K" is [Ctrl] [K], and
// "Ctrl++" is [Ctrl] [+]. Show a bound shortcut with
// Kbd{Chord: keymap.HintFor("file.save")}.
type Kbd struct {
	Chord string // a display chord, keys joined by "+" (kit.FormatChord's form)
}

// chordKeys splits a display chord into its keys. A trailing "+" after a
// separator is the plus key itself.
func chordKeys(chord string) []string {
	switch {
	case chord == "+":
		return []string{"+"}
	case strings.HasSuffix(chord, "++"):
		return append(strings.Split(chord[:len(chord)-2], "+"), "+")
	}
	return strings.Split(chord, "+")
}

// Layout draws one key cap per key, in a row. It takes no input.
func (k Kbd) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	keys := chordKeys(k.Chord)
	caps := make([]layout.FlexChild, 0, len(keys))
	for _, name := range keys {
		caps = append(caps, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
			return th.Styled(kit.Surface{Bg: th.Surface, Border: th.Border, Radius: th.RadiusSm()}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return kit.Flex{PadX: kit.Space1, PadY: kit.Space0p5}.Layout(gtx, th, kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Text(gtx, name, th.Type.Small, th.Muted, kit.Medium)
				}))
			})
		}))
	}
	return kit.Flex{Align: layout.Middle, Gap: kit.Space1}.Layout(gtx, th, caps...)
}

// Stat is a card with a caption and a big value; it fills the width (use
// it inside Flexed for equal-width rows).
type Stat struct {
	Title, Value string      // an uppercase caption above a large value
	ValueColor   color.NRGBA // zero is Fg
}

// Layout draws the card at the full available width. It takes no input.
func (s Stat) Layout(gtx layout.Context, th *kit.Theme) layout.Dimensions {
	vc := s.ValueColor
	if vc.A == 0 {
		vc = th.Fg
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return th.Styled(kit.Surface{Bg: th.Panel, Border: th.Divider, Radius: th.RadiusLg()}).Layout(gtx, kit.Stretch(kit.Pad(th.Space(kit.Space3), th.Space(kit.Space3p5), func(gtx layout.Context) layout.Dimensions {
		return kit.Col(gtx,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions { return th.Caption(gtx, s.Title) }),
			kit.Rigid(kit.Gap(0, th.Space(kit.Space1))),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return th.Text(gtx, s.Value, th.Type.Stat, vc, kit.SemiBold)
			}),
		)
	})))
}

// Artwork draws square, cropped artwork, or a placeholder while the image
// is not set. Label describes the image to screen readers and tests.
type Artwork struct {
	ID, Label string  // semantic id and alternative text (kit.TagImage)
	Size      unit.Dp // the square's side
}

// Layout draws im cropped to cover a Size square with the theme's corner
// radius, or an AccentBg placeholder while im holds no image. It reads im
// only; decode and Set it elsewhere. It takes no input.
func (a Artwork) Layout(gtx layout.Context, th *kit.Theme, im *kit.Image) layout.Dimensions {
	px := gtx.Dp(a.Size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	r := gtx.Dp(th.Radius)
	defer clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Max}, r).Push(gtx.Ops).Pop()
	kit.TagImage(gtx, a.ID, a.Label)
	if src, ok := im.Op(); ok {
		return widget.Image{Src: src, Fit: widget.Cover, Position: layout.Center}.Layout(gtx)
	}
	paint.FillShape(gtx.Ops, th.AccentBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// Split is a resizable two-pane layout. Drag the divider, or focus it (Tab)
// and use the arrow keys; keys it does not use reach the enclosing
// KeyContext. The divider's id is ID.
//
// Split fills its maximum constraints. Each pane keeps at least Min along
// the axis while there is room for both; when there is not, the panes
// share the space equally. With no room at all (zero size), both panes are
// laid out with zero size.
type Split struct {
	ID   string        // the divider's id
	Axis layout.Axis   // Horizontal: panes side by side; Vertical: stacked
	Min  unit.Dp       // minimum pane size; 0 is Theme.PaneMinSize
	A, B layout.Widget // the first (left or top) and second panes
}

// SplitState holds the divider position. Unlike most component values it
// is owned by the state: dragging is continuous interaction, so the split
// keeps its own Ratio rather than round-tripping it through props.
//
// Ratio is the first pane's share of the space along the axis, in 0..1;
// 0 means unset and becomes 0.5 at the first layout. Set it before layout
// to restore a saved split; it is clamped to keep both panes at least Min.
// Read it after layout to save it with your app state. A SplitState is its
// own event tag: do not copy it after first use.
type SplitState struct {
	Ratio float32 // the first pane's share, 0..1; see above

	drag  gesture.Drag
	hover gesture.Hover
	grab  float32 // pointer offset inside the handle at press
	start int     // handle position at press (px)
	pos   int     // handle position last frame (px)
	hov   kit.Tween
}

// splitKeyStep is how far one arrow key moves the divider: a share of the
// whole length.
const splitKeyStep = 0.05

// splitGeom is a split's geometry along its axis for one frame, in pixels.
type splitGeom struct {
	axis  layout.Axis
	total int // length along the axis
	min   int // minimum pane length
	hit   int // the handle's thickness: a usable target around the thin line
	line  int // the visible divider's thickness
}

// along is p's coordinate along the split axis.
func (g splitGeom) along(p image.Point) float32 {
	if g.axis == layout.Vertical {
		return float32(p.Y)
	}
	return float32(p.X)
}

// ratio converts a divider position px into a ratio, keeping both panes
// at least min long, or centring the divider when there is no room for
// that. It returns false when the split has no length.
func (g splitGeom) ratio(px float32) (float32, bool) {
	if g.total <= 0 {
		return 0, false
	}
	lo, hi := float32(g.min), float32(g.total-g.min)
	if hi < lo {
		lo, hi = float32(g.total)/2, float32(g.total)/2
	}
	return max(lo, min(hi, px)) / float32(g.total), true
}

// first is the first pane's length for ratio r, clamped like ratio and to
// leave room for the divider line.
func (g splitGeom) first(r float32) int {
	f := int(r * float32(g.total))
	f = max(min(f, g.total-g.min), min(g.min, g.total/2))
	return max(0, min(f, g.total-g.line))
}

// Layout draws both panes and the divider at s.Ratio, filling its maximum
// constraints. It handles the divider's drags, hover and arrow keys itself
// (see the package doc), clamps s.Ratio and leaves the result in it; there
// is no Update.
func (sp Split) Layout(gtx layout.Context, th *kit.Theme, s *SplitState) layout.Dimensions {
	if s.Ratio == 0 {
		s.Ratio = 0.5
	}
	g := splitGeom{
		axis:  sp.Axis,
		total: max(0, gtx.Constraints.Max.X),
		min:   gtx.Dp(sp.Min),
		hit:   gtx.Dp(th.HitSize),
		line:  max(1, gtx.Dp(th.Stroke)),
	}
	if sp.Axis == layout.Vertical {
		g.total = max(0, gtx.Constraints.Max.Y)
	}
	if sp.Min == 0 {
		g.min = gtx.Dp(th.PaneMinSize)
	}
	hovered := s.update(gtx, g)

	first := g.first(s.Ratio)
	s.pos = first - g.hit/2
	sp.pane(gtx, sp.A, 0, first)
	sp.pane(gtx, sp.B, first+g.line, g.total-first-g.line)
	s.paint(gtx, th, sp.ID, g, first, hovered)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

// update handles the divider's drag and keys before drawing, and reports
// whether the pointer is over it.
func (s *SplitState) update(gtx layout.Context, g splitGeom) bool {
	set := func(px float32) {
		if r, ok := g.ratio(px); ok {
			s.Ratio = r
			kit.Notify(gtx)
		}
	}
	axis := gesture.Horizontal
	if g.axis == layout.Vertical {
		axis = gesture.Vertical
	}
	for {
		e, ok := s.drag.Update(gtx.Metric, gtx.Source, axis)
		if !ok {
			break
		}
		// Gio reports a drag relative to the handle as it was at press time
		// (the grab), not as it is now: anchor to the press position.
		p := g.along(e.Position.Round())
		switch e.Kind {
		case pointer.Press:
			s.grab, s.start = p, s.pos
		case pointer.Drag:
			set(float32(s.start) + p - s.grab + float32(g.hit)/2)
		}
	}
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: s},
			key.Filter{Focus: s, Name: key.NameLeftArrow}, key.Filter{Focus: s, Name: key.NameRightArrow},
			key.Filter{Focus: s, Name: key.NameUpArrow}, key.Filter{Focus: s, Name: key.NameDownArrow},
		)
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		step := float32(g.total) * splitKeyStep
		switch ke.Name {
		case key.NameLeftArrow, key.NameUpArrow:
			set(s.Ratio*float32(g.total) - step)
		case key.NameRightArrow, key.NameDownArrow:
			set(s.Ratio*float32(g.total) + step)
		}
	}
	return s.hover.Update(gtx.Source)
}

// pane lays out w at off along the axis, size long and clipped to it.
func (sp Split) pane(gtx layout.Context, w layout.Widget, off, size int) {
	size = max(0, size)
	var o image.Point
	if sp.Axis == layout.Vertical {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, size))
		o = image.Pt(0, off)
	} else {
		gtx.Constraints = layout.Exact(image.Pt(size, gtx.Constraints.Max.Y))
		o = image.Pt(off, 0)
	}
	defer op.Offset(o).Push(gtx.Ops).Pop()
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	w(gtx)
}

// paint draws the divider line, highlighted while hovered, dragged or
// focused, and registers its handle: the drag, hover, cursor, focus and
// semantics.
func (s *SplitState) paint(gtx layout.Context, th *kit.Theme, id string, g splitGeom, first int, hovered bool) {
	focused := gtx.Focused(s)
	active := float32(0)
	if hovered || s.drag.Dragging() || focused {
		active = 1
	}
	ha := s.hov.To(gtx, active, th.Dur(kit.DurFast))
	// The highlight is the line widened by its own thickness on each side.
	maxX, maxY := gtx.Constraints.Max.X, gtx.Constraints.Max.Y
	lineRect := image.Rect(first, 0, first+g.line, maxY)
	hitRect := image.Rect(s.pos, 0, s.pos+g.hit, maxY)
	glow := image.Rect(first-g.line, 0, first+2*g.line, maxY)
	cursor := pointer.CursorColResize
	if g.axis == layout.Vertical {
		lineRect = image.Rect(0, first, maxX, first+g.line)
		hitRect = image.Rect(0, s.pos, maxX, s.pos+g.hit)
		glow = image.Rect(0, first-g.line, maxX, first+2*g.line)
		cursor = pointer.CursorRowResize
	}
	paint.FillShape(gtx.Ops, th.Border, clip.Rect(lineRect).Op())
	if ha > 0 {
		c := th.Accent
		if focused {
			c = th.Focus
		}
		paint.FillShape(gtx.Ops, kit.WithAlpha(c, uint8(255*ha)), clip.Rect(glow).Op())
	}
	defer clip.Rect(hitRect).Push(gtx.Ops).Pop()
	cursor.Add(gtx.Ops)
	s.drag.Add(gtx.Ops)
	s.hover.Add(gtx.Ops)
	event.Op(gtx.Ops, s)
	kit.Tag(gtx, id) // the drag listens for presses, so this area keeps its semantics
	kit.FocusID(gtx, s, id)
	kit.SetRole(gtx, id, kit.RoleSeparator)
}
