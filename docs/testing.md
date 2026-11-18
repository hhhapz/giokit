# kittest reference

The harness gives you real Gio layout, a real `input.Router`, and GPU rendering
through EGL surfaceless. There's no window and no display, and it runs alongside a
desktop session. If there's no GPU, `NewT` skips the test; with
`KITTEST_REQUIRE_GPU=1` it fails instead (CI sets this). Without a hardware GPU,
`LIBGL_ALWAYS_SOFTWARE=1` runs the suite on Mesa llvmpipe. Custom constructors
that call `kittest.New` should call `kittest.NoGPU(t, err)` on error.

## Setup

| Call | Notes |
|---|---|
| `NewT(t, wdp, hdp, frame)` | Scale 1. Reports via `t`. Saves the final frame to `$KITTEST_SHOTS/<test>.png`. |
| `NewTAt(t, wdp, hdp, scale, frame)` | `NewT` at another scale (HiDPI checks). |
| `New(wdp, hdp, scale, frame)` | For tools such as a `-shot` flag. Findings go to `h.Problems()` (a copy); `Expect*` methods panic. Call `h.Release()` when done. |
| `h.Theme = th` | Enables the missing-glyph audit. |
| `h.Strict` | Default true: re-render after each input to catch stale frames. |
| `h.MinTarget` | Smallest accepted clickable size, default 16dp. |

## Input

Every input call settles, then (in strict mode) runs a stale check.

- **Pointer:**
  - `Move(pt)`, `Click(pt)`, `ClickMods(pt, mods)`, `DoubleClick(pt)`, `RightClick(pt)`, `Scroll(pt, dy)`, `Drag(from, to)`
  - `ByRole(kit.RoleButton, "Save")` returns the id of the one control with
    that role and spoken label (fails on none or several); `ClickRole` clicks
    it. Roles are WAI-ARIA (`RoleTab`, `RoleOption`, `RoleMenuItem`,
    `RoleTreeItem`, `RoleRow`…), recorded by kit components; `h.Role(id)`
    reads one. Tests written this way read like the UI and check labels and
    roles for free.
  - `Hover(id)`, `ClickID(id)`, `ClickIDMods(id, mods)`, `DoubleClickID(id)`, `RightClickID(id)`, `ScrollID(id, dy)`, `DragID(id, dx, dy)`: aim at the visible part of the widget (its
    bounds clipped by ancestors such as a list's viewport); a widget clipped
    out of view fails the test instead of clicking whatever covers the point.
    Scroll it into view first.
- **Async:** pass `h.App()` to `kit.Spawn`/`Post` in the code under test:
  work runs off the UI goroutine as in a real window, so the test sees the
  in-flight state (a spinner, "loading"); `h.RunTasks()` waits for the work,
  its done callbacks, any work they start and any callbacks they post, and
  settles, so the results are drawn. Like a window, a settle keeps drawing
  frames while posted callbacks are queued (`App.Queued`). (A nil app
  still runs work inline.)
- **Keyboard:**
  - `Key(name, mods)`: unhandled Tab/Shift+Tab move focus, like a real window.
  - `Keys(names...)`: several key presses delivered in one frame (a fast
    typist); components must move once per key.
  - `Tab(back)`
  - `Type(text)`: inserts into the focused editor.
- **Cursor:** `Cursor()` returns the cursor the UI requests at the pointer (for
  example, the hand over buttons).

## Time and motion

- `h.Now` is a virtual clock passed as `gtx.Now`. Settle advances it one
  frame (`h.FrameStep`, 1/60 s) per immediate redraw or queued `Post`
  callback, so finite animations (`kit.Tween`) finish within one input call.
  `no-settle` fires after 120 such frames.
- Scheduled redraws (`kit.Loop`, caret blink, timers) are not chased:
  `h.Wait(d)` advances the clock and settles.
- More input: `Drag(from, to)`, `DragID(id, dx, dy)`, `Chord("Shortcut+K")`.
- Only one live harness per test: two share and tear down one EGL display.

## Real-window diagnostics

`GIOKIT_TRACE=1 go run ./examples/gallery 2>trace.log` (or
`GIOKIT_TRACE_FILE=trace.log`) writes one ordered log, each line
`giokit <ms> f<frame> <kind> key=value…`:

- **Pacing:** `frame` (lag behind the frame time), `frame_end` (build and
  submit ms), `pacing` once a second, `LATE_FRAME` (> 1.5× median interval).
  Late frames with a small build time point at pacing or the compositor.
- **Gio window events** (patched hooks, `docs/gio-patches.md`): `wl_axis*`,
  `scroll_flush`, `wl_frame_done`, `wl_draw`, `set_animating`,
  `schedule_timer`, `gpu_frame`, `present`.
- **Lists:** a `list` line on every frame a `kit.List` scrolls, settles, or
  changes items: wheel delta, glide, `pos` (last → after input → after layout),
  `moved` vs `req`, and every visible row as `index@top+height`. Events:
  `list_enter`/`list_leave` (with first-time build cost), `list_resize`,
  `list_relayout`, `list_slow_item` (> 2ms), `list_thumb_jump`. `LIST_SKIP`
  marks frames whose rows moved a different amount than asked, or unevenly
  (`uneven` lists each row's delta).

`go run ./examples/scrolltest` is the hands-on scroll bench: a plain-text
control list beside a component list, each graphing Δ per frame with skips in
red. `C` swaps components for same-height blocks (is it component-linked?),
`U` A/B-toggles Gio's upstream wheel scaling (pitfalls #22), `R` resets.
Set `GIOKIT_WHEEL_MINI_STEP_SCALE=1.5` before launch to try a different
high-resolution wheel sub-step multiplier (default 1.2).

In tests, assert `l.Jumps() == 0` after scrolling (see `TestScrollContinuity`).

## Queries (semantic tree)

- `Find(id)`, `FindText(s)`: pixel bounds.
- `TextsIn(id)`: labels whose origin is inside a widget.
- `Texts()`, `IDs()`: everything on screen.
- `Dump(w)`: the indented tree, with `[click]` marking interactive nodes.

## Pixels

- `Image()`, `Pixel(pt)`, `Save(path)`.
- `BgColor(id)`: samples 3px inside the left edge, at mid height.
- `Near(a, b, tol)`: color comparison with tolerance.

## Assertions (NewT only)

- `ExpectID`, `ExpectNoID`, `ExpectText`, `ExpectTextIn(id, s)`.
- `ExpectBg(id, color)`: ±2 per channel.
- `ExpectClean()`: fails on every Audit finding.
- `ExpectGolden(name)`: compares to `testdata/golden/name.png`. Set
  `KITTEST_UPDATE=1` to write. Goldens are machine-specific; use them sparingly.

## Automatic checks

- **stale-frame:** after an input, one more frame (with no input) changed the
  semantics (ids, labels, class, selected, disabled, bounds, kit roles, focus
  groups, focused id) or pixels. Catches draw-before-handle, late hover reads
  and a missing Notify. The message names each changed node and field, or the
  changed pixel rectangle.
- **no-settle:** still requesting immediate redraws after 120 frames (2s of
  virtual time). With `NewT`, both fail the test, from the first frame on.
- **Audit:**

`Problem{Kind, ID, Bounds, Msg}`; `Kind` is a `ProblemKind` constant
(`ProblemStaleFrame`, `ProblemNoSettle` and the audit kinds below).

| Finding | Meaning |
|---|---|
| unlabeled-clickable (`ProblemUnlabeledClickable`) | No id. Untestable, and silent to screen readers. |
| tiny-target (`ProblemTinyTarget`) | Smaller than MinTarget, usually a collapsed box. |
| occluded (`ProblemOccluded`) | Another interactive area covers the center. |
| duplicate-id (`ProblemDuplicateID`) | Two widgets share an id. |
| missing-glyph (`ProblemMissingGlyph`) | Text the fonts can't draw (tofu). |
| low-contrast (`ProblemLowContrast`) | Enabled text under `MinContrast` (default 4.5, WCAG AA) or `MinContrastLarge` (3, large text) against its background, measured from pixels. 0 turns it off. |

- **Contrast:** `h.Contrasts()` returns every enabled label's measured
  ratio, text color and background. Text inside `th.Disable` is exempt, as
  in WCAG. Thin glyphs at scale 1 never fully cover a pixel, so ink that
  sits 60%+ of the way from the background to a theme color counts as that
  color. **Set `h.Theme` again after switching themes**, or text is judged
  against the old palette.
- **Focus:** `h.FocusedID()` names the focused widget.
  `h.FocusEscape(id, n)` / `h.ExpectFocusTrapped(id, n)` press Tab and
  Shift+Tab n times each and report focus leaving `id` (a modal's trap) as a
  `FocusLeak`.
  `h.ExpectEscapeCloses(id)`.
  `h.TabUnreachable()` visits enabled named controls and returns those Tab
  skips; `h.ExpectTabReachable()` asserts the result is empty. Run it on a
  stable screen. `h.A11yTree()` snapshots IDs, labels, classes, bounds, and
  state as a tree.
- **Layers:** an open popup's panel `X` and backdrop `X:backdrop` may cover
  the page; the audit doesn't call that occlusion. Contrast skips covered
  text. `h.RightClick(pt)` opens context menus.
- **Disabled:** `h.Disabled(id)` reports whether a widget is disabled, itself
  or through an ancestor. `Dump` marks `[disabled]`.

## Visual review (token-cheap)

```go
h.Snap("initial"); h.Hover("row:2"); h.Snap("hover"); h.ClickID("x"); h.Snap("after")
h.SaveSheet(dir+"/sheet.png", 2, 2) // 2 columns, frames halved → one image to read
```

Apps should expose a `-shot` flag that renders via `kittest.New`. See the
gallery, and meetbill's `shot -hover <id>`.

## Recipes

- **Hover:**
  1. Hover row B, then `ExpectBg(B, th.Hover)` and `ExpectBg(A, th.Bg)`.
  2. Hover a child control inside B, and check B is still lit.
  3. Move off the table, and check nothing is lit.
- **Immediate update:** `ClickID(control)`, then assert the model, the control's
  own label, *and* everything derived from it (totals, sidebars). Strict mode
  covers the rest.
- **Persistence:** after the click, reload the store from disk and assert.

## Conformance suite (`kittest/conform`)

Every interactive component has a row in `kit/ui/conform_test.go`:

```go
conform.Case{Name: "Switch", ID: "wifi", Class: semantic.Switch, Role: kit.RoleSwitch, Label: "Wi-Fi",
	Build: conform.Clickable(func(th *kit.Theme, gtx layout.Context, c *kit.Clickable) layout.Dimensions {
		return ui.Switch{ID: "wifi", Label: "Wi-Fi"}.Layout(gtx, th, c)
	})}
```

`Build(th, disabled)` returns a `conform.Control`: a frame (which must wrap
the control in `th.Disable` when asked) and an activation counter. Per row,
subtests check: id, Gio semantic class, the expected kit role, and a spoken label that is not the id; hover changes pixels; one click
fires once; Tab reaches it; Enter and Space activate it (`NoKeys` opts out);
keyboard focus draws a ring; disabled blocks clicks, Tab and keys and says
so; `ExpectClean` under every preset, at scale 1.5 and 2, density 0.8 and
1.2, and 200dp wide. `TestCatchesBrokenControls` proves each check fires on a
broken control (run in a child process, since a failing subtest fails its
parent). Controls that open a layer (menus, selects) run `conform.Composite`
instead (`TestConformanceComposite`): a click and Enter on the trigger open
it, Down focuses the first item, Enter picks once and returns focus to the
trigger, Escape dismisses onto the trigger, and the open layer is clean under
every preset, at scale 2 and in the extreme styles.
`TestCatchesBrokenComposites` proves each step on a broken menu.
Focus-group collections (lists, trees, tables) run `conform.Group`
(`TestConformanceGroup`): one Tab stop on the selected row, Down selects the
next row with one pick, a click selects, disabled blocks Tab, keys and
clicks, clean under presets, scale and styles. `TestComponentCoverage`
(internal/lint) fails when a kit/ui component has no gallery demo or no row
(exemptions carry a reason).

## Lint (`internal/lint`)

`go/ast` checks over `kit`, `kit/ui` and `kit/x`: no `color.NRGBA` literal
outside the token files, no `clip.Stroke`, and two ratchets, geometry
literals (`Radius: 4`, `gtx.Dp(22)`, `kit.Space1 - 1`) and English strings,
whose per-file counts may only go down. Lower the baseline in the same
change (`LINT_BASELINE=1 go test ./internal/lint -v` prints it). The
`*Fixtures` tests prove each rule on snippets it must and must not flag.

## Writing a new check

1. Reproduce the bug as a tiny widget in `kittest/kittest_test.go` and show the
   check fails without the fix.
2. If the bug is detectable from structure (semantics, pixels, frame stability),
   add it to `Audit` or strict mode, so every app gets it for free.
3. Add a one-line rule to `skills/giokit/SKILL.md`, and an entry to
   `docs/pitfalls.md`.
