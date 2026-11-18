# Getting started

## Set up

Until there is a tagged release:

```
require github.com/hhhapz/giokit v0.0.0
replace github.com/hhhapz/giokit => /path/to/giokit
```

giokit builds on stock Gio. Linux needs the usual Gio dependencies (Wayland
or X11, EGL) and either the Vulkan headers or `-tags novulkan`; macOS and
Windows need nothing extra. Start from `examples/hello`.

Optionally, use giokit's Gio fork for its Wayland fixes (cursor scale, even
high-resolution wheel scrolling), and for `kit/x/video`, which needs it
([gio-patches.md](gio-patches.md)):

```
replace gioui.org => /path/to/giokit/third_party/gioui.org
```

## The frame

Every frame, giokit calls your frame function with a `gtx` (constraints,
input, the clock). The whole UI is drawn again from your state; nothing is
retained between frames except the state you keep.

A frame has two halves, in this order:

1. **Handle input.** Read clicks and events for this frame:
   `if v.save.Clicked(gtx) {...}`, `if id, ok := v.lang.Update(gtx); ok {...}`.
   Change your state.
2. **Draw.** Lay out components from that state.

Drawing first and handling after shows stale state until the next frame;
kittest's strict mode reports it as a stale frame.

## State

Your state is ordinary Go values in your own struct, owned by the UI
goroutine. Component state (a `kit.Clickable`, a `ui.SelectState`) lives
there too, one per control, so it persists across frames. For a list of
rows, keep it in a `kit.Pool[T]` keyed by row id. Background work goes
through `kit.Spawn(app, work, done)` or `app.Post(f)`; `done` and `f` run on
the UI goroutine. `Spawn` returns a `*kit.Task`: `Cancel` it when the result
is no longer wanted.

`App.Run` runs the window on its goroutine and returns the window's error
when it closes; start it on a goroutine and hand the main goroutine to
`kit.Main()`, as `examples/hello` does.

## Components

A component is a struct of props laid out with the theme and its state:

```go
langs := []ui.Choice{{ID: "en", Label: "English"}, {ID: "fr", Label: "Français"}}
ui.Select{ID: "lang", Options: langs, Selected: v.lang}.Layout(gtx, th, &v.langState)
```

Props are values you pass every frame (the selected option's ID lives in
your state, not the component's). Items are `ui.Choice{ID, Label}`: the ID
is stable and is what events report; the label is shown and spoken. Events
come from the state's `Update` in the handle half. Every interactive
component takes an `ID`: tests address it by id, and sub-parts are
`ID/part` (`lang/field`, `lang/en`). The spoken label is the visible text,
or a `Label` you pass for icon-only controls.

Layout uses `kit.Flex` (rows and columns with gaps, padding, a surface) and
the spacing scale `kit.Space0p5`…`kit.Space12` (4dp steps, with half
steps). Colours come from
the theme (`th.Fg`, `th.Panel`, `th.Accent`…); sizes from `th.Metrics`.

## Themes

`kit.Dark()`, `kit.Light()`, `kit.Presets()`, or `kit.Generate(seed)` from an
accent colour (OKLCH, with WCAG contrast enforced). Adjust with
`th.Derive(func(t *kit.Theme) { t.Radius = 999 })`; fonts with
`th.UseFonts(fonts.Inter())`; load and save DTCG token files with
`kit.ThemeFromTokens` and `th.Tokens()`.

## Keyboard

Everything interactive is reachable with Tab and works with Enter/Space.
Composites (lists, trees, tables, tabs, menus) are one Tab stop moved with
the arrow keys. App shortcuts go in a `kit.Keymap` bound to `kit.Actions`;
`ui.CommandPalette` searches them.

## Testing

```go
h := kittest.NewT(t, 800, 600, v.Frame)
h.ClickID("save")
h.ExpectText("Saved")
h.ExpectClean()
```

`kittest` drives real pointer and keyboard input through Gio's router and
renders offscreen (EGL, no display needed). `ExpectClean` runs the audits:
stale frames, controls without labels, tiny or occluded targets, low
contrast, missing glyphs. For background work, pass `h.App()` to
`kit.Spawn` and call `h.RunTasks()`. More in [testing.md](testing.md).
