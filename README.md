# giokit

Cross-platform desktop UI toolkit for Go, built on [Gio](https://gioui.org). It
follows GPUI's philosophy: state owned by the UI thread, views redrawn from state
every frame, and tests that simulate real input.

![The giokit gallery](docs/img/gallery.png)

- **`kit/`:** primitives: theme, layout, drawing, focus and keyboard dispatch,
  scrolling, layers, the app runner.
- **`kit/ui/`:** about forty components (buttons, fields, pickers, menus,
  lists, trees, tables, tabs, dialogs, toasts…), each a plain struct.
- **`kit/x/`:** specialised widgets (log view, lyrics).
- **`kittest/`:** headless test harness. It drives real pointer and keyboard input
  through Gio's router, renders offscreen, and catches stale frames, unlabeled or
  tiny or occluded targets, low contrast and missing glyphs automatically.
- **`examples/`:** `hello` (below), `gallery` (every component, with tests),
  `music` (a media app).

```go
type view struct {
	th         *kit.Theme
	count      int
	inc, greet kit.Clickable
	name       widget.Editor
	toasts     ui.Toasts
}

func (v *view) Frame(gtx layout.Context) {
	// 1. Handle input.
	if v.inc.Clicked(gtx) {
		v.count++
	}
	if v.greet.Clicked(gtx) {
		v.toasts.Push(kit.StatusGood, "Hello, "+v.name.Text(), "")
	}
	// 2. Draw from state.
	th := v.th
	kit.Fill(gtx, th.Bg, func(gtx layout.Context) layout.Dimensions {
		return ui.ToastHost{Page: func(gtx layout.Context) layout.Dimensions {
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3, Pad: kit.Space6}.Layout(gtx, th,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Heading(gtx, fmt.Sprintf("Clicked %d times", v.count))
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.Button{ID: "inc", Text: "Click me", Style: ui.Solid}.Layout(gtx, th, &v.inc)
				}),
				kit.Rigid(kit.Fixed(240, func(gtx layout.Context) layout.Dimensions {
					return ui.TextField{ID: "name", Placeholder: "Your name"}.Layout(gtx, th, &v.name)
				})),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.Button{ID: "greet", Text: "Greet"}.Layout(gtx, th, &v.greet)
				}),
			)
		}}.Layout(gtx, th, &v.toasts)
	})
}

func main() {
	v := &view{th: kit.Dark()}
	a := kit.NewApp("hello", "Hello", 480, 320)
	go func() {
		if err := a.Run(v.Frame); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	kit.Main()
}
```

And its test, headless, with real input (`examples/hello/main_test.go`):

```go
v := &view{th: kit.Dark()}
h := kittest.NewT(t, 480, 320, v.Frame)
h.Theme = v.th
h.ClickID("inc")
h.ClickID("inc")
h.ExpectText("Clicked 2 times")
h.ClickID("name")
h.Type("Ada")
h.ClickID("greet")
h.ExpectText("Hello, Ada")
h.ExpectClean()         // no stale frame, unlabeled control, tiny target, low contrast…
h.ExpectTabReachable()  // every control can be reached with Tab
```

```sh
go run ./examples/hello
go run ./examples/gallery
go test ./...
./examples/gallery/web/build.sh  # browser build: copy examples/gallery/web/ to any static host
```

Read [docs/getting-started.md](docs/getting-started.md) next, then the
[reference](docs/components.md).

## Using it from an app

```
require github.com/hhhapz/giokit v0.0.0
replace github.com/hhhapz/giokit => /path/to/giokit   # until there is a tagged release
```

`kit`, `kit/ui`, `kit/x` and `kittest` build on stock Gio (CI checks it:
`scripts/stock-gio.sh`; on Linux build with `-tags novulkan` or install the
Vulkan headers). giokit's Gio fork is optional: it adds Wayland fixes
(cursor scale, even wheel scrolling) and is required only by `kit/x/video`
(streaming images) and `kit/forktrace`. To use it:

```
replace gioui.org => /path/to/giokit/third_party/gioui.org
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## For AI agents

`AGENTS.md` is the guide for working on this repo.
Projects that use giokit should load `skills/giokit/SKILL.md` (Agent Skills
format). Link or copy it into the agent's skills directory, for example
`.claude/skills/`, `.codex/skills/`, `.opencode/skills/` or `.agents/skills/`.
For agents without skills, reference it from that project's `AGENTS.md`.
Detailed docs are in `docs/`.

## Licence

MIT, see `LICENSE`. The vendored Gio in `third_party/gioui.org` keeps its
own licence (Unlicense OR MIT).
