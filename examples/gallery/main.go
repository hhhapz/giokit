// Command gallery shows every giokit component: the kitchen sink and the
// reference for how an app is structured (handle → layout, ids on everything,
// keymap for navigation, theme as a value you can swap at runtime).
//
//	go run ./examples/gallery                        # window
//	go run ./examples/gallery -shot x.png [-page p]  # offscreen PNG
//	go run ./examples/gallery -tokens > theme.json   # export the theme as DTCG tokens
//	go run ./examples/gallery -theme theme.json      # start with a token file
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"gioui.org/layout"

	"github.com/hhhapz/giokit/kit"
	_ "github.com/hhhapz/giokit/kit/forktrace" // Gio window events in GIOKIT_TRACE
	"github.com/hhhapz/giokit/kit/ui"
)

type C = layout.Context
type D = layout.Dimensions

type gallery struct {
	catalog   *catalog // the pages; shared and never changed
	th        *kit.Theme
	page      string
	nav       kit.Pool[kit.Clickable]
	pageLists kit.Pool[kit.List] // demo pages' scroll state, by page id
	navGroup  kit.FocusGroup
	side      kit.List
	trans     kit.Transition
	keys      kit.Keymap
	actions   kit.Actions
	palette   ui.CommandPaletteState
	primary   kit.Clickable
	themeBt   kit.Clickable
	clicks    int
	status    string

	// Each page's demo state. A page's handle and layout methods own its
	// field; a few demos share controls across pages (the Overview's
	// playground reuses the Selection page's switches and slider).
	overview  overviewState
	controls  controlsState
	logs      logsState
	overlays  overlaysState
	panes     layoutState
	theme     themeState
	workspace workspaceState
	patterns  patternsState
	media     mediaState
}

func newGallery() *gallery {
	g := &gallery{catalog: demoCatalog, page: "overview"}
	g.initTheme()
	g.initOverview()
	g.initControls()
	g.initLogs()
	g.initOverlays()
	g.initPanes()
	g.initWorkspace()
	g.initPatterns()
	g.initMedia()
	defaults := map[string]string{"theme.next": "Shortcut+T", "palette.open": "Shortcut+P"}
	for i, p := range g.catalog.pages {
		id := "page." + p.id
		g.actions.MustRegister(kit.Action{ID: id, Title: "Go to " + p.title, Run: func() { g.page = p.id }})
		if i < 9 { // the rest are a palette search away
			defaults[id] = fmt.Sprintf("Shortcut+%d", i+1)
		}
	}
	g.actions.MustRegister(kit.Action{ID: "theme.next", Title: "Next theme", Run: g.nextTheme})
	g.actions.MustRegister(kit.Action{ID: "palette.open", Title: "Open command palette", Run: g.palette.Open})
	if err := g.keys.BindActions(&g.actions, defaults, nil); err != nil {
		panic(err)
	}
	return g
}

func (g *gallery) Frame(gtx C) {
	g.keys.Attach(gtx) // window-wide: resolved after layout, focused widgets first
	g.handle(gtx)
	g.layout(gtx)
}

// keyNav makes a column of clickables one focus group, with Up/Down/Home/End
// moving the current id: what kit's own lists do, built from the public
// FocusGroup, KeyContext and OnAction. The members join the group in the
// handle pass (FocusGroup.Join).
type keyNav struct {
	ID      string      // the group's id
	IDs     []string    // member ids, in order
	Current *string     // the current member; the keys move it
	Reveal  func(i int) // scrolls IDs[i] into view after a move
}

// Layout lays out w as the group's focusable area. Each move starts from
// the current member as the previous key left it, so keys delivered in one
// frame all count.
func (n keyNav) Layout(gtx C, grp *kit.FocusGroup, w layout.Widget) D {
	return kit.KeyContext(gtx, "nav", func(gtx C) D {
		jump := func(to int) {
			to = max(0, min(len(n.IDs)-1, to))
			*n.Current = n.IDs[to]
			n.Reveal(to)
		}
		step := func(delta int) func() {
			return func() { jump(slices.Index(n.IDs, *n.Current) + delta) }
		}
		kit.OnAction(gtx, kit.ActMoveUp, step(-1))
		kit.OnAction(gtx, kit.ActMoveDown, step(1))
		kit.OnAction(gtx, kit.ActFirst, func() { jump(0) })
		kit.OnAction(gtx, kit.ActLast, func() { jump(len(n.IDs) - 1) })
		return grp.Layout(gtx, n.ID, w)
	})
}

// handle reads every input before anything is drawn (rule 1).
func (g *gallery) handle(gtx C) {
	for _, p := range g.catalog.pages {
		c := g.nav.Get(p.id)
		g.navGroup.Join(c, p.id == g.page)
		if c.Clicked(gtx) {
			g.page = p.id
		}
	}
	if g.primary.Clicked(gtx) {
		g.clicks++
		g.status = fmt.Sprintf("Primary clicked %d×", g.clicks)
	}
	if g.themeBt.Clicked(gtx) {
		g.nextTheme()
	}
	g.handleOverview(gtx)
	g.handleControls(gtx)
	g.handleLogs(gtx)
	g.handleOverlays(gtx)
	g.handlePanes(gtx)
	g.handleTheme(gtx)
	g.handleWorkspace(gtx)
	g.handlePatterns(gtx)
	g.handleMedia(gtx)
}

func (g *gallery) layout(gtx C) D {
	th := g.th
	return ui.CommandPalette{ID: "commands", Actions: &g.actions, Page: func(gtx C) D {
		return ui.ToastHost{Page: func(gtx C) D {
			return th.Modal(gtx, &g.overlays.dialog, "dialog", g.window, g.deleteDialog)
		}}.Layout(gtx, th, &g.overlays.toasts)
	}}.Layout(gtx, th, &g.palette)
}

// window is the whole window: title bar, sidebar, page, status bar.
func (g *gallery) window(gtx C) D {
	th := g.th
	return kit.Fill(gtx, th.Bg, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return th.Bar(gtx, kit.Space2, kit.Space4, func(gtx C) D {
					return kit.Row(gtx,
						kit.Rigid(func(gtx C) D { return th.Text(gtx, "giokit gallery", th.Type.Text, th.Fg, kit.Bold) }),
						kit.Rigid(kit.Gap(th.Space(kit.Space3), 0)),
						kit.Rigid(func(gtx C) D { return th.Pill(gtx, "v0", th.Muted, th.Surface) }),
						kit.Flexed(1, kit.Grow),
						kit.Rigid(func(gtx C) D {
							return ui.Button{ID: "theme-next", Text: th.Name, Icon: kit.IconSun, Style: ui.Ghost}.Layout(gtx, th, &g.themeBt)
						}),
						kit.Rigid(kit.Gap(th.Space(kit.Space2), 0)),
						kit.Rigid(func(gtx C) D {
							return ui.Button{ID: "primary", Text: "Primary", Style: ui.Primary}.Layout(gtx, th, &g.primary)
						}),
					)
				})
			}),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Flexed(1, func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(kit.Fixed(200, g.sidebar)),
					kit.Rigid(kit.VLine(th.Border)),
					kit.Flexed(1, func(gtx C) D {
						gtx.Constraints.Min = gtx.Constraints.Max
						return g.trans.Layout(gtx, th, g.page, g.currentPage)
					}),
				)
			}),
			kit.Rigid(kit.HLine(th.Border)),
			kit.Rigid(func(gtx C) D {
				return th.Bar(gtx, kit.Space1, kit.Space4, func(gtx C) D {
					s := g.status
					if s == "" {
						s = "Ready"
					}
					return kit.Row(gtx,
						kit.Rigid(func(gtx C) D { return th.Text(gtx, s, th.Type.Small, th.Muted, kit.Regular) }),
						kit.Flexed(1, kit.Grow),
						kit.Rigid(func(gtx C) D { return th.Text(gtx, "Next theme", th.Type.Small, th.Dim, kit.Regular) }),
						kit.Rigid(kit.Gap(th.Space(kit.Space1p5), 0)),
						kit.Rigid(func(gtx C) D { return ui.Kbd{Chord: g.keys.HintFor("theme.next")}.Layout(gtx, th) }),
					)
				})
			}),
		)
	})
}

// pageFrame is the common page chrome: a title row, then the body.
func (g *gallery) pageFrame(gtx C, title string, actions layout.Widget, body layout.Widget) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return layout.Inset{Top: th.Space(kit.Space5), Bottom: th.Space(kit.Space3p5), Left: th.Space(kit.Space6), Right: th.Space(kit.Space6)}.Layout(gtx, func(gtx C) D {
				return kit.Row(gtx,
					kit.Rigid(func(gtx C) D { return th.Heading(gtx, title) }),
					kit.Flexed(1, kit.Grow),
					kit.Rigid(func(gtx C) D {
						if actions == nil {
							return D{}
						}
						return actions(gtx)
					}),
				)
			})
		}),
		kit.Flexed(1, body),
	)
}

func main() {
	shot := flag.String("shot", "", "render offscreen to this PNG and exit")
	pg := flag.String("page", "overview", "page to render with -shot")
	themeFile := flag.String("theme", "", "load a DTCG token file as the theme")
	tokens := flag.Bool("tokens", false, "print the current theme as DTCG tokens and exit")
	flag.Parse()
	g := newGallery()
	if *themeFile != "" {
		if err := g.loadTheme(*themeFile); err != nil {
			fail(err)
		}
	}
	if *tokens {
		if err := writeTokens(os.Stdout, g.th); err != nil {
			fail(err)
		}
		return
	}
	if *shot != "" {
		g.page = *pg
		if err := renderShot(g, *shot); err != nil {
			fail(err)
		}
		return
	}
	a := kit.NewApp("giokit-gallery", "giokit gallery", 1100, 720)
	go func() {
		if err := a.Run(g.Frame); err != nil {
			fail(err)
		}
		os.Exit(0)
	}()
	kit.Main()
}

// fail reports err and exits. Call it only where no deferred cleanup is
// pending.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "gallery:", err)
	os.Exit(1)
}

// loadTheme replaces the theme with the DTCG token file at path.
func (g *gallery) loadTheme(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	th, err := kit.ThemeFromTokens(data, g.th)
	if err != nil {
		return fmt.Errorf("theme %s: %w", path, err)
	}
	g.th = th
	return nil
}

// writeTokens writes th as DTCG tokens, one JSON document and a newline.
func writeTokens(w io.Writer, th *kit.Theme) error {
	b, err := th.Tokens()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
		return fmt.Errorf("write tokens: %w", err)
	}
	return nil
}
