# Gio patches (third_party/gioui.org, base v0.10.3)

Wired in through `replace gioui.org => ./third_party/gioui.org`. **Replace
directives are not inherited**, so every app using giokit must add its own
replace pointing here. Grep for `giokit patch` to find the changes.

## Cursor theme reload on scale change (`app/os_wayland.go`)

- **Upstream bug** (still present on main, Nov 2026): the cursor theme is loaded
  once, at the window's creation scale. `setCursor` divides hotspots by the
  *current* scale. On a scale-2 output the images stay 1× while the hotspots are
  halved, so the visible tip drifts from the real pointer. The hand cursor drifts
  the most.
- **Patch:**
  - Adds `cursor.name`, recorded in `SetCursor`.
  - Adds `reloadCursors()`, called from `updateOutputs` when the scale changes. It
    reloads the theme at `XCURSOR_SIZE*scale`, sets the cursor surface's buffer
    scale, restores the active cursor, and clears `system`.
- **Verify:** manually, on a scaled Wayland output. The hand cursor's tip must not
  jump when entering a button.
- **When bumping Gio:** re-apply the patch, or drop it if upstream fixed this.
  Consider sending it upstream.

## Even wheel scaling for high-resolution wheels (`app/os_wayland.go`)

- **Upstream bug:** `flushScroll` multiplies a frame's axis distance by 10 only
  when that frame carried `axis_discrete`. High-res wheels send an axis event
  every 1/8 click and the discrete step only with the last, so every 8th event
  jumps ~10× (a ~17px skip per click). See pitfalls #22.
- **Patch:** `app/giokit_scroll.go` (`wheelScale`, tested). A frame with a
  discrete step is scaled ×10 only if it carries a whole click (≥ 5 units per
  step, classic wheels send 10–15). High-res wheel sub-steps are all scaled
  ×1.2 by default, including the flagged 8th, so they move evenly. Set
  `GIOKIT_WHEEL_MINI_STEP_SCALE=1.5` before launch to change that multiplier,
  or call `app.SetGiokitWheelMiniStepScale(1.5)` at runtime. Values must be
  finite and positive; an invalid environment value falls back to 1.2.
  Wayland's axis source distinguishes wheel input from touchpads, whose
  motion is unchanged. Classic wheel clicks retain their ×10 scaling.
  `app.GiokitUpstreamWheel` restores upstream behavior for A/B testing.
- **Verify:** manually: `GIOKIT_TRACE=1`, slow wheel scroll; every
  `scroll_flush dy` within a click is equal (2.25 for a raw 1.875 high-res
  wheel sub-step).
- **When bumping Gio:** re-apply, or drop if upstream fixed it.

## Labels clip to their text (`widget/label.go`)

- **Upstream bug:** `Label.LayoutDetailed` clips the glyphs to the max
  constraints. The clip is the label's semantic area, so every label reports
  bounds as large as its container (`(304,-121)-(1099,999722)` was typical):
  wrong for screen readers, and kittest couldn't find a label's pixels.
- **Patch:** clip to the laid-out text size plus Gio's glyph padding.
- **Verify:** all five gallery pages render pixel-identical with and
  without it; `h.Contrasts()` bounds hug the text.
- **When bumping Gio:** re-apply. Worth upstreaming.

## Area bounds and focus for overlays (`io/input/giokit_bounds.go`)

- **Gap:** layout never learns absolute positions (transforms apply when ops
  are processed), so a popup can't tell whether it fits below its anchor.
- **Patch:** `Source.Bounds(tag)` returns the window-space bounds of the area
  where `tag`'s `event.Op` sat in the last frame (the router keeps them for
  hit testing). `Source.FocusedTag()` returns the focused tag (kittest's
  `FocusedID`). New file only; no upstream lines changed.
- **When bumping Gio:** re-apply; check `handler.pointer.areaPlusOne` and
  `pointer.queue.areas` still exist.

## Semantic image role (`io/semantic/semantic.go`)

- **Gap:** Gio's semantic classes had controls and editors but no image role.
  A labeled album cover looked like text to kittest's pixel contrast audit.
- **Patch:** append `semantic.Image` and its string name. `kit.Artwork` uses it;
  the audit treats its label as alternative text rather than painted glyphs.
- **When bumping Gio:** re-apply if upstream still lacks an image class; keep
  the enum value stable when possible.

## Native editing keys (`widget/editor.go`, `widget/text.go`)

Three commits, sent upstream from `editor-native-keys` on
https://git.sr.ht/~hhhapz/gio: Ctrl+Backspace/Delete word deletion outside
Apple platforms, Left/Right collapsing a selection, Up/Down to start/end in
single-line editors. Each adds a test to `widget/editor_test.go`.
**When bumping Gio:** drop what upstream merged; re-apply the rest from that
branch.

## Debug trace hooks (`app/giokit_trace.go`)

- `app.GiokitTrace`, called from `gtrace(...)` in `os_wayland.go` and
  `window.go` (axis, frame callbacks, draw, GPU/present timing). `kit.App`
  installs it under `GIOKIT_TRACE`. Re-apply when bumping; harmless when nil.

## Vulkan is opt-in on desktop Linux/FreeBSD

- **Upstream behavior:** Vulkan is on unless you pass `-tags novulkan`. `internal/vk`
  `#include`s `<vulkan/vulkan.h>` at compile time (the loader itself is dlopen'd),
  so `go run` fails with `vulkan/vulkan.h: No such file` on any machine without the
  `vulkan-headers` package, even when `libvulkan` is installed. On X11 Gio already
  skips Vulkan at runtime (`vulkanBuggy`), so only Wayland gained anything.
- **Patch:** `app/vulkan.go`, `app/vulkan_{x11,wayland}.go`,
  `gpu/internal/vulkan/vulkan.go`, `gpu/headless/headless_vulkan.go` require the
  `vulkan` tag on desktop. Android keeps upstream behavior (`!novulkan`); the NDK
  ships the headers. Without the tag Gio uses OpenGL ES via EGL, the same path
  kittest renders through.
- **Verify:** `go run ./examples/gallery` builds with no tags and no Vulkan headers.
- **When bumping Gio:** re-apply. Alternative if we ever want Vulkan by default:
  vendor `vulkan_core.h` into `internal/vk` (Apache-2.0/MIT).
