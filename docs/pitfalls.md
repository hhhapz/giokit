# Pitfalls: what went wrong building the first giokit app, and why

Each entry has four parts:
- **Symptom:** what the user saw.
- **Cause:** what actually went wrong.
- **Rule:** how to avoid it.
- **Detection:** which kittest check catches it.

The first app (meetbill) shipped several of these, and its author believed it was
correct because screenshots looked fine. Static screenshots never exercise input.

## 1. Clicks lag ("ridiculous delay")
- **Symptom:** clicking a stepper did nothing until the mouse moved again.
- **Cause:** each frame snapshotted state, drew it, and handled clicks *afterwards*.
  The frame showed old data, and Gio only redraws on events or invalidation, so
  nothing redrew.
- **Rule:** `Frame` = handle, then layout. Mutate the model the UI goroutine owns
  directly. `kit.Clickable` notifies on click, so anything drawn earlier in the
  frame also refreshes.
- **Detection:** strict mode (stale-frame). Test: `TestCatchesDrawBeforeHandle`.

## 2. Hover highlights the wrong row
- **Symptom:** the row highlight trailed the cursor. It showed on the row just left.
- **Cause:** `hovered := c.Hovered()` was read before `c.Layout` processed this
  frame's pointer events, so the value was one event old. No further frame
  followed.
- **Rule:** read state only inside the callback (`Press`, `Hover.Layout`), which
  run after the widget's `Update`.
- **Detection:** strict mode. Test: `TestCatchesLateHoverRead`.

## 3. Lost clicks
- **Symptom:** clicks silently ignored.
- **Cause:** `widget.Clickable.Layout` consumes and **discards** pending clicks.
  `Clicked` called after layout never sees them.
- **Rule:** call every `Clicked` in the handle pass, before layout.
- **Detection:** interaction tests that assert the model changed.

## 4. Checkboxes as a single corner, icon buttons as slivers
- **Cause:** fixed-size boxes were drawn as `layout.Stack` children. Stack resets
  `Min` for Stacked children, so an empty box returned zero size and only its
  stroke corner drew.
- **Rule:** `kit.Box(gtx, size, surface, w)` always reports its size.
- **Detection:** Audit (tiny-target, MinTarget 16dp). Test:
  `TestCatchesTinyTarget`.

## 5. "Right-aligned" buttons sitting next to the title
- **Cause:** `layout.Flexed(1, func(gtx) D { return D{} })`. A zero-size flexed
  child doesn't claim the space.
- **Rule:** use `kit.Flexed(1, kit.Grow)`.
- **Detection:** visual only. Render and look.

## 6. Table columns not under their headers
- **Cause:** material.List's default scrollbar *occupies* width, so rows got
  narrower than the header and every flexible column shifted.
- **Rule:** use `th.List` (overlay scrollbar), and give the header and the rows the
  same horizontal inset.
- **Detection:** compare `Find` bounds of a header label and a cell, or look at a
  sheet.

## 7. Tofu boxes (↻ rendered as □)
- **Cause:** the Go fonts lack many symbols. Worse, Gio falls back to *system*
  fonts by default, so on the dev box ✓ rendered fine, but it is tofu on other
  machines.
- **Rule:** `kit.New` disables system fonts. Draw symbols with `kit.Icon`.
- **Detection:** Audit (missing-glyph, needs `h.Theme`). Test:
  `TestCatchesMissingGlyph`.

## 8. Hover areas invisible to tests
- **Cause:** Gio's `pointerQueue.Frame` sets `semantic.valid = gestures != 0` for
  every area with a handler. A hover-only handler (no click or scroll) wipes that
  area's semantic description.
- **Rule:** `kit.Hover` puts the id on a nested, handler-free clip area. Do the
  same in any custom hover-only widget.
- **Detection:** `Find` fails. Run `h.Dump`.

## 9. Semantic quirks that break naive queries
- **Text labels:** they report unbounded heights (Max.Y ≈ 1e6), so containment
  checks fail. Match on the origin, as `TextsIn` does.
- **`AppendSemantics`:** returns the root followed by a flattened copy of every
  descendant. Walk from `[0]` only (`nodes()`), or every match appears twice.

## 10. Cursor tip drifts from the real pointer (Wayland, scaled output)
- **Symptom:** the hand cursor "teleports" ~9px when entering a button. Boxes feel
  misaligned, but the hit areas are correct.
- **Cause:** Gio loads the cursor theme once at the creation-time scale. After
  moving to a scale-2 output (niri at 1.25 reports 2), it divides hotspots by 2
  while still drawing 1× images.
- **Rule:** keep the `third_party/gioui.org` patch (`reloadCursors`).
- **Detection:** none headless; the compositor draws the cursor. Ask the user.

## 11. Redraw loops
- **Cause:** calling `Notify` unconditionally every frame burns CPU forever.
- **Detection:** Settle reports no-settle after 10 frames. Test:
  `TestCatchesRedrawLoop`.

## 12. Background work racing the UI
- **Cause:** an early version guarded state with a mutex and drew from snapshots,
  which made bug 1 possible.
- **Rule:** one owner. Results come back via `kit.Spawn` / `app.Post`.

## 13. Overlay shows a frame late (layout.Stack order)
- **Cause:** the LogView "jump to latest" button was a Stacked child; Stack
  lays those out *before* Expanded ones, so it read the list's scroll state
  before the list applied the scroll.
- **Rule:** draw state-dependent overlays after their content.
- **Detection:** strict mode. Test: `TestCatchesStackOrderLateRead`.

## 14. Scrollbar drags drawn a frame late
- **Cause:** material.List applies scrollbar deltas after laying out the list.
- **Rule:** `th.List` (kit's scrollbar) applies them before layout.

## 15. Tab dead whenever a spinner was on screen (harness bug)
- **Cause:** the stale check drew a frame without consuming the router's
  wakeup; a scheduled redraw made the next Tab look "handled".
- **Rule:** the harness consumes the wakeup after every frame, as a window does.
- **Detection:** `TestLoopSettlesAndTabStillWorks`.

## 16. Thin drag handles flagged as tiny targets
- **Cause:** any press handler (drags included) is a click gesture.
- **Rule:** handles and scrollbars get a 16dp hit area; the visible line
  stays thin.

## 17. Split divider ran away from the pointer (~2.5×)
- **Cause:** Gio reports drag positions relative to the handle *as it was at
  press time*; adding them to the handle's current position compounded.
- **Rule:** anchor drags to the press-time position (`kit.Split` does).
- **Detection:** `TestSplitDragAndKeys` asserts the divider moves exactly
  with the pointer. A "moved right" check missed it: assert amounts.

## 18. Wheel scrolling skipped (Wayland)
- **Cause:** Gio turns each wheel notch into one instant ~150px jump.
  Headless deltas are small, so tests looked smooth.
- **Rule:** `th.List` takes a `*kit.List` and glides large deltas; small
  deltas under 8px apply at once. This also glides ~9px mini-steps from a
  high wheel multiplier. A glide sets `BeforeEnd` so it can leave
  a `ScrollToEnd` tail.
- **Detection:** `TestWheelGlides`.

## 19. Focused slider couldn't take the wheel (overlay stole it)
- **Cause:** kit.List's wheel layer was drawn above the rows with a PassOp,
  so it claimed every scroll before nested widgets; bubbling ran backwards.
- **Rule:** the list's wheel handler is the rows' ancestor, and layout.List
  runs with input disabled (rows get the source back) so its own handler
  never registers. The range stops at the list's ends (scroll chaining).
- **Detection:** `TestSliderWheelOnlyWhenFocused`.

## 20. Scroll stalled 150–200ms when a spinner entered or left view
- **Symptom:** the scroll glide "skipped" as content entered or left view.
  The harness showed continuous positions and cheap frames.
- **Diagnosis:** `GIOKIT_TRACE=1` showed 165–190ms gaps between frames whose
  build took 1–2ms (outside the app), and 17.4ms pacing (a timer, not vsync)
  while the spinner was visible.
- **Cause (suspected, confirm on hardware):** `kit.Loop` asked for "now+16ms",
  so Gio switched to timer pacing. The glide's immediate requests switched it
  back to vsync, and toggling between the two stalled the window.
- **Rule:** continuous animations ask for "frame time + 1ns": vsync-paced in a
  window, not chased by kittest.
- **Result:** after the fix, the trace showed a steady 16.7ms during scrolling
  (late frames only at startup).
- **Tool:** `GIOKIT_TRACE=1` separates app cost (build time) from pacing (late
  frames with small builds), and logs list skips: rows moving by the wrong
  amount, or unevenly, during a scroll gesture.

## 21. Strokes ate half the frame
- **Cause:** Gio turns every `clip.Stroke` (borders, rings, icons) into
  geometry on the CPU every frame, with no cache. On Controls that was
  ~1.5 of 2.9 ms.
- **Rule:** draw outlines as filled rings (`kit.Border`: outer contour minus
  reversed inner) and icons as quads plus round dots in one path (nonzero
  union).
- **Detection:** `BenchmarkFrame` in the gallery; profile `collectOps`.

## 22. High-resolution wheel skipped ~17px once per click (Wayland)
- **Symptom:** while slowly wheel-scrolling Controls, content skipped "as a
  section entered or left view". Same report as #20, which fixed a real
  stall but not this. kittest (and `List.Jumps`) saw nothing.
- **Diagnosis:** the `GIOKIT_TRACE` log interleaves Gio's raw `wl_axis*`
  events with `scroll_flush` and the list's rows. The list moved exactly as
  asked; `scroll_flush` was wrong: `wl_axis value=1.88` +
  `wl_axis_discrete=1` → `dy=18.75`.
- **Cause:** Gio upstream scales a pointer frame's distance ×10 only when it
  carries a discrete step. A high-res wheel sends an axis event every 1/8
  click (1.875) and the discrete step only on the 8th, so every 8th event
  scrolled 10× its neighbours. The coincidence with content entering view
  was the eye, not the cause.
- **Rule:** scale a frame ×10 only if it carries a whole click, not because it
  had a discrete step; high-res wheel sub-steps use the same ×1.2 factor (Gio patch,
  `docs/gio-patches.md`). A first fix scaled every wheel sub-step ×10 instead:
  even, but 5× faster per click, and rejected. Input bugs below `pointer.Event` are
  invisible to kittest; check the trace's raw events against `scroll_flush`.
- **Detection:** `TestWheelScaleHighResEven` in `third_party/gioui.org/app`.

## 23. Disabled controls vanished from the semantic tree
- **Symptom:** with `th.Disable`, the save button, checkbox, tabs and slider
  had no ids (tests couldn't find them; screen readers wouldn't either). The
  text field kept its id.
- **Cause:** Gio drops semantics from an area whose handler asks for no
  gestures, and a disabled handler asks for none. The field's id sat on a
  handler-free area, so it survived.
- **Rule:** a disabled component tags a nested, handler-free area
  (`disabledArea`), as `kit.Hover` already does.
- **Detection:** `TestDisabledFormTakesNoInput` (`h.Disabled(id)` fails on
  a missing id).

## 24. kittest typed text in front of the caret
- **Symptom:** typing mid-field in a test inserted text but left the caret
  before it, so the next keystroke landed in the wrong place. Typing at the
  end of a field (every earlier test) hid it.
- **Cause:** `h.Type` sent only the `EditEvent`; a real window's
  `EditorInsert` follows it with a `SelectionEvent` after the inserted text.
- **Rule:** harness input must replay what `app.Window` sends, event for
  event. Read `app/window.go` when adding an input helper.
- **Detection:** `TestTextFieldNativeKeys`.

## Process lessons
- **Screenshots aren't tests.** Drive real input (`kittest`) and assert on the
  model, the semantics and the pixels.
- **Debug Gio from its source.** When behavior is surprising, read
  `$(go env GOMODCACHE)/gioui.org@…`. Two bugs above were found only there.
- **Break it on purpose.** After fixing a bug, re-introduce it and confirm the test
  fails. Otherwise you don't know the test works.
- **Separate draw from handle in tests too.** `Settle` never synthesizes input, so
  "one click, then correct" is meaningful.
