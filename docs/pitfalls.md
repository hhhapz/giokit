# Pitfalls

Common issues encountered when building on Gio and `giokit`, why they happen, and how to avoid them.

## 1. Clicks lag by one interaction
Handling clicks after laying out the UI draws pre-click state for the current frame. Because Gio only redraws on input events or explicit invalidation, the updated state stays invisible until the next pointer event. Always handle input at the top of `Frame` before laying out widgets (`kittest` strict mode checks for this after every input).

## 2. Hover highlights trail behind the cursor
Reading `c.Hovered()` before `c.Layout(gtx)` processes the current frame's pointer events returns the previous frame's state. Read hover and press state only inside the layout callback of `th.Press` or `kit.HoverArea.Layout`, which run after pointer events are processed.

## 3. Lost clicks
`widget.Clickable.Layout` and `kit.Clickable` drain unread clicks during layout. Poll every `Clicked(gtx)` and `state.Update(gtx)` in the handle pass before laying out the frame.

## 4. Fixed-size controls collapsing to slivers
`layout.Stack` resets `Constraints.Min` to zero for `Stacked` children, causing empty fixed-size boxes to return `0×0` dimensions. Use `kit.Box(gtx, size, surface, w)` or `kit.Fixed(width, w)` so fixed-size elements always report their dimensions (`kittest`'s `tiny-target` audit flags interactive targets smaller than `16dp`).

## 5. Right-aligned buttons sitting next to the title
In `layout.Flex`, a `Flexed` child that returns zero width (`layout.Dimensions{}`) does not consume the space allocated to it. Use `kit.Flexed(1, kit.Grow)` for spacers.

## 6. Table columns misaligned with their headers
`material.List`'s default scrollbar takes horizontal width inside the list body, making rows narrower than a fixed header outside the list. Use `th.List` (which draws an overlay scrollbar) and apply matching horizontal padding to headers and rows.

## 7. Missing glyphs rendering as tofu boxes (`□`)
Gio's default system font fallback can mask missing Unicode symbols on a developer's machine while rendering `□` on machines without that font. `kit.New` disables system font fallback (`text.NoSystemFonts()`); draw UI symbols as vector icons with `kit.Icon`.

## 8. Hover-only and keyboard-only areas missing from the semantic tree
Upstream Gio's pointer router discards semantic ops on any clip area whose handler registers for zero click or scroll gestures (hover-only, key-only, disabled). The fork keeps them ([gio-patches.md](gio-patches.md)), so `kit.Tag` works on any area. `kit.TagArea` only gives an area to content that has none.

## 9. Semantic label bounds and duplicate traversal nodes
In upstream Gio, unclipped `widget.Label` areas extend to `Max.Y ≈ 1e6`, and `Router.AppendSemantics` returns the root node at `[0]` followed by a flat copy of every descendant. The fork clips labels to their laid-out text ([gio-patches.md](gio-patches.md)), `TextsIn(id)` matches on label origin, and semantic tree walks start from `roots[0]` only.

## 10. Wayland cursor tip drifting on scaled displays
Upstream Gio loads the Wayland cursor theme once at window creation scale and divides cursor hotspots by the current output scale in `setCursor`, leaving 1× cursor bitmaps with halved hotspots on 2× displays. The `0002-wayland-cursor-wheel-trace` patch reloads the cursor theme when output scale changes.

## 11. Redraw loops burning CPU
Calling `kit.Notify(gtx)` unconditionally on every frame keeps the window rendering at 100% CPU. Call `Notify` only when state changes mid-frame (`kittest`'s `Settle` fails with `no-settle` after 120 consecutive immediate-redraw frames). Use `kit.Tween` for finite transitions and `kit.Loop` for continuous animations.

## 12. Background goroutines racing with layout
Mutating view fields from background goroutines under a mutex or drawing from partial snapshots causes data races and stale frames. Keep all view and theme state on the UI goroutine and deliver background results through `kit.Spawn` or `app.Post`.

## 13. Overlays appearing one frame late (`layout.Stack` order)
`layout.Stack` lays out `Stacked` children before `Expanded` ones. If an overlay button is a `Stacked` child that checks scroll state on an `Expanded` list, it reads the state before the list processes the current frame's scroll input. Lay out state-dependent overlays after their content in sequential code.

## 14. Scrollbar thumb drag lagging by one frame
`material.List` applies scrollbar drag deltas after laying out the list children. `th.List` applies scrollbar deltas before laying out the list.

## 15. Unconsumed router wakeups in headless tests
A headless harness must call `router.WakeupTime()` after every frame just as `app.Window` does; leaving a scheduled wakeup unconsumed alters how the router processes subsequent `Tab` key events.

## 16. Thin resize handles flagged as tiny hit targets
Any pointer press or drag handler registers an interactive target on its clip area. Give split-pane dividers and scrollbars a `16dp` (`th.HitSize`) interactive clip area while drawing the narrow visual line centered inside it.

## 17. Split divider running away from the pointer
Gio reports pointer drag coordinates relative to the handler's clip area as positioned at the start of the press. Adding a drag delta to the divider's current position compounds the movement each frame; always anchor drags to the press-time position (`ui.Split`).

## 18. Discrete mouse-wheel scrolling jumping in large steps
Upstream Gio converts each discrete wheel notch into an instant ~150px jump. `th.List` glides wheel deltas >= 8px over a short ease curve while applying small touchpad deltas immediately.

## 19. Scroll container stealing wheel events from a focused slider
Placing a list's wheel handler in an overlay above its rows with `pointer.PassOp` causes the list to claim wheel events before nested child controls can read them. `th.List` registers its scroll handler on the ancestor container around the rows and bounds its scroll range to the remaining scrollable distance so focused child widgets receive wheel input first.

## 20. Scroll stalling 150–200ms when a spinner entered or left view
Scheduling `kit.Loop` wakeups at `gtx.Now.Add(16ms)` caused Gio's window loop to switch from vsync pacing to timer pacing, stalling when a scroll glide simultaneously requested immediate vsync frames. `kit.Loop` schedules wakeups at `gtx.Now.Add(1)` (1ns after the frame timestamp), which stays vsync-paced in a window without being chased by `kittest`'s `Settle`.

## 21. Strokes ate half the frame
Gio tessellates `clip.Stroke` paths on the CPU on every frame without caching. `giokit` draws borders as filled outer-minus-inner contour rings (`kit.Border`) and icons as filled vector polygons, and forbids `clip.Stroke`.

## 22. High-resolution wheel skipped ~17px once per click (Wayland)
High-resolution mouse wheels on Wayland send eight `wl_pointer.axis` events of `1.875` units per detent and attach `wl_pointer.axis_discrete = 1` only to the 8th event. Upstream Gio multiplies a scroll frame by `10` whenever `axis_discrete` is non-zero, making every 8th sub-step jump 10× farther than the previous seven. The `0002-wayland-cursor-wheel-trace` patch applies the `×10` multiplier only when the frame carries a full classic click (>= 5 units per step) and scales high-resolution sub-steps uniformly by `×1.2` ([gio-patches.md](gio-patches.md)).

## 23. Disabled controls vanishing from the semantic tree
Under `gtx.Disabled()` pointer handlers register for zero gestures, and upstream Gio drops semantic ops on such areas. The fork keeps them (see #8), so a disabled control keeps its id and `semantic.EnabledOp(false)` on its own area.

## 24. Synthesizing text input in tests
In Gio's `app.Window`, inserting text into an editor sends a `key.EditEvent` followed immediately by a `key.SelectionEvent` moving the caret to the end of the inserted text. Sending `EditEvent` alone leaves the caret ahead of the inserted text when typing mid-field.

## 25. Draining chained `app.Post` callbacks in tests
A live window draws a frame for every queued `app.Post` callback. `kittest`'s `Settle` and `RunTasks` keep drawing frames while `app.Queued() > 0` so callbacks posted from inside another callback finish before strict mode checks the settled frame.
