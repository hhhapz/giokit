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

	overviewState
	controlsState
	logsState
	overlaysState
	layoutState
	themeState
	workspaceState
	patternsState
	mediaState
}

func newGallery() *gallery {
	g := &gallery{page: "overview"}
	g.initTheme()
	g.initOverview()
	g.initControls()
	g.initLogs()
	g.initOverlays()
	g.initWorkspace()
	g.initPatterns()
	g.initMedia()
	defaults := map[string]string{"theme.next": "Shortcut+T", "palette.open": "Shortcut+P"}
	for i, p := range pages {
		p := p
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

// navKeys lays out w as one focus group over ids, with Up/Down/Home/End
// moving the current id: what kit's own lists do, built from the public
// FocusGroup, KeyContext and OnAction.
func navKeys(gtx C, grp *kit.FocusGroup, scroll func(i int), id string, ids []string, cur string, set func(string), w kit.W) D {
	i := slices.Index(ids, cur)
	return kit.KeyContext(gtx, "nav", func(gtx C) D {
		move := func(to int) func() {
			return func() {
				to = max(0, min(len(ids)-1, to))
				set(ids[to])
				scroll(to)
			}
		}
		kit.OnAction(gtx, kit.ActMoveUp, move(i-1))
		kit.OnAction(gtx, kit.ActMoveDown, move(i+1))
		kit.OnAction(gtx, kit.ActFirst, move(0))
		kit.OnAction(gtx, kit.ActLast, move(len(ids)-1))
		return grp.Layout(gtx, id, w)
	})
}

// handle reads every input before anything is drawn (rule 1).
func (g *gallery) handle(gtx C) {
	for _, p := range pages {
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
	g.handleTheme(gtx)
	g.handleWorkspace(gtx)
	g.handlePatterns(gtx)
	g.handleMedia(gtx)
}

func (g *gallery) layout(gtx C) D {
	th := g.th
	return ui.CommandPalette{ID: "commands", Actions: &g.actions, Page: func(gtx C) D {
		return ui.ToastHost{Page: func(gtx C) D {
			return th.Modal(gtx, &g.dialog, "dialog", g.window, g.deleteDialog)
		}}.Layout(gtx, th, &g.toasts)
	}}.Layout(gtx, th, &g.palette)
}

// window is the whole window: title bar, sidebar, page, status bar.
func (g *gallery) window(gtx C) D {
	th := g.th
	return kit.Fill(gtx, th.Bg, func(gtx C) D {
		gtx.Constraints.Min = gtx.Constraints.Max
		return kit.Col(gtx,
			kit.Rigid(func(gtx C) D {
				return th.Bar(gtx, 8, 16, func(gtx C) D {
					return kit.Row(gtx,
						kit.Rigid(func(gtx C) D { return th.Text(gtx, "giokit gallery", th.Type.Text, th.Fg, kit.Bold) }),
						kit.Rigid(kit.Gap(12, 0)),
						kit.Rigid(func(gtx C) D { return th.Pill(gtx, "v0", th.Muted, th.Surface) }),
						kit.Flexed(1, kit.Grow),
						kit.Rigid(func(gtx C) D {
							return ui.Button{ID: "theme-next", Text: th.Name, Icon: kit.IconSun, Style: ui.Ghost}.Layout(gtx, th, &g.themeBt)
						}),
						kit.Rigid(kit.Gap(8, 0)),
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
				return th.Bar(gtx, 4, 16, func(gtx C) D {
					s := g.status
					if s == "" {
						s = "Ready"
					}
					return kit.Row(gtx,
						kit.Rigid(func(gtx C) D { return th.Text(gtx, s, th.Type.Small, th.Muted, kit.Regular) }),
						kit.Flexed(1, kit.Grow),
						kit.Rigid(func(gtx C) D { return th.Text(gtx, "Next theme", th.Type.Small, th.Dim, kit.Regular) }),
						kit.Rigid(kit.Gap(6, 0)),
						kit.Rigid(func(gtx C) D { return ui.Kbd{Chord: g.keys.Hint("Next theme")}.Layout(gtx, th) }),
					)
				})
			}),
		)
	})
}

// pageFrame is the common page chrome: a title row, then the body.
func (g *gallery) pageFrame(gtx C, title string, actions kit.W, body kit.W) D {
	th := g.th
	return kit.Col(gtx,
		kit.Rigid(func(gtx C) D {
			return layout.Inset{Top: 20, Bottom: 14, Left: 24, Right: 24}.Layout(gtx, func(gtx C) D {
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
		data, err := os.ReadFile(*themeFile)
		if err == nil {
			var th *kit.Theme
			if th, err = kit.ThemeFromTokens(data, g.th); err == nil {
				g.th = th
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *tokens {
		os.Stdout.Write(g.th.Tokens())
		fmt.Println()
		return
	}
	if *shot != "" {
		g.page = *pg
		if err := renderShot(g, *shot); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	kit.NewApp("giokit-gallery", "giokit gallery", 1100, 720).Run(g.Frame)
}
