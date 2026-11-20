# Architecture

`giokit` uses immediate-mode rendering with [GPUI](https://www.gpui.rs/)'s approach to state ownership, scoped identity, keyboard dispatch, and headless testing.

GPUI combines retained entities and observers on the main thread with lightweight elements rebuilt each frame. `giokit` uses only the per-frame half: application state lives in plain Go structs owned by the UI goroutine, and the window is laid out from that state on each frame.

| GPUI | giokit |
|---|---|
| Main thread owns entities; views render from state each frame | UI goroutine owns plain structs; `Frame` handles input first, then lays out the window |
| `cx.notify()` | `kit.Notify(gtx)` (called by `kit.Clickable` and component state transitions) |
| `cx.spawn` / background executor | `kit.Spawn(app, work, done)` returning a cancellable `*kit.Task`, plus `app.Post` |
| Fluent element builder (`div().flex().gap_2().p_4()`) | Plain Go structs (`kit.Flex{Axis, Gap, Pad, Bg, Border, Radius}`, `ui.Button{...}`) |
| Actions, keymaps, and focus handles | `kit.Actions` registry, `kit.Keymap` with JSON overrides, `kit.KeyContext`, `kit.OnAction`, `kit.Focus`, `kit.FocusGroup` |
| Theme as data | `Palette`, `TypeScale`, `Metrics`, `Strings`, OKLCH `Generate` with WCAG contrast fitting, DTCG JSON import/export |
| `TestAppContext` / `VisualTestContext` | `kittest` and `kittest/conform`: real `input.Router`, headless GPU rendering, semantic and role queries, strict stale-frame check, and audits |
| Font system | Bundled SIL OFL font pairs (`kit/fonts`: Go, Inter, Plex, Atkinson) with system font fallback disabled |

## Packages

- `kit` holds foundational primitives: theme tokens, OKLCH palette generation, flex and flow layout, surface drawing, vector icons, interaction state (`Clickable`, `HoverArea`, `Pool`), scoped semantic IDs and WAI-ARIA roles, focus management (`Focus`, `FocusGroup`, `FocusVisible`), keyboard dispatch (`Window`, `KeyContext`, `OnAction`, `Keymap`, `Actions`), scrolling (`List`, `AnchoredList`), overlay hosts (`Popup`, `Dialog`, `Layer`), and the application runner (`App`, `Spawn`, `Task`).
- `kit/ui` holds the standard UI components (buttons, toggles, fields, pickers, menus, lists, trees, tables, navigation, overlays, and split panes), built strictly from `kit`'s exported API.
- `kit/x` holds specialized widgets (`LogView` with `LogBuffer`, and `Lyrics`) built on `kit` and `kit/ui`.
- `kit/x/video` and `media/` handle streaming GPU video (`video.Video`) and FFmpeg video decoding. `media/` is a separate Go module (`github.com/hhhapz/giokit/media`) so applications that do not play video do not link FFmpeg. It links the system FFmpeg by default, or a minimal static one with `-tags ffmpeg_static` ([static-ffmpeg.md](static-ffmpeg.md)).
- `desktop` defines a cross-platform `Platform` interface for native file choosers (`OpenFile`, `SaveFile`) and `OpenURL`, with a `*desktop.Fake` for tests.
- `kittest` and `kittest/conform` provide the headless test harness and the control conformance suite.

## The Frame Lifecycle and `kit.Window`

Both `kit.App.Run` and `kittest.Harness` wrap every frame in a `kit.Window`:

```go
app.Drain()          // Run callbacks queued via app.Post and kit.Spawn
gtx = win.Begin(gtx) // Install per-frame state in gtx.Values
frame(gtx)           // Application Frame: handle input first, then lay out
win.End(gtx)         // Resolve unclaimed keyboard events through the dispatch tree
```

`Window.Begin(gtx)` installs a per-frame record in `gtx.Values["giokit.frame"]`. During layout:
- `kit.Scope`, `kit.Tag`, `role.Add(gtx.Ops)`, and `kit.FocusID` record each widget's scoped path, WAI-ARIA role, and focusable tag.
- `kit.KeyContext`, `kit.OnAction`, and `Keymap.Attach` build a tree of keyboard scopes matching the layout hierarchy.

After `frame(gtx)` returns, `Window.End(gtx)` pops the root area and runs the keyboard dispatcher:
1. Keys already consumed during layout by a focused widget (such as text entry in a `widget.Editor` or arrow keys on a `ui.Slider`) have already been removed from Gio's input queue.
2. For remaining key events, `End` finds which registered focus tag currently holds Gio focus (`gtx.Focused(tag)`), locates its `KeyContext` node in the frame tree, and walks outward from that node to the root.
3. The innermost matching `Keymap` binding or `kit.OnAction` handler runs and calls `kit.Notify(gtx)` so the resulting state change is drawn on the next frame.

## Component Contract: Props, State, and Events

Components in `kit/ui` and `kit/x` separate per-frame configuration (the props struct) from persistent interaction state (the state pointer):

```go
if id, ok := v.roleState.Update(gtx); ok {
	v.selectedRole = id
}

ui.Select{
	ID:       "role",
	Options:  roles,
	Selected: v.selectedRole,
}.Layout(gtx, th, &v.roleState)
```

### Controlled Props and State-Owned Values

By default, values are controlled: the caller passes `Selected`, `Checked`, `On`, `Value`, or `Open` in the props struct each frame, and updates its own state when `state.Update(gtx)` or `Clickable.Clicked(gtx)` reports an event.

A few values live in the state struct instead because they represent continuous or local interaction state:

| State Type | Retained Value | Reason |
|---|---|---|
| `ui.SplitState` | `Ratio float32` | Updates continuously on every pointer drag event. |
| `ui.NumberFieldState` | Text buffer and float value (`SetValue`, `Value`) | Keeps intermediate text (like `"3."` or `"-0"`) while typing. |
| `ui.TreeState` | Expanded node set (`SetExpanded`, `Expanded`) | Local disclosure state independent of which node is selected. |
| `ui.DataTableState` | `Sorted bool`, `SortColumn int`, `SortDesc bool` | Interactive header sort state applied to `Rows`. |
| `widget.Editor` | Text buffer, caret, and selection | Owned by Gio's editor for `TextField`, `TextArea`, and `Combobox`. |
| `x.LogViewState` | `Filter string`, `MinLevel LogLevel`, `NoTime bool` | Drives the incremental log filter index directly. |

### Event Delivery

`state.Update(gtx)` is called in the handle pass before layout. Pick components (`Menu`, `Select`, `Tabs`, `Toolbar`, `Breadcrumbs`, `Accordion`) consume and return one event per `Update` call; value components (`Slider`, `NumberField`) apply all inputs from the frame in order and report the final value once.

Because focus-group keyboard actions (`ActMoveUp`, `ActMoveDown`, `ActConfirm`) run inside `Window.End(gtx)` after layout, a key pick is queued on the component's state and `kit.Notify(gtx)` schedules an immediate frame. On that frame's handle pass, `state.Update(gtx)` returns the queued pick.

A few components handle their own input inside `Layout` because it depends on their props or does not report an event to the caller: `Combobox` (filters `Options`), `CommandPalette` (searches and runs `Actions`), `Tooltip` (hover delay timer), `ToastHost` (toast expiry and dismiss button), and `Split` (updates `SplitState.Ratio`).

## Identity, Scopes, and Semantics

`giokit` keeps test IDs, state keys, and spoken screen-reader labels separate:

| Role | Mechanism | Example |
|---|---|---|
| Test and automation ID | Scoped path registered with `kit.Tag(gtx, id)` | `"invoice:42/items/delete"` |
| State key | Struct field or `kit.Pool[T]` keyed by domain ID | `v.deleteBtns.Get(item.ID)` |
| Spoken label | `semantic.LabelOp` via `kit.TagLabel` or `Label` / `Text` | `"Delete line item"` |

`kit.Scope(gtx, name, widget)` pushes a path segment onto the frame's scope stack: inside `Scope(gtx, "sidebar", ...)`, a control with `ID: "save"` gets the full path `"sidebar/save"`. Composite components name their sub-parts with `kit.Part(id, parts...)` (`qty/minus`, `qty/plus`, `lang/field`). Literal `/` and `%` characters inside a segment are escaped (`%2F`, `%25`) with `kit.EscapeID`. Because scoping only affects semantic paths and not state keys, moving a component into or out of a `Scope` never resets its state.

A `kit.Role` is the fork's `semantic.ClassOp`, which adds WAI-ARIA roles (`RoleTab`, `RoleTabList`, `RoleOption`, `RoleListbox`, `RoleMenu`, `RoleMenuItem`, `RoleTree`, `RoleTreeItem`, `RoleGrid`, `RoleRow`, `RoleGridCell`, `RoleDialog`, `RoleSlider`, `RoleCombobox`, `RoleStatus`, `RoleImage`, ...) to Gio's five classes. A control adds its role to its clip area beside its `Tag` (`kit.RoleSlider.Add(gtx.Ops)`), so roles live in the semantic tree that kittest and the platform read; `kit.Group`, `kit.List.Role`, `kit.Popup.Role` and `kit.EditorField.Role` take one for the areas they own.

## Focus Groups and Key Dispatch

### `:focus-visible` Rings

`kit.FocusVisible` tracks whether focus came from the pointer or the keyboard (matching CSS `:focus-visible`). Clicking a button with the mouse hides its focus ring until a key is pressed; `Tab` or arrow-key navigation shows the ring immediately.

### Active-Descendant Focus Groups (`kit.FocusGroup`)

Collections (`SelectableList`, `Tree`, `DataTable`, `Tabs`, `Menu`, `Select`) act as a single `Tab` stop. Giving individual rows Gio focus breaks in virtualized lists because once a focused row scrolls offscreen, it is no longer laid out and Gio drops focus.

`kit.FocusGroup` uses an active-descendant model:
- The collection container owns a single `kit.Focus` handle, which stays laid out regardless of scroll offset.
- Rows join the group via `group.Join(rowClickable, isActive)` so they do not register separate `Tab` stops.
- Arrow keys (`ActMoveUp`, `ActMoveDown`, `ActFirst`, `ActLast`) update the active item ID and call `List.ScrollIntoView`.
- While the group holds keyboard focus, the active row draws the focus ring.

### Key Contexts and Bubbling

During layout, views and components wrap subtrees in `kit.KeyContext(gtx, name, widget)` and register handlers with `kit.OnAction(gtx, actionID, fn)` or attach keymaps with `keymap.Attach(gtx)`. Platform shortcut modifiers (`"Shortcut+K"`, `ActSelectAll`, multi-select modifier clicks) use Gio's `key.ModShortcut`, which maps to `Cmd` on macOS and `Ctrl` on Windows, Linux, and WebAssembly.

When `Window.End(gtx)` processes unclaimed keys at the end of the frame, it walks from the `KeyContext` containing the focused control outward to the window root. Because popups and dialogs record their `KeyContext` under the node that opened them, pressing `Escape` (`ActCancel`) closes the innermost open menu or popover first before reaching an outer dialog or page handler.

## Overlay Layers (`Popup`, `Dialog`, and `Layer`)

- `kit.Layer(gtx, offset, widget)` defers drawing until the end of the frame while keeping the widget attached to the caller's `KeyContext` node. Tooltips and toasts use `Layer`.
- `kit.Popup` and `th.Popover(gtx, popup, id, anchor, content)` draw anchored floating panels for `Menu`, `ContextMenu`, `Select`, `Combobox`, and `DateField`. `Popover` reads where the anchor sat in the window last frame (`input.Source.Bounds`, a fork addition) and flips above the anchor or shifts horizontally to stay inside the window bounds. Outside presses hit `id/backdrop` and close the popup.
- `kit.Dialog` and `th.Modal(gtx, dialog, id, page, content)` draw modal dialogs. While open, `page` is laid out inside `th.Disable` beneath `th.Scrim`, focus is trapped inside the dialog, and `Escape` or a backdrop click closes it unless `Dialog.Persistent` is set.

## Themes, OKLCH Palettes, and Design Tokens

A `*kit.Theme` holds all visual tokens read by components:
- `Palette` defines 21 color roles (`Bg`, `Panel`, `Surface`, `Hover`, `Border`, `Divider`, `Fg`, `Muted`, `Dim`, `Accent`, `AccentBg`, `AccentBorder`, `OnAccent`, `Good`, `GoodBg`, `Warn`, `WarnBg`, `Bad`, `BadBg`, `Focus`, `Overlay`, `Scrim`).
- `kit.GeneratePalette(seed)` builds light and dark palettes in OKLCH space with sRGB gamut clipping. `Palette.Legible(dark)` fits text colors against every surface they appear on to meet WCAG AA contrast (at least 4.5:1 for normal text), and `Palette.WithContrast(dark, level)` pushes text toward WCAG AAA (7:1) and borders/focus rings to 3:1.
- `Metrics` and `ControlSize` define sizes for icons, checkboxes, switches, hit targets, and overlay widths. Buttons, text fields, number fields, selects, comboboxes, date fields, and tabs share identical heights at `ControlSmall`, `ControlMedium`, and `ControlLarge`.
- `Strings` holds all built-in labels and format strings (`th.Strings`).
- `kit/fonts` embeds four SIL OFL font pairs (`Go`, `Inter`, `Plex`, `Atkinson`) with system font fallback disabled (`text.NoSystemFonts()`) so text rendering is deterministic across machines.
- `th.Tokens()` and `kit.ThemeFromTokens(data, base)` serialize and parse W3C Design Token (DTCG) JSON files, resolving `{group.token}` aliases and returning `*kit.TokenError` on invalid input.

## Streaming Video

Gio's `paint.NewImageOp` is built for static images: each new `ImageOp` allocates a GPU texture, uploads CPU pixels, and builds mipmaps.

Video streaming uses three components:
1. `paint.Stream` (in `giokit`'s Gio fork, `gio/op/paint`) is a thread-safe frame sink owned by the producer. Any goroutine calls `stream.Submit(paint.StreamFrame{...})`. If the UI has not drawn the previous frame yet, the newest frame replaces it and the old frame's `Release` callback runs immediately. The renderer keeps one persistent texture per stream (without mipmaps) across all five Gio GPU backends (Metal, Direct3D 11, OpenGL ES, Vulkan, and WebGL) and uploads only when the sequence number changes. On Linux OpenGL ES, it also supports zero-copy `DMA-BUF` import; backends without zero-copy import fall back to texture upload automatically.
2. `kit/x/video` (`video.Video`) lays out a `*paint.Stream` with `FitContain`, `FitCover`, `FitFill`, or `FitNone`, tags it with `kit.RoleImage`, and records the frame placement in `video.State` so callers can map pointer coordinates (`st.ToFrame(pt)`) or place overlays (`st.FromFrame(pt)`).
3. `github.com/hhhapz/giokit/media` wraps FFmpeg's `libavcodec` and `libavformat`. It decodes on the GPU through VA-API (Linux), D3D11VA (Windows) or VideoToolbox (macOS), or in software. Under `media.HardwareAuto`, it uses hardware decoding + zero-copy export when the target `paint.Stream` supports it (VA-API DMA-BUFs; Windows and macOS have no zero-copy export yet), and falls back to software decoding + texture upload otherwise, since copying a GPU frame back cost more than decoding it on the CPU when measured with VA-API. `media.HardwareGPU` with `DMABuf` off forces the readback path; `GIOKIT_VIDEO` overrides `HardwareAuto` from the environment.

## Performance and Scaling

`giokit` draws only when something changes (input, an animation, `kit.Notify`, or a posted callback). An idle UI schedules zero frames and uses zero CPU; `kit.Loop` is the only primitive that requests continuous vsync redraws.

Typical gallery pages at `1100×720` dp lay out in 0.12 to 0.45 ms and render a full headless GPU frame in 0.65 to 1.60 ms (`go test ./examples/gallery -run '^$' -bench 'Layout|Frame' -benchmem`), compared to a 16.7 ms frame budget at 60 Hz.

- `th.List` and every collection built on it (`SelectableList`, `Tree`, `DataTable`, `ReorderList`, `LogView`, `Lyrics`) lay out only the rows visible in the viewport.
- `SelectableList`, `Tree`, and `DataTable` copy row IDs and labels into reused buffers on the state struct for keyboard navigation and type-ahead. `DataTable` sorts its rows when a sort column is active (~6 ms per frame at 10,000 rows; for larger tables, sort in your model when the data or sort column changes). `LogView` scans only lines appended since the last frame.
- Gio's `text.Shaper` caches shaped glyph runs across frames.
- Borders (`kit.Border`) and vector icons (`kit.Icon`) are drawn as filled rings and polygons rather than `clip.Stroke`, which Gio re-flattens on the CPU every frame ([pitfalls.md #21](pitfalls.md#21-strokes-ate-half-the-frame)).
