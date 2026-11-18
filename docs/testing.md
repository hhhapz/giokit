# Testing with kittest

`kittest` runs your `Frame` function through Gio's layout engine, sends pointer and keyboard events through a real `input.Router`, and renders every frame into an offscreen GPU framebuffer (Metal on macOS, Direct3D 11 on Windows, surfaceless EGL on Linux and BSD, and offscreen WebGL on WebAssembly). It opens no windows and requires no display server.

## Creating a Harness

| Call | Behavior |
|---|---|
| `kittest.NewT(t, wdp, hdp, frame)` | Scale `1.0`. Reports failures through `t`, registers `t.Cleanup(h.Release)`, and saves the final frame to `$KITTEST_SHOTS/<TestName>.png` when `KITTEST_SHOTS` is set. Skips if no GPU is available unless `KITTEST_REQUIRE_GPU=1` is set. |
| `kittest.NewTAt(t, wdp, hdp, scale, frame)` | Same as `NewT` at a custom scale factor (such as `1.5` or `2.0`). |
| `kittest.New(wdp, hdp, scale, frame)` | Standalone constructor without `testing.TB`, used for screenshot tools. Collects findings in `h.Problems()`. Call `h.Release()` when done. |

On a headless Linux machine or container without a hardware GPU, set `LIBGL_ALWAYS_SOFTWARE=1` to run on Mesa `llvmpipe`.

Fields on `*kittest.Harness`:
- `h.Theme = th`: attaches the `*kit.Theme` so `Audit` can check for missing font glyphs and measure sub-pixel text contrast against the palette. Set `h.Theme` again if your UI switches themes during a test.
- `h.Strict`: `true` by default. After each input settles, renders one more frame and fails if anything changes.
- `h.MinTarget`: minimum accepted clickable size (default `16` dp).
- `h.MinContrast` and `h.MinContrastLarge`: minimum WCAG contrast ratios for normal text (default `4.5`) and large text (default `3.0`). Set to `0` to disable.

## Input Simulation

Every input call settles immediate redraws and finite animations, and (when `h.Strict` is true) runs a stale-frame check.

### Pointer Input

- By ID: `h.Hover(id)`, `h.ClickID(id)`, `h.ClickIDMods(id, mods)`, `h.DoubleClickID(id)`, `h.RightClickID(id)`, `h.ScrollID(id, dy)`, `h.DragID(id, dx, dy)`. These target the visible bounds of the widget (clipped by ancestor scroll viewports). If the widget is scrolled out of view, `ClickID` fails the test instead of clicking whatever covers that point.
- By WAI-ARIA role and spoken label: `h.ByRole(kit.RoleButton, "Save")` returns the ID of the unique control with that role and spoken label (failing on zero or multiple matches), and `h.ClickRole(kit.RoleButton, "Save")` clicks it.
- By pixel coordinates: `h.Move(pt)`, `h.Click(pt)`, `h.ClickMods(pt, mods)`, `h.DoubleClick(pt)`, `h.RightClick(pt)`, `h.Scroll(pt, dy)`, `h.Drag(from, to)`.
- `h.Cursor()` returns the `pointer.Cursor` requested at the current pointer position.

### Keyboard Input

- `h.Key(name, mods)`: sends a single key press. Unclaimed `Tab` and `Shift+Tab` presses move focus just as in a live window.
- `h.Keys(names...)`: sends multiple key presses in a single frame before settling.
- `h.Chord("Shortcut+K")`: parses and sends a key chord (`"Shortcut"` resolves to `Cmd` on macOS and `Ctrl` on Windows and Linux).
- `h.Tab(back)`: presses `Tab` (`false`) or `Shift+Tab` (`true`).
- `h.Type(text)`: inserts text into the focused `widget.Editor`.

## Virtual Time, Animations, and Async Tasks

`h.Now` is a virtual clock passed to every frame as `gtx.Now`. When an input triggers immediate redraws or a finite animation (`kit.Tween`), `Settle` advances `h.Now` by `h.FrameStep` (default `1/60` s) per frame until the UI stops requesting immediate redraws. If the UI still requests immediate redraws after 120 frames (2 seconds of virtual time), `Settle` fails with `no-settle`.

Scheduled future redraws (`kit.Loop`, spinners, caret blink, timers) are not chased by `Settle`. Call `h.Wait(d)` to advance the clock by `d` and settle.

To test views that start background work with `kit.Spawn` or `app.Post`, pass `h.App()` to your view and call `h.RunTasks()`:

```go
v := newView(h.App())
h.ClickID("load-data")
h.ExpectText("Loading…")

h.RunTasks()
h.ExpectText("Loaded 42 items")
```

`h.RunTasks()` waits for all running `Spawn` tasks, their `done` callbacks, any new tasks they start, and any `Post` callbacks they queue, and then settles the UI.

## Queries and Assertions

### Semantic Tree Queries

- `h.Find(id)` and `h.FindText(s)`: pixel bounds (`image.Rectangle`) and whether the node was found.
- `h.Texts()` and `h.TextsIn(id)`: all text labels on screen, or labels whose origin sits inside `id`.
- `h.IDs()`: all semantic IDs in the last frame.
- `h.Role(id)`: the `kit.Role` recorded for `id`.
- `h.FocusedID()`: the ID of the focused widget (or the active item of a focused `kit.FocusGroup`).
- `h.Disabled(id)`: whether `id` or an ancestor is disabled.
- `h.A11yTree()`: snapshot of the semantic tree (`[]kittest.A11yNode`).
- `h.Dump(os.Stdout)`: prints the indented semantic tree, marking `[click]` and `[disabled]`.

### Pixel Queries and Assertions

- `h.Image()`, `h.Pixel(pt)`, `h.Save(path)`: read or save the rendered RGBA image.
- `h.BgColor(id)`: samples the pixel 3px inside the left edge of `id` at mid-height.
- `h.ExpectID(id)` and `h.ExpectNoID(id)`.
- `h.ExpectText(s)` and `h.ExpectTextIn(id, s)`.
- `h.ExpectBg(id, color)`: checks `BgColor(id)` within ±2 per channel.
- `h.ExpectClean()`: fails on any `Audit` finding.
- `h.ExpectTabReachable()`: presses `Tab` through the screen and fails if any enabled control is skipped.
- `h.ExpectFocusTrapped(modalID, steps)`: presses `Tab` and `Shift+Tab` `steps` times and fails if focus leaves `modalID`.
- `h.ExpectEscapeCloses(id)`: presses `Escape` and checks that `id` is gone.
- `h.ExpectGolden(name)`: compares against `testdata/golden/<name>.png` (`KITTEST_UPDATE=1` writes it).

## Automatic Checks and Audits

### Stale-Frame Check (`h.Strict`)

After each input settles, `kittest` renders one extra frame with no input and compares the semantic snapshot (IDs, labels, classes, roles, selected/disabled flags, focus groups, focused ID, bounds) and pixels against the settled frame. Any difference fails the test with `ProblemStaleFrame`, naming the changed semantic field or pixel rectangle. This catches drawing before handling input, reading hover outside a widget callback, and mutating state mid-frame without calling `kit.Notify(gtx)`.

### Structural and Pixel Audits (`h.Audit()` / `h.ExpectClean()`)

| Kind | Meaning |
|---|---|
| `unlabeled-clickable` (`ProblemUnlabeledClickable`) | Interactive handler has no semantic ID or spoken label. |
| `tiny-target` (`ProblemTinyTarget`) | Clickable area is smaller than `h.MinTarget` (16dp). |
| `occluded` (`ProblemOccluded`) | Another interactive clip area covers the widget's center (open popups and their `:backdrop` layers are exempted). |
| `duplicate-id` (`ProblemDuplicateID`) | Two widgets in the same frame share one semantic ID. |
| `missing-glyph` (`ProblemMissingGlyph`) | Rendered text has runes missing from the bundled fonts (needs `h.Theme`). |
| `low-contrast` (`ProblemLowContrast`) | Enabled text is below `MinContrast` (4.5, WCAG AA) or `MinContrastLarge` (3.0) against the rendered pixels behind it. Disabled controls and `kit.TagImage` artwork are exempt. |

## Contact Sheets

Use `Snap` and `SaveSheet` to combine several interaction steps into a single image for visual inspection:

```go
h.Snap("initial")
h.Hover("row:2")
h.Snap("hover")
h.ClickID("row:2")
h.Snap("selected")
h.SaveSheet("/tmp/sheet.png", 3, 1)
```

## Conformance Suite (`kittest/conform`)

Package `kittest/conform` tests interactive controls against a shared set of behavior, keyboard, accessibility, and styling checks:
- `conform.Run(t, conform.Case{...})` checks leaf controls: ID, semantic class, `kit.Role`, spoken label, hover pixel change (on both `th.Bg` and `th.Panel`), single click, `Tab` reachability, `Enter`/`Space` activation, visible focus ring, disabled behavior, and `ExpectClean()` across theme presets, scales `1.5` and `2`, densities `0.8` and `1.2`, narrow `200dp` width, and pill/sharp/high-contrast theme styles.
- `conform.RunComposite(t, conform.Composite{...})` checks popup-opening controls (`Menu`, `Select`): click and `Enter` open the popup, `Down` moves into the items, `Enter` picks once and returns focus to the trigger, `Escape` closes onto the trigger, and the open popup passes `ExpectClean()`.
- `conform.RunGroup(t, conform.Group{...})` checks focus-group collections (`SelectableList`, `Tree`, `DataTable`): single `Tab` stop, `Down` selects the next row, click selects a row, and `th.Disable` blocks all input.

## Real-Window Tracing (`GIOKIT_TRACE`)

Run an app with `GIOKIT_TRACE=1` to write frame pacing and scroll diagnostics (`frame`, `frame_end`, `pacing`, `LATE_FRAME`, `list`, `LIST_SKIP`) to stderr or `GIOKIT_TRACE_FILE`:

```sh
GIOKIT_TRACE=1 go run ./examples/gallery 2>trace.log
```

When building against the patched Gio fork, importing `_ "github.com/hhhapz/giokit/kit/forktrace"` also logs low-level window and scroll events. `go run ./examples/scrolltest` opens an interactive scroll benchmark window.
