# Gio Fork (`gio/`)

`giokit` carries a hard fork of [Gio](https://gioui.org) in `gio/`, imported as `github.com/hhhapz/giokit/gio/...`: `go get github.com/hhhapz/giokit` brings it along, with no `replace` directive. Apps import `github.com/hhhapz/giokit/gio/app`, `.../gio/layout` and so on instead of `gioui.org/...`. It holds upstream `gioui.org`, the `gioui.org/shader` packages Gio uses (`shader`, `shader/gio`, `cmd/convertshaders`), `gioui.org/x/styledtext`, and `gogio` from `gioui.org/cmd` (without its end-to-end tests); `gio/UPSTREAM` records the upstream versions. Gio's license (Unlicense OR MIT) stays in `gio/LICENSE`.

Nothing in an app's dependency graph may import upstream `gioui.org`: two Gios cannot link into one binary (their cgo exports collide), and their types never mix. Bring a Gio library into `gio/` (add it to `modules` in `scripts/gio-sync.sh`) rather than depending on it.

Linker flags name the fork's path: `-ldflags="-X 'github.com/hhhapz/giokit/gio/app.ID=com.example.App'"`.

Package apps for Android, iOS and the other targets with the fork's `gogio` (`gio/cmd/gogio`): stock `gogio` looks for `gioui.org/app` to find Gio's Java and Objective-C glue, which the fork moves. Pin it as a tool in the app's module and run it from there:

```sh
go get -tool github.com/hhhapz/giokit/gio/cmd/gogio
go tool gogio -target android -appid com.example.app -icon icon.png -o app.apk ./cmd/app
```

Android builds need `ANDROID_HOME` (the SDK, with a platform and build-tools) and `ANDROID_NDK_ROOT`. A `~/.android/debug.keystore` (password `android`) signs every debug build with the same key; without one, each build gets a new key and `adb install -r` refuses to replace the previous build.

## Working With Upstream (`scripts/gio-sync.sh`)

The rewrite is one text substitution on Go files, `"gioui.org/` to `"github.com/hhhapz/giokit/gio/`; doc comments keep upstream names. Because it is a pure substitution, whole trees and single patches rewrite identically:

- **Snapshot commits** (`gio: upstream gioui.org@… gioui.org/shader@… gioui.org/x@…`) hold pristine rewritten upstream and change nothing but `gio/`. Each new one is parented on the previous one. The newest reachable from `HEAD` is the **base**: our changes are what came after it. No branch holds upstream. Squash anything else freely, but never squash a snapshot commit into another commit, or the base is lost.
- **Bump Gio:** `scripts/gio-sync.sh update v0.11.0 [shader] [x]` commits a snapshot of those versions on top of the base and merges it into `HEAD`. Conflicts appear only where we changed Gio. Drop our changes that upstream merged, and remove skips or workarounds they made unnecessary. Versions are anything `go mod download` takes, so a commit hash tracks upstream `main`.
- **Take one upstream fix early:** `scripts/gio-sync.sh pick <sha>` applies that upstream commit to master (`git am`, rewritten).
- **Send a change upstream:** `scripts/gio-sync.sh export [rev-range]` writes our `gio/` commits (by default everything since the base) as patches against upstream paths, for `git send-email` to `~eliasnaur/gio-patches@lists.sr.ht`.
- **Check:** `scripts/gio-sync.sh check` (in CI) rebuilds the snapshot of the base's versions and compares it with the base, and fails if a module requires upstream `gioui.org`.
- **Our differences:** `git diff <base> -- gio/`, where the base is `git log -1 --format=%h --grep='^gio: upstream '`.

Rules for changes in `gio/`:
1. Commits may change `gio/` together with the rest of the repository; only snapshot commits must hold nothing but pristine upstream.
2. Prefer new files (`giokit_*.go`) over edits to upstream files; keep edits to upstream files small. Merge conflicts grow with the lines changed in upstream's files, not with what we add.
3. Mark edits inside upstream files with a `giokit:` comment saying why.

`go test ./...` runs Gio's tests too.

## Changes

Our changes to upstream (the versions in `gio/UPSTREAM`), by area:

| Area | Change |
|---|---|
| Vulkan | Asks for Vulkan 1.1 and adds `GIOKIT_GPU` backend selection to headless rendering. |
| Wayland | Wayland cursor reload on scale change, even high-resolution wheel scaling, Vulkan before EGL, and `app.GiokitTrace` hooks. |
| Labels | Clips `widget.Label` semantic bounds to laid-out text size. |
| Input | Adds `Source.Bounds` and `Source.FocusedTag` on `input.Source`; keeps semantics on areas whose handler asks for no gestures; adds `key.MoveFocusCmd` so non-keyboard input (gamepads) can move focus. |
| Semantics | Adds `semantic.Image` and WAI-ARIA role classes (`Link`, `Slider`, `Tab`, `MenuItem`, ...) to `io/semantic`. |
| Editor | Adds `Ctrl+Backspace`/`Delete` word delete (Windows, Linux), selection collapse on `Left`/`Right`, and single-line `Up`/`Down`. |
| Keys | Adds physical keys: `key.Code` (named like the web's `KeyboardEvent.code`) on `key.Event` and `key.Filter`, reported on the web, X11, Wayland, Windows and macOS, with the US place standing in elsewhere; `Code.Name` gives what the current layout types there, from the platform (web where supported, xkb, Windows) or learned from presses, where one press that reveals a common layout (QWERTZ, AZERTY, Dvorak, Colemak, Workman) relabels every key. |
| Streams | Adds cross-platform `paint.Stream` (Metal, Direct3D 11, OpenGL ES, Vulkan, WebGL) plus zero-copy Linux DMA-BUF texture import (OpenGL ES and Vulkan). |
| GPU stats | Adds `gpu.ReadStats` and `headless.Window.Stats`: shapes flattened and uploaded per GPU, for `kittest`'s path-build checks (pitfall 26). |
| Scaled images | An image painted under an axis-aligned scale (no shear, rotation or mirroring) gets no clip path; upstream built one for any non-identity transform, so every scaled image drawn into a frame was a path upload per frame. Colour fills keep the path, which antialiases fractional edges. |
| Headless | Falls back to WARP on Direct3D 11 without a GPU; `GIOKIT_GPU=warp` forces it. |
| Web | Delivers key events from the window when no text input has focus, so shortcuts and key handlers work in the browser (upstream only listens on the hidden textarea, which is focused only for text input). |

## Patch Details

### 1. Vulkan 1.1 and Backend Selection 

Like upstream, giokit builds Vulkan on Linux/FreeBSD, so building needs the system `vulkan-headers` package (`internal/vk` includes `<vulkan/vulkan.h>`); `-tags novulkan` builds OpenGL only. Gio loads `libvulkan` at runtime, so a binary built with Vulkan still starts where Vulkan is missing and uses EGL.

`vk.CreateInstance` asks for Vulkan 1.1 where the loader has it (DMA-BUF import needs it), else 1.0.

`GIOKIT_GPU` picks the backend: `vulkan` uses only Vulkan, `opengl` never uses it. Headless rendering (`gpu/headless`, which kittest uses) keeps EGL first and Vulkan as the fallback, so `GIOKIT_GPU=vulkan go test ./...` is how to cover the Vulkan renderer. Windows on Wayland honour it too (the Wayland change).

### 2. Wayland Cursor Reload, High-Res Wheel Scaling, Backend Order, and Trace Hooks 

#### Vulkan Before EGL (`app/os_wayland.go`)
Upstream `NewContext` on Wayland tries EGL first and Vulkan only if EGL fails. The patch tries Vulkan first: on Mesa, the GL driver (`libgallium`) costs about 30 MB more RSS per process than RADV (the gallery idles at about 85 MB on Vulkan and 120 MB on GL). EGL remains the fallback when there is no Vulkan driver or device creation fails; `GIOKIT_GPU=opengl` or `GIOKIT_GPU=vulkan` forces one. X11 keeps upstream's order (Vulkan is disabled there: `vulkanBuggy`, gio#347).

#### Cursor Theme Reload on Scale Change (`app/os_wayland.go`)
Upstream Gio loads the cursor theme once at the window's creation scale, while `setCursor` divides cursor hotspots by the current output scale. Moving onto a scale-2 monitor leaves cursor images at 1× while halving their hotspots, so the visible tip drifts ~9px from the pointer.

The patch records `cursor.name` in `SetCursor` and adds `reloadCursors()`, called from `updateOutputs` when the scale changes. It reloads the cursor theme at `XCURSOR_SIZE * scale`, sets the cursor surface buffer scale, and restores the active cursor.

#### Even Wheel Scaling for High-Resolution Wheels (`app/os_wayland.go`, `app/giokit_scroll.go`)
Upstream `flushScroll` multiplies a pointer frame's scroll distance by `10` only when that frame carried `axis_discrete`. High-resolution wheels send an axis event of `1.875` every 1/8th click and attach `axis_discrete = 1` only on the 8th event, causing every 8th event to jump 10× ([pitfalls.md #22](pitfalls.md#22-high-resolution-wheel-skipped-17px-once-per-click-wayland)).

In `app/giokit_scroll.go` (`wheelScale`), a frame with a discrete step is scaled by `10` only if it carries a whole click (>= 5 units per step; classic wheels send 10 to 15). High-resolution wheel sub-steps are all scaled by `1.2` by default (`GIOKIT_WHEEL_MINI_STEP_SCALE` overrides it at startup). Touchpad scrolling is unchanged.

#### Debug Trace Hooks (`app/giokit_trace.go`)
Adds `app.GiokitTrace`, called from `os_wayland.go` and `window.go` on axis events, frame callbacks, draw passes, and GPU presentation. Importing `_ "github.com/hhhapz/giokit/kit/forktrace"` installs the hook when `GIOKIT_TRACE` is set.

### 3. Labels Clip to Their Text 

Upstream `widget.Label.LayoutDetailed` clips glyphs to `gtx.Constraints.Max`, causing semantic bounds for labels to span the full container height. The patch clips to the laid-out text size plus glyph padding.

### 4. Area Bounds and Focus Queries 

Adds `Source.Bounds(tag)` and `Source.FocusedTag()` in `io/input/giokit_bounds.go`.

### 4b. Semantics on Gesture-less Handlers

Upstream `pointerQueue.Frame` set an area's semantic validity from its handler's gestures alone, so an area whose handler asked for no clicks or scrolls (hover, keys only, or disabled) lost the description, label and class its ops gave it. The patch only ever turns validity on (`io/input/giokit_semantic_test.go`).

### 5. Semantic Image Role 

Appends `semantic.Image` and the WAI-ARIA role classes kit's components need (`Link`, `Slider`, `ComboBox`, `ListBox`, `Option`, `List`, `ListItem`, `Tree`, `TreeItem`, `Grid`, `Row`, `GridCell`, `TabList`, `Tab`, `Menu`, `MenuItem`, `Dialog`, `Toolbar`, `Status`, `Tooltip`, `Group`, `Separator`) to `semantic.ClassOp` in `io/semantic/semantic.go`; `kit.Role` is that type. Android reports the button-like ones as `android.widget.Button`.

### 6. Native Editing Keys 

Adds three desktop editing behaviors to `widget/editor.go` and `widget/text.go`:

| Key | Fork | Stock Gio v0.10.3 |
|---|---|---|
| `Ctrl+Backspace` / `Ctrl+Delete` (Linux, Windows) | Deletes the previous / next word | Ignored (`Ctrl` is swallowed by the shortcut branch) |
| `Left` / `Right` with a selection | Collapses caret to the left / right edge of the selection | Clears selection and moves caret one character from its old end |
| `Up` / `Down` in a single-line editor | Moves caret to the start / end of the line | Calls `MoveLines` on the single line, leaving the caret in place |

### 7. Streaming Images and DMA-BUF Import 

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
