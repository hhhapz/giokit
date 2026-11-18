---
name: giokit
description: Build, debug, or test Go desktop GUIs with giokit (Gio-based, GPUI-style). Use when writing or changing a giokit/Gio UI, adding components, fixing hover/click/redraw bugs, or verifying a UI headlessly with kittest.
---

# giokit

Immediate-mode Go UI on Gio. Every frame redraws the whole UI from app state.
Docs: `$(go list -m -f '{{.Dir}}' github.com/hhhapz/giokit)/docs/`. Read only what the task needs:
`pitfalls.md` (when debugging), `testing.md`, `components.md`, `philosophy.md`.

## Rules (each one is a bug that shipped)

1. **Frame = handle, then layout.** Read every `Clicked` first, then mutate state,
   then draw. Drawing first shows stale state until the next mouse move.
   `Clickable.Layout` drops unread clicks.
2. **Read hover/press only inside the component callback**
   (`th.Press(gtx, c, id, func(gtx, st kit.State) layout.Dimensions)`, `kit.HoverArea.Layout`). A value read before
   layout is one event old, so the highlight lands on the wrong row.
3. **Mid-frame mutation → `kit.Notify(gtx)`.** `kit.Clickable` does this for you.
4. **UI goroutine owns state.** No mutexes or snapshots. Background work goes
   through `kit.Spawn(app, work, done)` or `app.Post`.
5. **Every interactive or scrollable widget gets a semantic id** (`kind:recordID`). Ids are paths: components name their parts `id/part` (`kit.Part`), `kit.Scope(gtx, name, w)` prefixes everything inside, `/` is escaped in segments.
   Tests find it by id; screen readers announce its label (`kit.TagLabel`), never the id.
6. **Fixed sizes use `kit.Box` / `kit.Fixed`.** Stack resets Min for Stacked
   children, so "fixed" boxes collapse to slivers.
7. **Flex spacer is `kit.Flexed(1, kit.Grow)`.** A zero-size Flexed child claims
   nothing.
8. **Icons are vectors (`kit.Icon`), not symbol characters.** The bundled fonts
   lack ✓ ↻ ▾ ▸. System fonts are disabled on purpose.
9. **Lists use `th.List` with a `*kit.List`** (or `ui.SelectableList` for selection) (overlay scrollbar, smooth wheel). The default scrollbar steals width
   and misaligns columns.
10. **Animations:** `kit.Tween.To(gtx, target, th.Dur(d))` for finite ones,
    `kit.Loop` for continuous ones (spinners). Never `Notify` every frame.
11. **`layout.Stack` lays out Stacked children before Expanded ones:** an
    overlay that depends on the content's state is drawn after it in plain
    code.
12. **`kit.Tag` names the current clip area:** push one first, or it names the
    whole window.
13. **Input bubbles, like the DOM:** the innermost widget that *asks* for a
    pointer event gets it; keys nobody claimed during layout resolve after it,
    from the focused widget out through `kit.KeyContext`s (`kit.OnAction`,
    `Keymap.Attach`). Composites are one Tab stop: `kit.FocusGroup`.
14. **Outlines are fills, never `clip.Stroke`:** use `kit.Border` and the icon
    helpers. Gio re-flattens strokes on the CPU every frame (it was half the
    frame).
15. **Disabled is `th.Disable(gtx, cond, w)`,** never a gray color: it also
    blocks input, leaves the Tab order and tells screen readers.
16. **Floating things are kit overlays** (`Popover`, `Menu`, `Select`,
    `ContextMenu`, `Modal`, `ToastHost`), never a hand-rolled `op.Defer`:
    they place against window edges, trap focus, and close on Escape.
17. **giokit builds on stock Gio** (Linux: `-tags novulkan` or the Vulkan
    headers). The fork (`replace gioui.org => <giokit>/third_party/gioui.org`)
    is optional (Wayland cursor and wheel fixes; Vulkan opt-in) and required
    only by `kit/x/video` and `kit/forktrace`.
18. **Custom buttons must be Tab reachable:** give icon-only controls a human label with `TagLabel` or `ui.IconButton{Label: ...}`, then check `h.TabUnreachable()`.
19. **Alternative text is not painted text:** tag artwork with `kit.TagImage(gtx, id, label)` (role image): the label is spoken, and the contrast audit skips its pixels.
20. **Spacing comes from the scale:** `kit.Space2` (8dp), `kit.Space3` (12dp)…
    through `th.Space(...)` or a `kit.Flex{Gap, Pad}`, so Density applies. One
    styled `kit.Flex` replaces Col + Gap + Pad + Surface nesting.
21. **Components live in `kit/ui` and are props structs:**
    `ui.Combobox{ID: "city", Options: opts}.Layout(gtx, th, &v.city)`; read
    events with `v.x.Update(gtx)` in the handle pass. Props are the value
    (controlled); the state struct holds interaction state only.
22. **Changing images (video, camera, remote screens) use `paint.Stream` +
    `video.Video`,** never `paint.NewImageOp` per frame (a new texture and
    mipmaps every frame). Submit from any goroutine; `OnFrame` → `app.Invalidate`.
23. **Call every conditional component's `Update` every frame,** even while
    it is hidden, after the input that decides whether it shows: otherwise,
    on the frame it appears, its handler asked for no events and Gio hides
    its semantics, so it shows up a frame late (strict mode: stale frame).
24. **A state's `Update` runs outside `th.Disable`:** remember at layout
    whether it was disabled (`!gtx.Enabled()`), register no handlers then,
    and ignore input in `Update`, or a disabled control still reacts
    (`kit.Clickable` does this; a custom state must too).

## Verify every UI change headlessly (no window needed)

```go
h := kittest.NewT(t, 1100, 720, v.Frame)
h.Theme = v.th
h.Hover("row:2")
h.ExpectBg("row:2", th.Hover)
h.ClickID("qty:2/plus")
h.ExpectTextIn("row:2", "1.5")
h.ExpectClean() // audit: ids, tiny targets, occlusion, duplicate ids, tofu, contrast
```

- **Custom controls:** add a `conform.Case` (`kittest/conform`) and
  `conform.Run(t, ...)`: role, label, hover, click, Tab, keys, ring, disabled,
  presets, HiDPI. Layer openers use `conform.Composite`, collections
  `conform.Group`.
- **Background work in tests:** pass `h.App()` to `kit.Spawn`/`Post`, then
  `h.RunTasks()` waits for the work and its callbacks.
- **Strict mode is on by default.** After each input the harness re-renders, and
  any change means the frame was stale (catches rules 1–3 automatically).
- **Look at the UI:** `h.Snap(label)` …, then `h.SaveSheet(path, 2, 2)`, then read
  the one PNG. `KITTEST_SHOTS=dir go test` saves each test's final frame.
- **When `Find` fails:** run `h.Dump(os.Stdout)`.
- **Time is virtual:** Settle runs animations to the end; use `h.Wait(d)` for
  delays (tooltips, streams). `h.Chord("Shortcut+1")` and `h.DragID` drive
  keymaps and splits.
- **Out of reach headlessly:** compositor cursor rendering and real-monitor scaling.
  Ask the user to check those.
