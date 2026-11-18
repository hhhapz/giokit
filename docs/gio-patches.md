# Gio patches (third_party/gioui.org, base v0.10.3)

Wired in through `replace gioui.org => ./third_party/gioui.org` for this
repository's own builds. **Replace directives are not inherited**, and since
2026-11-17 apps do not need one: `kit`, `kit/ui`, `kit/x` and `kittest` build
and pass on stock Gio (`scripts/stock-gio.sh`, a CI job). The fork is an
optional upgrade for its Wayland fixes, and required by `kit/x/video`
(`paint.Stream`, DMA-BUF import) and `kit/forktrace` (window event trace).
kit no longer uses `Source.Bounds`/`FocusedTag`, the label clip or
`semantic.Image`; those patches (0003–0005) stay for programs that use the
fork directly, and dropping one is a breaking change for them. Grep for
`giokit patch` to find the changes.

## The patch series (`third_party/patches`)

The fork is upstream `gioui.org@v0.10.3` plus seven patches, one concern
each, generated and applied by `scripts/gio-fork.sh`:

| Patch | Concern |
|---|---|
| `0001-vulkan-opt-in` | Vulkan behind `-tags vulkan`, so plain builds need no Vulkan headers |
| `0002-wayland-cursor-wheel-trace` | Cursor reload on scale change, even wheel scaling, trace hooks |
| `0003-label-clips-to-text` | Label areas hug their text |
| `0004-source-bounds-focus` | `Source.Bounds`, `Source.FocusedTag` |
| `0005-semantic-image` | `semantic.Image` class |
| `0006-editor-keys` | Single-line Up/Down, collapse selection, word delete |
| `0007-paint-stream-dmabuf` | `paint.Stream` and DMA-BUF import (video). One patch on purpose: both share `streamTex`, `internal/stream`, `op/paint/stream.go` and the GL texture struct. Upstreaming: propose `paint.Stream` (upload path) first; DMA-BUF import as a follow-up. |

- **After editing the fork:** `scripts/gio-fork.sh diff` regenerates the
  patches; `scripts/gio-fork.sh check` proves they rebuild the fork exactly.
  `diff` writes the new series beside the old one and replaces it only when
  every patch was generated, so a failed run leaves the patches intact. A
  new fork file belongs in its concern's entry in the script's `series`
  (directories are expanded). CI runs `check` and the tests of the patched
  packages (`gio-fork` job). The fork includes upstream's
  `gpu/internal/rendertest/refs/*.png`; `third_party/.gitignore` keeps the
  root `*.png` rule from dropping them, which would break `check`.
- **Bumping Gio:** `scripts/gio-fork.sh apply vX.Y.Z` rebuilds the fork from
  the new upstream plus the patches; fix any patch that no longer applies,
  then `diff` again. Patches whose concern upstream fixed are deleted from
  the series.

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
  Both settings are process-wide (every window) and atomic: safe to change
  from any goroutine, read at each scroll flush. The environment variable is
  read once, at package initialization.
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
- **Verify:** the gallery pages render pixel-identical with and without
  it; `h.Contrasts()` bounds hug the text.
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
- **Patch:** append `semantic.Image` and its string name. kit no longer uses
  it: `kit.TagImage` records `kit.RoleImage` instead, which works on stock
  Gio, and the audit treats that label as alternative text rather than
  painted glyphs.
- **When bumping Gio:** re-apply if upstream still lacks an image class; keep
  the enum value stable when possible.

## Native editing keys (`widget/editor.go`, `widget/text.go`)

Three commits, sent upstream from `editor-native-keys` on
https://git.sr.ht/~hhhapz/gio: Ctrl+Backspace/Delete word deletion outside
Apple platforms, Left/Right collapsing a selection, Up/Down to start/end in
single-line editors. Each adds a test to `widget/editor_test.go`.
**When bumping Gio:** drop what upstream merged; re-apply the rest from that
branch.

**Editors behave differently on stock Gio.** kit builds on both, and
`widget.Editor` (so `ui.TextField`, `ui.TextArea` and every field built on
an editor) follows whichever Gio the app links:

| Key | Fork | Stock Gio v0.10.3 |
|---|---|---|
| Ctrl+Backspace / Ctrl+Delete (Linux, Windows) | deletes the previous / next word | nothing: Ctrl is the shortcut modifier there, and the key is swallowed by the shortcut branch (Alt+Backspace on macOS works on both) |
| Left / Right with a selection | caret to the selection's left / right edge | selection cleared, then the caret moves one character from where it was |
| Up / Down in a single-line editor | caret to the start / end of the text | `MoveLines` on the only line: the caret does not reach the start / end |

Components that handle Up/Down themselves (NumberField steps, Combobox and
the command palette navigate options) behave the same on both. kittest's
stock-Gio run (`scripts/stock-gio.sh`) passes because no kit test depends
on these keys; a test that does must be fork-only. The fork's own
`widget/editor_test.go` cases cover the patched behaviour (`gio-fork` CI
job).

## Debug trace hooks (`app/giokit_trace.go`)

- `app.GiokitTrace`, called from `gtrace(...)` in `os_wayland.go` and
  `window.go` (axis, frame callbacks, draw, GPU/present timing).
  `kit/forktrace` installs it from its `init` under `GIOKIT_TRACE` (import
  it for its side effect); kit itself never touches it, so it builds on
  stock Gio. It is a plain variable read without synchronization on window
  goroutines: set it once before the first window, never later.
  Re-apply when bumping; harmless when nil.

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

## Streaming images (`op/paint/stream.go`, `internal/stream`, `gpu/gpu.go`)

- **Gap:** an `ImageOp` is immutable. Showing video meant `paint.NewImageOp`
  per frame: a new cache key each time, so a new texture, a full upload and a
  mipmap rebuild every frame, with the old texture evicted a frame later.
- **Patch:** `paint.Stream` (`NewStream`, `Submit(StreamFrame)`, `Close`,
  `Size`, `Add`, `ImportState`, and the `Filter` and `OnFrame` fields). A producer on any goroutine submits RGBA frames
  (latest wins; the replaced frame's `Release` runs). `Add` encodes an ordinary
  image op whose handle is the stream's `*internal/stream.Stream` and whose
  `src` is a size-only placeholder (the renderer reads `src` only for its
  size). `renderer.texHandle` recognises that handle and calls `streamTex`:
  one texture per stream, linear or nearest filter, **no mipmaps**, re-created
  on size change, uploaded only when the stream's sequence number moved. The
  upload holds the stream's lock, so `Submit` cannot release a frame mid-copy.
  If the stream is not drawn for a frame the cache frees its texture; the next
  draw re-uploads the retained current frame.
- **Frame ownership:** the stream owns a frame from `Submit` until its
  `Release`, which runs exactly once, never under the stream's lock, on the
  goroutine that ends the need for it: the producer's (`Submit`, `Close`) or
  a window's render goroutine (a newer frame bound, or the texture dropped).
  Holds on DMA-BUF frames are counted, so several windows can draw one
  stream; the frame is released when the last one lets go.
  `Submit(StreamFrame{})` releases the current frame and keeps the stream
  open; `Close` releases it and refuses (releases at once) later frames.
  Tested without a GPU: `internal/stream/stream_test.go` (replace, hold,
  two holders, close) and `gpu/stream_test.go` (two renderers, import cache
  beyond its cap, with a fake importer).
- **Verify:** `go test ./kit/x/video` (newest frame reaches the screen, hidden
  and shown again, frame mapping), `go test ./examples/gallery -run VideoDemo`,
  `go test -run x -bench Video ./internal/bench` (Nov 2026, RX 6700 XT:
  0.84 ms/frame at 1080p, 2.2 ms at 2960×1848, 0.12–0.16 ms without a new
  frame; CPU time is on par with ImageOp-per-frame, the saving is GPU mipmaps
  and texture churn, which this headless bench does not time).
- **DMA-BUF frames (zero copy, Linux OpenGL):** `StreamFrame.DMABuf` (`paint.DMABuf`:
  ID, fd, offset, pitch, DRM fourcc and modifier, size) and
  `Stream.ImportState()`. `driver.DMABufImporter` is implemented by the
  OpenGL backend in `gpu/internal/opengl/dmabuf_linux.go`: `eglCreateImageKHR`
  (`EGL_LINUX_DMA_BUF_EXT`, modifiers when given) on the current display,
  `glEGLImageTargetTexStorageEXT` for immutable storage, then a
  `glTextureViewOES` with `SRGB8_ALPHA8` so it samples as sRGB like every Gio
  texture (Mesa has no `EGL_EXT_image_gl_colorspace`). `streamTex` imports
  each buffer ID once (≤ 12 kept, LRU, never the bound one nor the one
  being bound), **holds** the
  bound frame (`stream.Use.Hold`/`Unhold`) because it is sampled in place,
  and releases it only after binding a newer one or when the cache drops the
  texture. A failed import sets `ImportUnsupported` and draws the last good
  texture or a 1×1 transparent one; producers then send CPU frames.
  Other backends (Vulkan, Metal, D3D11) report unsupported.
- **Verify (zero copy):** `cd media && go test .` (`TestZeroCopyRender` compares the
  imported frames with the software decode, `TestZeroCopyColour`,
  `TestImportFailureDegrades`); `go test -run x -bench Render ./media`
  (Nov 2026: 3.2 ms/frame decode+render at 2960×1848 zero copy vs 8.0 ms
  software decode + upload).
- **When bumping Gio:** re-apply. Written to be proposed upstream.
