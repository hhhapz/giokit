---
name: giokit
description: Build, debug, or test Go desktop GUIs with giokit (Gio-based, GPUI-style). Use when writing or changing a giokit/Gio UI, adding components, fixing hover/click/redraw bugs, or verifying a UI headlessly with kittest.
---

# giokit

Immediate-mode Go UI on Gio. Every frame redraws the whole UI from app state.
Docs: `$(go list -m -f '{{.Dir}}' github.com/hhhapz/giokit)/docs/`. Read only what the task needs:
`architecture.md`, `components.md`, `testing.md`, `pitfalls.md` (when debugging).

## Rules (each one is a bug that shipped)

1. **Frame = handle, then layout.** Read every `Clicked` and `Update` first, mutate state, then draw. Drawing first shows stale state until the next mouse move, and `Clickable.Layout` drops unread clicks.
2. **Read hover/press only inside the component callback** (`th.Press(gtx, c, id, func(gtx, st kit.State) layout.Dimensions)`, `kit.HoverArea.Layout`). Reading hover before layout is one event stale.
3. **Mid-frame mutation requires `kit.Notify(gtx)`.** `kit.Clickable` does this automatically for clicks.
4. **The UI goroutine owns all state.** No mutexes or snapshots. Run background work with `kit.Spawn(app, work, done)` or `app.Post`.
5. **Every interactive or scrollable widget gets a stable semantic id** (`kind:recordID`). Sub-parts use `kit.Part(id, "part")` (`id/part`), and `kit.Scope(gtx, name, w)` prefixes nested IDs. Screen readers announce the human label (`kit.TagLabel`), never the id.
6. **Fixed sizes use `kit.Box` or `kit.Fixed`.** `layout.Stack` resets `Constraints.Min` for `Stacked` children, causing empty boxes to collapse.
7. **Flex spacer is `kit.Flexed(1, kit.Grow)`.** A zero-size `Flexed` child claims no space.
8. **Icons are vectors (`kit.Icon`), not Unicode symbols.** Bundled fonts lack `✓ ↻ ▾ ▸`, and system font fallback is disabled for determinism.
9. **Lists use `th.List` with a `*kit.List`** (or `ui.SelectableList` for selection), providing an overlay scrollbar and smooth wheel glide. Gio's default scrollbar steals layout width and misaligns table columns.
10. **Animations:** Use `kit.Tween.To(gtx, target, th.Dur(d))` for finite transitions and `kit.Loop` for continuous animations (spinners). Never call `Notify` unconditionally every frame.
11. **`layout.Stack` lays out `Stacked` children before `Expanded` ones.** Draw state-dependent overlays after their content in sequential code.
12. **`kit.Tag` names the current clip area.** Push a clip rect first, or the tag names the entire window.
13. **Input bubbles:** The innermost widget registering for a pointer event claims it; unclaimed keys resolve after layout from the focused widget outward through `kit.KeyContext` scopes (`kit.OnAction`, `Keymap.Attach`). Collections use `kit.FocusGroup` as a single `Tab` stop.
14. **Outlines are fills, never `clip.Stroke`.** Use `kit.Border` and filled icon polygons; `clip.Stroke` re-tessellates on the CPU every frame.
15. **Disabled is `th.Disable(gtx, cond, w)`,** never just a gray color: it blocks input, removes controls from `Tab` order, and sets disabled semantics.
16. **Floating surfaces use kit overlays** (`Popover`, `Menu`, `Select`, `ContextMenu`, `Modal`, `ToastHost`), never raw `op.Defer`: they flip at window edges, trap focus, and dismiss on `Escape`.
17. **Import Gio from giokit's fork, `github.com/hhhapz/giokit/gio/...`,** never `gioui.org` (two Gios cannot link). Linux builds need Vulkan headers, or `-tags novulkan`.
18. **Custom buttons must be Tab-reachable:** Give icon-only controls a human label (`TagLabel` or `ui.IconButton{Label: ...}`) and verify with `h.ExpectTabReachable()`.
19. **Alternative text is not painted text:** Tag artwork with `kit.TagImage(gtx, id, label)` (`RoleImage`) so screen readers announce it and the pixel contrast audit skips it.
20. **Spacing comes from the `kit.SpaceN` scale** (`kit.Space2` = 8dp, `kit.Space3` = 12dp) passed through `th.Space(...)` or `kit.Flex{Gap, Pad}` so `th.Density` applies.
21. **Components live in `kit/ui` as props structs:** `ui.Combobox{ID: "city", Options: opts}.Layout(gtx, th, &v.city)`. Read events via `v.x.Update(gtx)` in the handle pass. Props hold controlled values; state structs hold interaction state only.
22. **Streaming images (video, camera, remote screens) use `paint.Stream` + `video.Video`,** never `paint.NewImageOp` per frame. Submit frames from any goroutine and wire `stream.OnFrame = app.Invalidate`.
23. **Call every conditional component's `Update` on every frame,** even while hidden, after the input that toggles its visibility; otherwise it appears one frame late (`ProblemStaleFrame`).
24. **Custom state `Update` methods run outside `th.Disable`:** Record `!gtx.Enabled()` during `Layout`, register no input handlers when disabled, and ignore events in `Update`.

## Verify Every UI Change Headlessly

```go
h := kittest.NewT(t, 1100, 720, v.Frame)
h.Theme = v.th
h.Hover("row:2")
h.ExpectBg("row:2", th.Hover)
h.ClickID("qty:2/plus")
h.ExpectTextIn("row:2", "1.5")
h.ExpectClean() // Audits: unlabeled controls, tiny targets, occlusion, duplicate IDs, missing glyphs, contrast
```

- **Custom controls:** Add a `conform.Case` (`kittest/conform`) and run `conform.Run(t, ...)`. Layer-opening controls use `conform.RunComposite`; collections use `conform.RunGroup`.
- **Background work:** Pass `h.App()` to `kit.Spawn`/`Post` and call `h.RunTasks()` to wait for tasks and callbacks to settle.
- **Strict mode (`h.Strict`):** Enabled by default; re-renders after each input and fails if anything changes on the extra frame (catching rules 1 to 3 automatically).
- **Visual inspection:** Call `h.Snap(label)` at key steps and `h.SaveSheet(path, cols, scale)` to inspect a contact sheet PNG, or run `KITTEST_SHOTS=dir go test`.
- **Debugging missing IDs:** Run `h.Dump(os.Stdout)`.
- **Virtual clock:** `Settle` finishes finite animations automatically; use `h.Wait(d)` for timers (tooltips, toasts).
- **Out of reach headlessly:** OS compositor cursor rendering and physical monitor scaling; ask the user to verify those on a desktop session.
