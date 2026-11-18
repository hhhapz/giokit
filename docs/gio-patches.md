# Gio Patches (`third_party/gioui.org`)

`giokit` works with stock upstream Gio (`gioui.org@v0.10.3`) and also maintains an optional patch series in `third_party/gioui.org`.

To use the patched fork in an application, add a `replace` directive to your `go.mod`:

```go
replace gioui.org => /path/to/giokit/third_party/gioui.org
```

The fork is required by `kit/x/video` and `media` (which use `paint.Stream`) and `kit/forktrace` (which uses window trace hooks), and is optional for other packages.

## The Patch Series (`third_party/patches`)

The fork is upstream `gioui.org@v0.10.3` plus seven patches managed by `./scripts/gio-fork.sh`:

| Patch | Concern |
|---|---|
| `0001-vulkan-opt-in` | Gates desktop Linux/FreeBSD Vulkan behind `-tags vulkan` so default builds need no Vulkan headers. |
| `0002-wayland-cursor-wheel-trace` | Wayland cursor reload on scale change, even high-resolution wheel scaling, and `app.GiokitTrace` hooks. |
| `0003-label-clips-to-text` | Clips `widget.Label` semantic bounds to laid-out text size. |
| `0004-source-bounds-focus` | Adds `Source.Bounds` and `Source.FocusedTag` on `input.Source`. |
| `0005-semantic-image` | Adds `semantic.Image` to `io/semantic`. |
| `0006-editor-keys` | Adds `Ctrl+Backspace`/`Delete` word delete (Windows, Linux), selection collapse on `Left`/`Right`, and single-line `Up`/`Down`. |
| `0007-paint-stream-dmabuf` | Adds cross-platform `paint.Stream` (Metal, Direct3D 11, OpenGL ES, Vulkan, WebGL) plus zero-copy Linux DMA-BUF texture import. |

### Maintaining the Patches

- After editing `third_party/gioui.org`, run `./scripts/gio-fork.sh diff` to regenerate the patch files and `./scripts/gio-fork.sh check` to verify that applying the patches to a clean v0.10.3 tree reproduces `third_party/gioui.org`.
- To upgrade Gio to a new tag, run `./scripts/gio-fork.sh apply vX.Y.Z`, resolve any conflicts, drop any patches merged upstream, and run `diff` and `check`.

## Patch Details

### 1. Opt-In Vulkan on Desktop Linux and FreeBSD (`0001-vulkan-opt-in`)

Upstream Gio enables Vulkan on Linux/FreeBSD unless `-tags novulkan` is passed. Because `internal/vk` includes `<vulkan/vulkan.h>` at compile time, builds fail on Linux machines without the `vulkan-headers` package installed.

This patch requires `-tags vulkan` on desktop Linux/FreeBSD (`app/vulkan*.go`, `gpu/internal/vulkan/vulkan.go`, `gpu/headless/headless_vulkan.go`). Without the tag, Gio uses OpenGL ES via EGL.

### 2. Wayland Cursor Reload, High-Res Wheel Scaling, and Trace Hooks (`0002-wayland-cursor-wheel-trace`)

#### Cursor Theme Reload on Scale Change (`app/os_wayland.go`)
Upstream Gio loads the cursor theme once at the window's creation scale, while `setCursor` divides cursor hotspots by the current output scale. Moving onto a scale-2 monitor leaves cursor images at 1× while halving their hotspots, so the visible tip drifts ~9px from the pointer.

The patch records `cursor.name` in `SetCursor` and adds `reloadCursors()`, called from `updateOutputs` when the scale changes. It reloads the cursor theme at `XCURSOR_SIZE * scale`, sets the cursor surface buffer scale, and restores the active cursor.

#### Even Wheel Scaling for High-Resolution Wheels (`app/os_wayland.go`, `app/giokit_scroll.go`)
Upstream `flushScroll` multiplies a pointer frame's scroll distance by `10` only when that frame carried `axis_discrete`. High-resolution wheels send an axis event of `1.875` every 1/8th click and attach `axis_discrete = 1` only on the 8th event, causing every 8th event to jump 10× ([pitfalls.md #22](pitfalls.md#22-high-resolution-wheel-skipped-17px-once-per-click-wayland)).

In `app/giokit_scroll.go` (`wheelScale`), a frame with a discrete step is scaled by `10` only if it carries a whole click (>= 5 units per step; classic wheels send 10 to 15). High-resolution wheel sub-steps are all scaled by `1.2` by default (configurable via `GIOKIT_WHEEL_MINI_STEP_SCALE` or `app.SetGiokitWheelMiniStepScale`). Touchpad scrolling is unchanged.

#### Debug Trace Hooks (`app/giokit_trace.go`)
Adds `app.GiokitTrace`, called from `os_wayland.go` and `window.go` on axis events, frame callbacks, draw passes, and GPU presentation. Importing `_ "github.com/hhhapz/giokit/kit/forktrace"` installs the hook when `GIOKIT_TRACE` is set.

### 3. Labels Clip to Their Text (`0003-label-clips-to-text`)

Upstream `widget.Label.LayoutDetailed` clips glyphs to `gtx.Constraints.Max`, causing semantic bounds for labels to span the full container height. The patch clips to the laid-out text size plus glyph padding.

### 4. Area Bounds and Focus Queries (`0004-source-bounds-focus`)

Adds `Source.Bounds(tag)` and `Source.FocusedTag()` in `io/input/giokit_bounds.go`.

### 5. Semantic Image Role (`0005-semantic-image`)

Appends `semantic.Image` in `io/semantic/semantic.go`.

### 6. Native Editing Keys (`0006-editor-keys`)

Adds three desktop editing behaviors to `widget/editor.go` and `widget/text.go`:

| Key | Fork | Stock Gio v0.10.3 |
|---|---|---|
| `Ctrl+Backspace` / `Ctrl+Delete` (Linux, Windows) | Deletes the previous / next word | Ignored (`Ctrl` is swallowed by the shortcut branch) |
| `Left` / `Right` with a selection | Collapses caret to the left / right edge of the selection | Clears selection and moves caret one character from its old end |
| `Up` / `Down` in a single-line editor | Moves caret to the start / end of the line | Calls `MoveLines` on the single line, leaving the caret in place |

### 7. Streaming Images and DMA-BUF Import (`0007-paint-stream-dmabuf`)

#### `paint.Stream` (`op/paint/stream.go`, `internal/stream`, `gpu/gpu.go`)
An `ImageOp` in Gio is immutable: displaying video with `paint.NewImageOp` per frame creates a new cache key, allocates a new texture, uploads CPU pixels, and builds mipmaps every frame.

`paint.Stream` (`NewStream`, `Submit(StreamFrame)`, `Close`, `Size`, `Add`, `ImportState`, `Filter`, `OnFrame`) lets a producer on any goroutine submit frames where the latest frame wins and any replaced frame has its `Release` callback called immediately. Across all five Gio GPU backends (Metal on macOS/iOS, Direct3D 11 on Windows, OpenGL ES and Vulkan on Linux/Android, and WebGL on WebAssembly), the renderer keeps one persistent texture per stream (linear or nearest filter, no mipmaps), recreates it only when dimensions change, and uploads only when the stream's sequence number advances.

The stream owns a submitted frame until its `Release` callback runs. Holds on DMA-BUF frames are reference-counted (`Hold`/`Unhold`) so multiple windows or renderers can sample the same stream safely. `Close` releases the current frame and rejects later submissions.

#### Zero-Copy Linux DMA-BUF Import (`gpu/internal/opengl/dmabuf_linux.go`)
When `StreamFrame.DMABuf` (`paint.DMABuf`: ID, fd, offset, pitch, DRM fourcc and modifier, size) is set, the OpenGL ES backend imports the buffer via `eglCreateImageKHR` (`EGL_LINUX_DMA_BUF_EXT`), allocates immutable storage with `glEGLImageTargetTexStorageEXT`, and creates a `glTextureViewOES` with `SRGB8_ALPHA8` so it samples as sRGB like every other Gio texture.

`streamTex` caches up to 12 imported buffer IDs per stream and holds the bound frame until a newer frame is bound or the texture is dropped. If an import fails, the stream sets `ImportUnsupported` and producers fall back to CPU pixel uploads.
