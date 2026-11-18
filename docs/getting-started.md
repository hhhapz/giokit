# Getting Started

## Setup

Add `giokit` to your `go.mod`:

```go
require github.com/hhhapz/giokit v0.0.0
replace github.com/hhhapz/giokit => /path/to/giokit
```

`giokit` runs on stock Gio across macOS, Windows, Linux, BSD, and WebAssembly. On Linux, either install `vulkan-headers` or pass `-tags novulkan` (OpenGL only), with stock Gio or the fork. The fork prefers Vulkan on Wayland and falls back to OpenGL.

To use the optional Gio fork in `third_party/gioui.org` (needed for `kit/x/video` and `media`; see [gio-patches.md](gio-patches.md)):

```go
replace gioui.org => /path/to/giokit/third_party/gioui.org
```

## Opening a Window

Create a `*kit.App` with `kit.NewApp`, run `app.Run` on a goroutine, and call `kit.Main()` at the end of `main`:

```go
func main() {
	v := &view{th: kit.Dark()}
	app := kit.NewApp("com.example.hello", "Hello", 640, 420)
	go func() {
		if err := app.Run(v.Frame); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	kit.Main()
}
```

The first argument to `kit.NewApp` is the platform application identifier (`"com.example.hello"`). `app.Run` runs the window loop and returns when the window closes; `kit.Main()` hands the main OS thread to the platform event loop.

## The Frame Loop

`giokit` calls your `Frame(gtx layout.Context)` function whenever input arrives, an animation ticks, or background work finishes. The UI is laid out from your Go structs on each frame.

Every frame runs in two steps:

1. Read pending clicks and events, and update your state.
2. Lay out components from that state.

```go
func (v *view) Frame(gtx layout.Context) {
	// 1. Handle input.
	if id, ok := v.langState.Update(gtx); ok {
		v.lang = id
	}
	if v.save.Clicked(gtx) {
		v.saveChanges()
	}

	// 2. Lay out from state.
	th := v.th
	kit.Fill(gtx, th.Bg, func(gtx layout.Context) layout.Dimensions {
		langs := []ui.Choice{
			{ID: "en", Label: "English"},
			{ID: "fr", Label: "Français"},
		}
		return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3, Pad: kit.Space6}.Layout(gtx, th,
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Select{ID: "lang", Options: langs, Selected: v.lang}.Layout(gtx, th, &v.langState)
			}),
			kit.Rigid(func(gtx layout.Context) layout.Dimensions {
				return ui.Button{ID: "save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &v.save)
			}),
		)
	})
}
```

Always handle input before drawing. Laying out a control before checking its input discards unread clicks and draws stale state for the rest of the frame.

## State and Background Work

Application state and component state (`kit.Clickable`, `ui.SelectState`, `widget.Editor`) live on your view struct and belong to the UI goroutine.

For a dynamic list of items where each row needs its own widget state, store that state in a `kit.Pool[T]` keyed by a stable item ID and call `Sweep()` once per frame:

```go
for _, item := range v.items {
	if v.deleteBtns.Get(item.ID).Clicked(gtx) {
		v.removeItem(item.ID)
	}
}
v.deleteBtns.Sweep()
```

Run blocking I/O or background work off the UI goroutine with `kit.Spawn`:

```go
v.searchTask.Cancel()
v.searchTask = kit.Spawn(v.app,
	func(ctx context.Context) ([]Result, error) {
		return searchDatabase(ctx, query)
	},
	func(results []Result, err error) {
		v.results = results
		v.err = err
	},
)
```

`Spawn` runs the work function on a background goroutine and delivers the result to the `done` callback on the UI goroutine before the next frame. Calling `Cancel()` on the returned `*kit.Task` cancels the context and drops the result. You can also post callbacks directly to the UI goroutine with `app.Post(fn)`.

## Components and Layout

A component is a props struct laid out with a theme and a pointer to its state:

```go
ui.Checkbox{
	ID:      "notifications",
	Label:   "Enable desktop notifications",
	Checked: v.notifications,
}.Layout(gtx, th, &v.notificationsClick)
```

Props carry the current value (`Checked`, `Selected`, `Value`); the state pointer holds focus, scroll, and animation state across frames. The zero value of every state struct is ready to use, and state structs must not be copied after their first layout.

Every interactive component takes an `ID` used by tests (`h.ClickID("notifications")`). Composite components name their sub-parts `<ID>/<part>` (`lang/field`, `lang/fr`).

Use `kit.Flex` for row and column layouts with gaps, padding, surfaces, and size constraints:

```go
kit.Flex{
	Axis:   layout.Horizontal,
	Align:  layout.Middle,
	Gap:    kit.Space2,
	Pad:    kit.Space3,
	Bg:     th.Panel,
	Border: th.Border,
	Radius: th.RadiusLg(),
	FillW:  true,
}.Layout(gtx, th,
	kit.Rigid(func(gtx layout.Context) layout.Dimensions {
		return th.Heading(gtx, "Project Settings")
	}),
	kit.Flexed(1, kit.Grow),
	kit.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ui.Button{ID: "reset", Text: "Reset"}.Layout(gtx, th, &v.reset)
	}),
)
```

Spacing uses the `kit.SpaceN` scale (`kit.Space0p5` = 2dp, `kit.Space1` = 4dp, `kit.Space2` = 8dp, up to `kit.Space12` = 48dp) and scales with `th.Density`.

## Themes and Fonts

Build a theme once at startup with `kit.Dark()`, `kit.Light()`, `kit.Presets()`, or `kit.Generate(seed)`. Use `Derive` to adjust a theme for a subtree:

```go
compact := th.Derive(func(t *kit.Theme) {
	t.Density = 0.85
	t.Controls = kit.ControlSmall
	t.Radius = 4
})
```

To switch to another bundled font pair (`fonts.Go()`, `fonts.Inter()`, `fonts.Plex()`, or `fonts.Atkinson()`), call `th.UseFonts(fonts.Inter())` once at startup. Themes also import and export W3C Design Token JSON files with `kit.ThemeFromTokens` and `th.Tokens()`.

## Keyboard Navigation

Interactive controls are reachable with `Tab` and activated with `Enter` or `Space`. Collections (`SelectableList`, `Tree`, `DataTable`, `Tabs`, `Menu`) act as a single `Tab` stop and use arrow keys, `Home`/`End`, and type-ahead to move inside the group.

Application commands go in a `kit.Actions` registry bound to shortcuts through a `kit.Keymap`, and `ui.CommandPalette` provides a searchable modal picker over those actions. In chord strings such as `"Shortcut+K"`, `"Shortcut"` maps to `Cmd` on macOS and `Ctrl` on Windows and Linux.

## Testing

`kittest` drives pointer and keyboard input through Gio's router and renders offscreen on a headless GPU context:

```go
func TestSettingsView(t *testing.T) {
	v := newSettingsView()
	h := kittest.NewT(t, 800, 600, v.Frame)
	h.Theme = v.th

	h.ClickID("lang/field")
	h.ClickID("lang/fr")
	h.ClickRole(kit.RoleButton, "Save")

	h.ExpectText("Saved")
	h.ExpectClean()
	h.ExpectTabReachable()
}
```

See [testing.md](testing.md) for the full `kittest` reference.
