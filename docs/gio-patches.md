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
| `0001-vulkan` | Asks for Vulkan 1.1 and adds `GIOKIT_GPU` backend selection to headless rendering. |
| `0002-wayland-cursor-wheel-trace` | Wayland cursor reload on scale change, even high-resolution wheel scaling, Vulkan before EGL, and `app.GiokitTrace` hooks. |
| `0003-label-clips-to-text` | Clips `widget.Label` semantic bounds to laid-out text size. |
| `0004-source-bounds-focus` | Adds `Source.Bounds` and `Source.FocusedTag` on `input.Source`. |
| `0005-semantic-image` | Adds `semantic.Image` to `io/semantic`. |
| `0006-editor-keys` | Adds `Ctrl+Backspace`/`Delete` word delete (Windows, Linux), selection collapse on `Left`/`Right`, and single-line `Up`/`Down`. |
| `0007-paint-stream-dmabuf` | Adds cross-platform `paint.Stream` (Metal, Direct3D 11, OpenGL ES, Vulkan, WebGL) plus zero-copy Linux DMA-BUF texture import (OpenGL ES and Vulkan). |

### Maintaining the Patches

- After editing `third_party/gioui.org`, run `./scripts/gio-fork.sh diff` to regenerate the patch files and `./scripts/gio-fork.sh check` to verify that applying the patches to a clean v0.10.3 tree reproduces `third_party/gioui.org`.
- To upgrade Gio to a new tag, run `./scripts/gio-fork.sh apply vX.Y.Z`, resolve any conflicts, drop any patches merged upstream, and run `diff` and `check`.

## Patch Details

### 1. Vulkan 1.1 and Backend Selection (`0001-vulkan`)

Like upstream, the fork builds Vulkan on Linux/FreeBSD, so building needs the system `vulkan-headers` package (`internal/vk` includes `<vulkan/vulkan.h>`); `-tags novulkan` builds OpenGL only. Gio loads `libvulkan` at runtime, so a binary built with Vulkan still starts where Vulkan is missing and uses EGL.

`vk.CreateInstance` asks for Vulkan 1.1 where the loader has it (DMA-BUF import, patch 7, needs it), else 1.0.

`GIOKIT_GPU` picks the backend: `vulkan` uses only Vulkan, `opengl` never uses it. Headless rendering (`gpu/headless`, which kittest uses) keeps EGL first and Vulkan as the fallback, so `GIOKIT_GPU=vulkan go test ./...` is how to cover the Vulkan renderer. Windows on Wayland honour it too (patch 2).

### 2. Wayland Cursor Reload, High-Res Wheel Scaling, Backend Order, and Trace Hooks (`0002-wayland-cursor-wheel-trace`)

#### Vulkan Before EGL (`app/os_wayland.go`)
Upstream `NewContext` on Wayland tries EGL first and Vulkan only if EGL fails. The patch tries Vulkan first: on Mesa, the GL driver (`libgallium`) costs about 30 MB more RSS per process than RADV (the gallery idles at about 85 MB on Vulkan and 120 MB on GL). EGL remains the fallback when there is no Vulkan driver or device creation fails; `GIOKIT_GPU=opengl` or `GIOKIT_GPU=vulkan` forces one. X11 keeps upstream's order (Vulkan is disabled there: `vulkanBuggy`, gio#347).

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

#### Zero-Copy Linux DMA-BUF Import, OpenGL ES (`gpu/internal/opengl/dmabuf_linux.go`)
When `StreamFrame.DMABuf` (`paint.DMABuf`: ID, fd, offset, pitch, DRM fourcc and modifier, size) is set, the OpenGL ES backend imports the buffer via `eglCreateImageKHR` (`EGL_LINUX_DMA_BUF_EXT`), allocates immutable storage with `glEGLImageTargetTexStorageEXT`, and creates a `glTextureViewOES` with `SRGB8_ALPHA8` so it samples as sRGB like every other Gio texture.

#### Zero-Copy Linux DMA-BUF Import, Vulkan (`internal/vk/giokit_dmabuf.go`, `gpu/internal/vulkan/dmabuf.go`)
Device creation (`app/vulkan.go`, `gpu/headless/headless_vulkan.go`) enables `VK_KHR_external_memory_fd`, `VK_EXT_external_memory_dma_buf`, `VK_KHR_image_format_list`, `VK_EXT_image_drm_format_modifier` and `VK_EXT_queue_family_foreign` when the device has all of them on Vulkan 1.1, plus `VK_KHR_external_semaphore_fd` where present, and reports it in `gpu.Vulkan.DMABuf`. The backend creates a `VK_IMAGE_TILING_DRM_FORMAT_MODIFIER_EXT` image with the buffer's explicit modifier and plane layout, binds it to dedicated memory imported from a duplicate of the fd, and samples it through an sRGB view of its UNORM storage (`VK_IMAGE_CREATE_MUTABLE_FORMAT_BIT` with a format list). ABGR8888, XBGR8888, ARGB8888 and XRGB8888 are supported (X formats read alpha as 1). Modifiers with more than one memory plane (compression metadata) and `DRM_FORMAT_MOD_INVALID` are refused, which reports `ImportUnsupported`.

The image belongs to the foreign queue family between frames. `PrepareTexture` acquires it (`GENERAL` → `SHADER_READ_ONLY_OPTIMAL`, from `VK_QUEUE_FAMILY_FOREIGN_EXT`) and first exports the buffer's pending write fences as a sync file (`DMA_BUF_IOCTL_EXPORT_SYNC_FILE`, Linux 6.0), which the frame's submission waits on: RADV does not wait on a DMA-BUF's implicit fences the way GL drivers do. `EndFrame` releases every image acquired that frame back to the foreign queue. Without sync file export, the producer must have finished writing before `Submit` (the media decoder calls `vaSyncSurface`). Writes after reads need no fence: Gio waits for a frame's fence before building the next, and a buffer goes back to its producer only once a later frame bound a newer one.

`media`'s zero-copy tests (`TestZeroCopyRender`, `TestZeroCopyColour`, `TestImportFailureDegrades`) pass under `GIOKIT_GPU=vulkan`, and report nothing with `VK_INSTANCE_LAYERS=VK_LAYER_KHRONOS_validation VK_LAYER_VALIDATE_SYNC=1`.

`streamTex` caches up to 12 imported buffer IDs per stream and holds the bound frame until a newer frame is bound or the texture is dropped. If an import fails, the stream sets `ImportUnsupported` and producers fall back to CPU pixel uploads.
