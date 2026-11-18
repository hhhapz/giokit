package conform

import (
	"image"
	"image/color"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gioui.org/gesture"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

// Each broken control must make the suite fail with the right message. A
// failing subtest fails its parent, so each runs in a child process
// (CONFORM_BROKEN names the case) and the parent checks the output.
var broken = map[string]struct {
	want string
	c    Case
}{
	"unlabeled": {"needs a human label", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: custom(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State) {
			semantic.Button.Add(gtx.Ops) // no LabelOp: a screen reader says nothing (or the id)
			fill(gtx, th.Surface, st)
		})}},
	"no-hover": {"hover changes nothing", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: custom(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State) {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp("X").Add(gtx.Ops)
			st.Hover, st.Hovered = 0, false
			fill(gtx, th.Surface, st)
		})}},
	// A transparent control whose hover fill is Panel: visible on the window,
	// gone on a card.
	"hover-equals-card": {"hover changes nothing on a card", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: custom(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State) {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp("X").Add(gtx.Ops)
			defer clip.Rect{Max: image.Pt(80, 32)}.Push(gtx.Ops).Pop()
			if st.Hovered {
				paint.Fill(gtx.Ops, th.Panel)
			}
			if st.Focused {
				kit.Border(gtx, image.Rectangle{Max: image.Pt(80, 32)}, 0, 2, th.Focus)
			}
		})}},
	// A button that tells kit it is a tab: a screen reader announces the
	// wrong control.
	"wrong-role": {"role tab, want button", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: custom(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State) {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp("X").Add(gtx.Ops)
			kit.SetRole(gtx, "x", kit.RoleTab)
			fill(gtx, th.Surface, st)
		})}},
	"no-ring": {"no visible ring", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: custom(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State) {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp("X").Add(gtx.Ops)
			st.Focused = false
			fill(gtx, th.Surface, st)
		})}},
	// The Solid-button bug: a ring in the fill's own colour, visible only in
	// a few anti-aliased corner pixels.
	"invisible-ring": {"no visible ring", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: custom(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State) {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp("X").Add(gtx.Ops)
			s := kit.Surface{Bg: th.Accent, Radius: 6}
			if st.Focused {
				s.Focus = th.Accent
			}
			s.Bg = kit.LerpColor(s.Bg, th.Fg, 0.3*st.Hover)
			s.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(80, 32)} })
		})}},
	"ignores-disabled": {"fired 1 times on click", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton,
		Build: func(th *kit.Theme, disabled bool) Control {
			var c kit.Clickable
			n := new(int)
			return Control{Activations: n, Frame: func(gtx layout.Context) { // never wraps itself in th.Disable
				if c.Clicked(gtx) {
					*n++
				}
				ui.Button{ID: "x", Text: "X"}.Layout(gtx, th, &c)
			}}
		}}},
	"pointer-only": {"Tab cannot reach", Case{ID: "x", Class: semantic.Button, Role: kit.RoleButton, NoKeys: true,
		Build: func(th *kit.Theme, disabled bool) Control {
			var g gesture.Click
			var hover gesture.Hover
			n := new(int)
			return Control{Activations: n, Frame: func(gtx layout.Context) {
				for {
					e, ok := g.Update(gtx.Source)
					if !ok {
						break
					}
					if e.Kind == gesture.KindClick {
						*n++
					}
				}
				h := hover.Update(gtx.Source)
				defer clip.Rect{Max: image.Pt(80, 32)}.Push(gtx.Ops).Pop()
				g.Add(gtx.Ops)
				hover.Add(gtx.Ops)
				kit.TagLabel(gtx, "x", "X")
				semantic.Button.Add(gtx.Ops)
				col := th.Surface
				if h {
					col = th.Panel
				}
				paint.Fill(gtx.Ops, col)
			}}
		}}},
}

// custom builds a case from a raw draw callback on a Clickable's Press.
func custom(draw func(th *kit.Theme, gtx layout.Context, c *kit.Clickable, st kit.State)) Builder {
	return func(th *kit.Theme, disabled bool) Control {
		var c kit.Clickable
		n := new(int)
		return Control{Activations: n, Frame: func(gtx layout.Context) {
			if c.Clicked(gtx) {
				*n++
			}
			th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
				return th.Press(gtx, &c, "x", func(gtx layout.Context, st kit.State) layout.Dimensions {
					draw(th, gtx, &c, st)
					return layout.Dimensions{Size: image.Pt(80, 32)}
				})
			})
		}}
	}
}

func fill(gtx layout.Context, base color.NRGBA, st kit.State) {
	c := kit.LerpColor(base, kit.WithAlpha(base, 120), st.Hover)
	defer clip.Rect{Max: image.Pt(80, 32)}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, c)
	if st.Focused {
		kit.Border(gtx, image.Rectangle{Max: image.Pt(80, 32)}, 0, 2, kit.WithAlpha(base, 40))
	}
}

func TestBrokenControl(t *testing.T) {
	name := os.Getenv("CONFORM_BROKEN")
	if name == "" {
		t.Skip("run by TestCatchesBrokenControls")
	}
	b := broken[name].c
	b.Name = name
	Run(t, b)
}

func TestCatchesBrokenControls(t *testing.T) {
	if os.Getenv("CONFORM_BROKEN") != "" {
		t.Skip()
	}
	for name, b := range broken {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestBrokenControl$", "-test.v")
			cmd.Env = append(os.Environ(), "CONFORM_BROKEN="+name)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the suite passed a broken control:\n%s", out)
			}
			if !strings.Contains(string(out), b.want) {
				t.Fatalf("the suite failed, but not with %q:\n%s", b.want, out)
			}
		})
	}
}

// brokenMenu builds a menu composite and lets break_ misbehave after each
// frame's Update (picked reports a pick this frame).
func brokenMenu(break_ func(gtx layout.Context, m *ui.MenuState, other *kit.Clickable, picked bool, n *int)) func(*kit.Theme) Control {
	return func(th *kit.Theme) Control {
		var m ui.MenuState
		var other kit.Clickable
		n := new(int)
		items := []ui.MenuItem{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
		return Control{Activations: n, Frame: func(gtx layout.Context) {
			_, picked := m.Update(gtx)
			other.Clicked(gtx)
			break_(gtx, &m, &other, picked, n)
			kit.Col(gtx,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.Menu{ID: "m", Items: items, Trigger: func(gtx layout.Context) layout.Dimensions {
						return ui.Button{ID: "m:btn", Text: "Menu"}.Layout(gtx, th, &m.Trigger)
					}}.Layout(gtx, th, &m)
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.Button{ID: "other", Text: "Other"}.Layout(gtx, th, &other)
				}),
			)
		}}
	}
}

var brokenComposite = map[string]struct {
	want string
	c    Composite
}{
	"focus-lost-on-pick": {"after a pick: focus", Composite{Trigger: "m:btn", Panel: "m", Item: "m/a",
		Build: brokenMenu(func(gtx layout.Context, m *ui.MenuState, other *kit.Clickable, picked bool, n *int) {
			if picked {
				*n++
				other.Focus(gtx) // focus lands somewhere else
			}
		})}},
	"escape-reopens": {"Escape left", Composite{Trigger: "m:btn", Panel: "m", Item: "m/a",
		Build: brokenMenu(reopen())}},
	"double-pick": {"Enter picked 2 times", Composite{Trigger: "m:btn", Panel: "m", Item: "m/a",
		Build: brokenMenu(func(gtx layout.Context, m *ui.MenuState, other *kit.Clickable, picked bool, n *int) {
			if picked {
				*n += 2
			}
		})}},
}

// reopen breaks a menu by reopening it whenever it closes without a pick:
// the dismissal is ignored.
func reopen() func(gtx layout.Context, m *ui.MenuState, other *kit.Clickable, picked bool, n *int) {
	wasOpen := map[*ui.MenuState]bool{}
	return func(gtx layout.Context, m *ui.MenuState, other *kit.Clickable, picked bool, n *int) {
		if picked {
			*n++
		} else if wasOpen[m] && !m.Opened() {
			m.Show()
		}
		wasOpen[m] = m.Opened()
	}
}

func TestBrokenComposite(t *testing.T) {
	name := os.Getenv("CONFORM_BROKEN_COMPOSITE")
	if name == "" {
		t.Skip("run by TestCatchesBrokenComposites")
	}
	b := brokenComposite[name].c
	b.Name = name
	RunComposite(t, b)
}

func TestCatchesBrokenComposites(t *testing.T) {
	if os.Getenv("CONFORM_BROKEN_COMPOSITE") != "" {
		t.Skip()
	}
	for name, b := range brokenComposite {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestBrokenComposite$", "-test.v")
			cmd.Env = append(os.Environ(), "CONFORM_BROKEN_COMPOSITE="+name)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the suite passed a broken composite:\n%s", out)
			}
			if !strings.Contains(string(out), b.want) {
				t.Fatalf("the suite failed, but not with %q:\n%s", b.want, out)
			}
		})
	}
}

// A "list" of separate buttons: every row its own Tab stop, no arrow keys.
var brokenGroup = map[string]struct {
	want string
	c    Group
}{
	"buttons-not-a-group": {"Down reported 0 picks", Group{First: "l/a", Second: "l/b",
		Build: func(th *kit.Theme, disabled bool) Control {
			var a, b kit.Clickable
			n := new(int)
			return Control{Activations: n, Frame: func(gtx layout.Context) {
				if a.Clicked(gtx) {
					*n++
				}
				if b.Clicked(gtx) {
					*n++
				}
				th.Disable(gtx, disabled, func(gtx layout.Context) layout.Dimensions {
					return kit.Col(gtx,
						kit.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.Button{ID: "l/a", Text: "Alpha"}.Layout(gtx, th, &a)
						}),
						kit.Rigid(func(gtx layout.Context) layout.Dimensions {
							return ui.Button{ID: "l/b", Text: "Beta"}.Layout(gtx, th, &b)
						}),
					)
				})
			}}
		}}},
}

func TestBrokenGroup(t *testing.T) {
	name := os.Getenv("CONFORM_BROKEN_GROUP")
	if name == "" {
		t.Skip("run by TestCatchesBrokenGroups")
	}
	b := brokenGroup[name].c
	b.Name = name
	RunGroup(t, b)
}

func TestCatchesBrokenGroups(t *testing.T) {
	if os.Getenv("CONFORM_BROKEN_GROUP") != "" {
		t.Skip()
	}
	for name, b := range brokenGroup {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestBrokenGroup$", "-test.v")
			cmd.Env = append(os.Environ(), "CONFORM_BROKEN_GROUP="+name)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the suite passed a broken group:\n%s", out)
			}
			if !strings.Contains(string(out), b.want) {
				t.Fatalf("the suite failed, but not with %q:\n%s", b.want, out)
			}
		})
	}
}
