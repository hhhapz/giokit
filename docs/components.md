# Components and API Reference

`giokit` is split across `kit` (primitives), `kit/ui` (standard components), `kit/x` and `kit/x/video` (specialized widgets), `kit/fonts` (bundled font pairs), `desktop` (native OS dialogs and links), and `media` (FFmpeg video decoding). Layout signatures use Gio's `layout.Context`, `layout.Dimensions`, and `layout.Widget` directly.

## Component Conventions

Every component in `kit/ui` and `kit/x` is a struct of props laid out with a `*kit.Theme` and a pointer to its state:

```go
// 1. Handle events before laying out the frame.
if id, ok := v.lang.Update(gtx); ok {
	v.langID = id
}
if v.save.Clicked(gtx) {
	v.saveChanges()
}

// 2. Build props structs each frame and pass state by pointer.
langs := []ui.Choice{
	{ID: "en", Label: "English"},
	{ID: "fr", Label: "Français"},
}
ui.Select{ID: "lang", Options: langs, Selected: v.langID}.Layout(gtx, th, &v.lang)
ui.Button{ID: "save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &v.save)
```

- The props struct carries the controlled value for this frame (`Selected`, `Checked`, `Value`). The state pointer holds persistent interaction state across frames (focus tags, scroll offsets, hover timing, open popups). Zero values of state structs are ready to use; never copy a state struct after its first layout.
- `Select`, `Combobox`, `Tabs`, `SelectableList`, and `ReorderList` take items as `ui.Choice{ID, Label}`. `ID` is the stable identifier passed to `Selected` and returned by `Update(gtx)`; `Label` is the text shown on screen and announced to screen readers.
- Every interactive component requires an `ID`. Sub-parts are addressed as `<ID>/<part>` via `kit.Part` (such as `lang/field` or `qty/plus`). Accessibility labels default to the visible `Text` or `Label`; icon-only controls (`ui.IconButton`) require an explicit `Label`.
- `Button`, `TextField`, `NumberField`, `Select`, `Combobox`, and `DateField` accept a per-instance `Size` (`kit.ControlSmall`, `kit.ControlMedium`, `kit.ControlLarge`, or `kit.ControlDefault` to use `th.Controls`).
- `Select.Option`, `SelectableList.Item`, `Tree.Node`, `DataTable.Cell`, and `ReorderList.Item` accept optional custom rendering callbacks when plain text is not enough.

## `kit`: Primitives

### Application Runner and Concurrency

- `kit.NewApp(id, title string, w, h unit.Dp) *kit.App`: creates a single-window runner where `id` is the platform application identifier (such as `"com.example.app"`).
- `app.Run(frame func(layout.Context)) error`: runs the window loop on the calling goroutine and returns when the window closes. Start `app.Run` on a goroutine and call `kit.Main()` at the end of `main()`.
- `app.Post(f func())`: runs `f` on the UI goroutine before the next frame and triggers a redraw. `app.OnPanic` optionally recovers panics in posted callbacks.
- `app.Quit()` and `app.Invalidate()`: close the window or request a redraw from any goroutine. `app.Pending()` counts running `Spawn` tasks; `app.Queued()` counts queued `Post` callbacks.
- `kit.Spawn(app, work, done) *kit.Task`: runs `work(ctx)` off the UI goroutine and delivers `(T, error)` to `done` on the UI goroutine. `task.Cancel()` cancels `ctx` and drops the result so `done` never runs. With a `nil` app, `work` runs inline.
- `kit.Notify(gtx)`: requests an immediate redraw when state changes mid-frame.
- `kit.Pool[T]`: keeps per-item widget state keyed by a stable string ID (`Get(id)`, `Each(fn)`, `Delete(id)`). Call `Sweep()` once per frame on dynamic data to evict entries unseen for 64 frames.

### Theme, Colors, Fonts, and Design Tokens

Build a theme with `kit.Dark()`, `kit.Light()`, `kit.Presets()`, `kit.Generate(kit.Seed{...})`, `kit.New(palette)`, or `kit.ThemeFromTokens(jsonData, baseTheme)`. Call `th.Derive(func(t *kit.Theme) { ... })` to copy and adjust a theme for a subtree while sharing its text shaper.

- Palette tokens (`th.Palette`): `Bg`, `Panel`, `Surface`, `Hover`, `Overlay`, `Scrim`, `Border`, `Divider`, `Focus`, `Fg`, `Muted`, `Dim`, `Accent`, `AccentBg`, `AccentBorder`, `OnAccent`, `Good`, `GoodBg`, `Warn`, `WarnBg`, `Bad`, `BadBg`. `p.WithContrast(dark, level)` boosts text contrast toward WCAG AAA and borders/focus toward 3:1.
- Type scale (`th.Type`): `Caption` (11sp), `Small` (12sp), `Body` (13sp), `Text` (14sp), `Stat` (20sp), `Title` (22sp). `th.Type.Scaled(factor)` scales all text sizes proportionally. Weights: `kit.Regular`, `kit.Medium`, `kit.SemiBold`, `kit.Bold`.
- Shape, spacing, and motion fields on `Theme`:
  - `Radius` (control corner radius), with `th.RadiusSm()`, `th.RadiusLg()`, and `th.RadiusFull()`.
  - `Stroke` (border width, default 1dp) and `Ring` (focus ring width, default 2dp).
  - `Controls` (`kit.ControlSmall`, `kit.ControlMedium`, `kit.ControlLarge`).
  - `Density` (scales `th.Space(...)`, default 1.0), `Elevation` (0 flat, 1 soft shadow, 2 strong shadow), `Motion` (scales `th.Dur(...)`; `0` disables animations), and `DisabledAlpha`.
- Metrics (`th.Metrics`): standard sizes for icons, checkboxes, switches, icon buttons, minimum hit targets (`HitSize`), pane minimums, and overlay widths.
- Bundled fonts (`kit/fonts`): `fonts.Go()` (default), `fonts.Inter()`, `fonts.Plex()`, and `fonts.Atkinson()`. Apply once at startup with `th.UseFonts(fonts.Inter())`.
- Design tokens: `th.Tokens()` exports DTCG JSON; `kit.ThemeFromTokens(data, base)` reads and validates DTCG JSON, returning `*kit.TokenError` (`Path`, `Err`) on invalid values.

### Layout

`kit.Flex` is the styled row and column container:

```go
kit.Flex{
	Axis: layout.Vertical, Gap: kit.Space2, Pad: kit.Space4,
	Align: layout.Start, Justify: layout.SpaceBetween,
	Bg: th.Panel, Border: th.Border, Radius: th.RadiusLg(),
	MinW: 200, MaxW: 600, FillW: true,
}.Layout(gtx, th, kit.Rigid(header), kit.Flexed(1, body))
```

Spacing constants follow a 4dp scale (`Space0p5` = 2dp, `Space1` = 4dp, `Space1p5` = 6dp, `Space2` = 8dp, `Space2p5` = 10dp, `Space3` = 12dp, `Space3p5` = 14dp, `Space4` = 16dp, `Space5` = 20dp, `Space6` = 24dp, `Space8` = 32dp, `Space10` = 40dp, `Space12` = 48dp) and scale with `th.Density` via `th.Space` or `kit.Flex{Gap, Pad}`.

Additional layout helpers: `kit.Row` (middle-aligned horizontal flex), `kit.Col`, `kit.Rigid`, `kit.Flexed`, `kit.Grow` (horizontal spacer for `Flexed`), `kit.Gap(w, h)`, `kit.Pad(v, h, w)`, `kit.Fixed(dp, w)`, `kit.Stretch(w)`, `kit.Box(gtx, size, surface, w)`, `kit.Flow(gtx, hgap, vgap, children...)` (wrapping row), and `kit.Table{Cols}` (aligned columns).

### Drawing, Text, and Icons

- Surfaces and chrome: `kit.Surface{Bg, Border, Radius, Focus, Stroke, Ring}.Layout`, `th.Styled(Surface{...})`, `kit.Fill`, `kit.Border(gtx, rect, r, width, c)`, `kit.HLine`, `kit.VLine`, `th.FocusRing(gtx, size, r)`, `th.Card`, `th.Bar`, `th.Pill`, `th.FloatingPanel`.
- Color helpers: `th.StateColor(base, st)`, `th.Highlight(lit)`, `th.Tint`, `th.StatusColors(status)`, `kit.LerpColor`, `kit.WithAlpha`.
- Progress and spinners: `th.Progress(gtx, v)` (`v < 0` draws an indeterminate bar) and `th.Spinner(gtx, size)`.
- Text: `th.Text(gtx, s, size, color, weight)`, `th.Heading(gtx, s)`, `th.Caption(gtx, s)`, `th.Paragraph(gtx, s, color)`, `th.ParagraphLine(gtx, s, color, weight)`, `th.Code(gtx, s, size, color)`, `th.CodeBlock(gtx, s, size, color)`, and `th.StyledText(gtx, spans...)`.
- Vector icons (`kit.Icon`): `IconCheck`, `IconChevronDown`, `IconChevronRight`, `IconChevronLeft`, `IconChevronUp`, `IconPlus`, `IconMinus`, `IconX`, `IconArrowDown`, `IconSearch`, `IconMenu`, `IconMore`, `IconInfo`, `IconAlert`, `IconCircleCheck`, `IconCircleX`, `IconSun`, `IconLayout`.

### Interaction, Semantics, Focus, and Key Dispatch

- Click and hover: `kit.Clickable` (`Clicked(gtx)`, `Click()`, `Focus(gtx)`, `FocusQuiet(gtx)`, `FocusOnClick()`, `Modifiers()`, `Anim()`), `th.Press(gtx, c, id, func(gtx, kit.State) layout.Dimensions)`, `kit.HoverArea`, `th.Disable(gtx, cond, w)`, `th.WithLabel`.
- IDs and WAI-ARIA roles: `kit.Tag(gtx, id)`, `kit.TagLabel(gtx, id, label)`, `kit.TagImage(gtx, id, label)`, `kit.TagArea(gtx, id, w)`, `kit.Group(gtx, id, label, w)`, `kit.SetRole(gtx, id, role)`, `kit.Part(id, parts...)`, `kit.Scope(gtx, name, w)`, `kit.Path(gtx, id)`, `kit.EscapeID(s)`.
- Focus and actions: `kit.Focus`, `kit.FocusGroup` (`Join`, `Layout`, `Focus`, `HasFocus`, `FocusHandle`, `RingVisible`), `kit.FocusVisible`, `kit.FocusID(gtx, tag, id)`, `kit.FocusWithin(gtx, w)`, `kit.KeyContext(gtx, name, w)`, `kit.OnAction(gtx, act, fn)` (`ActMoveUp`, `ActMoveDown`, `ActFirst`, `ActLast`, `ActExpand`, `ActCollapse`, `ActConfirm`, `ActCancel`, `ActExtendUp`, `ActExtendDown`, `ActSelectAll`, `ActContextMenu`), `kit.Keymap` (`"Shortcut"` maps to `Cmd` on macOS and `Ctrl` on Windows and Linux), and `kit.Actions`.

### Scrolling, Overlays, and Building Blocks

- Scrolling: `th.List(gtx, list, id, count, item)` (overlay scrollbar and smooth wheel glide; exposes `id`, `id/scrollbar`, `id/thumb`), `list.ScrollIntoView(i)`, and `th.AnchoredList(gtx, anchored, id, ids, item)` (keeps the first visible ID anchored when items are inserted or moved).
- Overlays: `kit.Popup` + `th.Popover(gtx, popup, id, anchor, content)`, `kit.Dialog` + `th.Modal(gtx, dialog, id, page, content)`, and `kit.Layer(gtx, offset, w)`.
- Building blocks: `th.Field(gtx, st, w)`, `th.SizedField(gtx, size, st, w)`, `th.EditorField(gtx, editor, id, EditorField{...})`, `th.MenuRow(gtx, clickable, id, MenuRow{...})`, `th.MenuRowWidth`, `th.FieldPad(size)`, `th.ControlPad(size, v)`, `kit.Image` (`Set`, `Op`), and `th.Required(id, value)`.

## `kit/ui`: Components

### Buttons and Toolbars

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.Button{ID, Text, Icon, Label, Style, Size}` | `*kit.Clickable` | `c.Clicked(gtx)` | Styles are `ui.ButtonStyle` functions: `ui.Secondary` (default), `ui.Primary`, `ui.Solid`, `ui.Ghost`, `ui.Danger`, or a custom function. |
| `ui.IconButton{ID, Icon, Label, Active}` | `*kit.Clickable` | `c.Clicked(gtx)` | Square icon button. `Label` is required. |
| `ui.Link{ID, Text}` | `*kit.Clickable` | `c.Clicked(gtx)` | Inline accent link with underline on hover and focus. |
| `ui.Toolbar{ID, Actions}` | `*ui.ToolbarState` | `st.Update(gtx) (string, bool)` | Wrapping row of `ui.ToolbarAction{ID, Text, Icon, Style, Disabled}`. Sub-IDs: `<ID>/<action.ID>`. |

```go
if actionID, ok := v.toolbar.Update(gtx); ok {
	v.handleToolbar(actionID)
}
ui.Toolbar{
	ID: "editor-tools",
	Actions: []ui.ToolbarAction{
		{ID: "run", Text: "Run", Icon: kit.IconCheck, Style: ui.Primary},
		{ID: "clear", Text: "Clear", Style: ui.Ghost},
	},
}.Layout(gtx, th, &v.toolbar)
```

### Toggles, Sliders, and Steppers

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.Checkbox{ID, Label, Checked}` | `*kit.Clickable` | `c.Clicked(gtx)` | Controlled boolean checkbox. Flip your bool when `Clicked(gtx)` is true. |
| `ui.Switch{ID, Label, On}` | `*kit.Clickable` | `c.Clicked(gtx)` | Animated toggle switch. |
| `ui.Radio{ID, Label, Selected}` | `*kit.Clickable` | `c.Clicked(gtx)` | Single radio button. |
| `ui.Slider{ID, Label, Value}` | `*ui.SliderState` | `st.Update(gtx) (float32, bool)` | `[0, 1]` slider supporting drag, arrow keys, `Home`/`End`, and mouse wheel while focused. |
| `ui.Stepper{ID, Label, Value}` | `*ui.StepperState` | `st.Update(gtx) (int, bool)` | Integer step buttons (`<ID>/minus`, `<ID>/plus`), reporting `-1` or `+1`. |

### Text Fields, Number Fields, and Forms

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.TextField{ID, Placeholder, Label, Icon, Password, Size}` | `*widget.Editor` | `editor.Text()` | Single-line text input (masked when `Password: true`). |
| `ui.TextArea{ID, Placeholder, Label, Lines}` | `*widget.Editor` | `editor.Text()` | Multi-line text editor with minimum height scaled to `Lines`. |
| `ui.NumberField{ID, Placeholder, Label, Min, Max, Step, Size}` | `*ui.NumberFieldState` | `st.Update(gtx) (float64, bool)` | Numeric editor with `<ID>/minus` and `<ID>/plus` buttons, arrow stepping, and bounds validation when `Max > Min`. Set initial value with `st.SetValue(v)`. |
| `ui.FieldError{Message}` | Stateless | None | Draws a validation error below a field when `Message != ""`. |
| `ui.Form{ID, Fields}` | Stateless | None | Responsive layout of `ui.FormField{ID, Label, Hint, Error, Widget}`. Stacks labels above fields below `th.NarrowWidth`. |

### Pickers (`Select`, `Combobox`, `DateField`)

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.Select{ID, Options, Selected, Option, Size}` | `*ui.SelectState` | `st.Update(gtx) (string, bool)` | Dropdown picker over `[]ui.Choice`. Trigger is `<ID>/field`, list is `<ID>/list`, options are `<ID>/<choice.ID>`. |
| `ui.Combobox{ID, Placeholder, Options, Size}` | `*ui.ComboboxState` | `st.Editor.Text()` | Free-text editor with filtered suggestions (`<ID>/opt/<choice.ID>`). Typing filters options; arrows and `Enter` complete into `st.Editor`. |
| `ui.DateField{ID, Value, Placeholder, Format, SundayFirst, Size}` | `*ui.DateFieldState` | `st.Update(gtx) (time.Time, bool)` | Date input with a popover calendar supporting day, month, and year views (`<ID>/view`, `<ID>/prev`, `<ID>/next`, `<ID>/YYYY-MM-DD`). Keeps `Value`'s time zone. |

### Menus and Command Palette

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.Menu{ID, Items, Trigger}` | `*ui.MenuState` | `st.Update(gtx) (string, bool)` | Dropdown menu opened by `Trigger(gtx, openFn)`. Items are `ui.MenuItem{ID, Label, Icon, Shortcut, Danger, Disabled, Divider}`. |
| `ui.ContextMenu{ID, Items, Content}` | `*ui.MenuState` | `st.Update(gtx) (string, bool)` | Wraps `Content` and opens a menu at the pointer on right-click or at the control on `Shift+F10`. |
| `ui.CommandPalette{ID, Actions, Page}` | `*ui.CommandPaletteState` | Runs `Action.Run` | Modal command picker over a `*kit.Actions` registry. Open with `st.Open()`. |

### Collections (`SelectableList`, `Tree`, `DataTable`, `ReorderList`)

Collections act as a single `Tab` stop via `kit.FocusGroup`. Arrow keys, `Home`/`End`, and alphanumeric type-ahead move selection. Setting `Multiple: true` on `SelectableList`, `Tree`, or `DataTable` enables multi-selection (`Cmd`+click on macOS or `Ctrl`+click on Windows/Linux toggles items; `Shift`+click and `Shift`+Up/Down select ranges; `Cmd`+A or `Ctrl`+A selects all).

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.SelectableList{ID, Items, Selected, Chosen, Multiple, Item}` | `*ui.SelectableListState` | `st.Update(gtx) (ui.SelectionEvent, bool)` | Virtualized list of `[]ui.Choice`. Sub-IDs: `<ID>/<choice.ID>`. |
| `ui.Tree{ID, Roots, Selected, Chosen, Multiple, Node}` | `*ui.TreeState` | `st.Update(gtx) (ui.SelectionEvent, bool)` | Tree of `[]ui.TreeNode{ID, Label, Children}`. `Right`/`Left` expand and collapse nodes; `st.SetExpanded(id, open)` sets expansion in code. |
| `ui.DataTable{ID, Columns, Rows, Selected, Chosen, Multiple, Cell}` | `*ui.DataTableState` | `st.Update(gtx) (ui.SelectionEvent, bool)` | Virtualized table with sticky sortable headers (`<ID>/sort/<col.ID>`). Custom sorting uses `DataColumn.Compare`. |
| `ui.ReorderList{ID, Items, Item}` | `*ui.ReorderListState` | `st.Update(gtx) (from, to int, ok bool)` | Drag and keyboard reorderable list (`<ID>/handle/<item.ID>`, `<ID>/up/<item.ID>`, `<ID>/down/<item.ID>`). Apply moves with `ui.Move(slice, from, to)`. |

### Navigation

| Component | State | Event | Notes |
|---|---|---|---|
| `ui.Tabs{ID, Items, Selected, Style}` | `*ui.TabsState` | `st.Update(gtx) (string, bool)` | Tab bar (`ui.TabsUnderline` or `ui.TabsSegmented`). Single `Tab` stop; `Left`/`Right`/`Home`/`End` move between tabs. |
| `ui.Disclosure{ID, Title, Trailing, Open, Body}` | `*ui.Collapsible` | `c.Clicked(gtx)` | Expandable section with animated chevron (`ui.Collapsible` embeds `kit.Clickable`). |
| `ui.Accordion{ID, Sections, Open}` | `*ui.AccordionState` | `st.Update(gtx) (string, bool)` | Stack of mutually exclusive expandable `ui.AccordionSection{ID, Title, Body}`. |
| `ui.Breadcrumbs{ID, Path}` | `*ui.BreadcrumbsState` | `st.Update(gtx) (string, bool)` | Trail of `[]ui.Breadcrumb{ID, Label}`; all items except the last crumb are links. |
| `ui.NavItem{ID, Label, Trailing, Active}` | `*kit.Clickable` | `c.Clicked(gtx)` | Sidebar navigation row with active indicator bar and optional trailing text. |

### Overlays, Display, and Layout

| Component | State | Notes |
|---|---|---|
| `ui.Tooltip{ID, Text, Content}` | `*ui.TooltipState` | Shows a floating label after `ui.TooltipDelay` on hover, or immediately when `Content` holds focus; `Escape` hides it. |
| `ui.ToastHost{ID, Page, Duration}` | `*ui.Toasts` | Draws stacked status notifications pushed via `toasts.Push(status, title, body)` and auto-dismisses them on the frame clock. |
| `ui.DialogContent{Title, Body, Actions}` | Stateless | Standard dialog layout placed inside `th.Modal`. |
| `ui.Badge{Status, Text}` | Stateless | Pill status tag (`kit.StatusInfo`, `StatusGood`, `StatusWarn`, `StatusBad`). |
| `ui.Banner{Status, Title, Body}` | Stateless | Tinted callout box with status icon, title, and body text. |
| `ui.Kbd{Chord}` | Stateless | Keycap badge (pair with `keymap.HintFor("action.id")`). |
| `ui.Stat{Title, Value, ValueColor}` | Stateless | Metric card with a caption label and large figure. |
| `ui.EmptyState{ID, Title, Body, Action}` | `*kit.Clickable` | Centered placeholder card with optional action button (`<ID>/action`). |
| `ui.Artwork{ID, Label, Size}` | `*kit.Image` | Rounded square image with fallback frame and `kit.RoleImage` alt-text semantics. |
| `ui.Split{ID, Axis, Min, A, B}` | `*ui.SplitState` | Resizable two-pane container (`st.Ratio` in `[0, 1]`). Divider (`<ID>`) supports pointer drag and arrow keys. |

## `kit/x` and `kit/x/video`: Specialized Widgets

### `x.LogView` and `x.LogBuffer`

`x.LogView` is a virtualized monospace log viewer backed by a fixed-capacity ring buffer (`x.LogBuffer`):

```go
buf := x.NewLogBuffer(x.DefaultLogCap)
buf.Append(x.LogInfo, "server", "listening on :8080")

x.LogView{ID: "logs", Buffer: buf}.Layout(gtx, th, &v.logState)
```

`v.logState.Filter` and `v.logState.MinLevel` (`x.LogDebug`, `LogInfo`, `LogWarn`, `LogError`) filter lines incrementally, scanning only newly appended entries. The view follows the tail while scrolled to the bottom; scrolling up pauses tailing and shows a floating "Latest" button (`<ID>/jump`).

### `x.Lyrics`

`x.Lyrics` renders time-synced lines (`[]x.LyricLine{At, Text}`) for playback position `At`, highlighting the active line (`x.ActiveLyric`) and scrolling to keep it in view while `LyricsState.Follow` is true. Manual scrolling pauses `Follow`.

### `video.Video` (`github.com/hhhapz/giokit/kit/x/video`)

`video.Video` draws frames from a `*paint.Stream` (requires the Gio fork; see [gio-patches.md](gio-patches.md)):

```go
video.Video{
	ID:     "stream",
	Stream: v.stream,
	Fit:    video.FitContain,
	Label:  "Camera feed",
}.Layout(gtx, th, &v.videoState)
```

Supports `FitContain` (letterboxed), `FitCover`, `FitFill`, and `FitNone` (1:1 centered). `v.videoState.ToFrame(windowPt)` maps window pixel coordinates to video frame coordinates (returning `false` in letterbox bars), and `FromFrame(framePt)` maps frame coordinates back to widget space for overlays.

## `desktop`: Native OS Integration

Package `github.com/hhhapz/giokit/desktop` provides a cross-platform interface (`desktop.Platform`) for native file choosers and URL opening:

- `OpenFile(ctx, desktop.FileOptions{Title, Filters, Multiple, Directory}) ([]string, error)`
- `SaveFile(ctx, desktop.SaveOptions{Title, Name, Folder, Filters}) (string, error)`
- `OpenURL(ctx, url) error`

`desktop.System()` returns the host OS implementation:

- **Linux:** XDG desktop portals over D-Bus. Without a FileChooser portal (no session bus, or no portal implementing it), the choosers run `zenity`, then `kdialog` (`kdialog` first on KDE Plasma).
- **Windows:** the Common Item Dialog (`IFileOpenDialog`/`IFileSaveDialog`) through COM, without cgo; links through `ShellExecute`.
- **macOS:** `NSOpenPanel`/`NSSavePanel` through cgo (macOS 11+); links through `open`. Built with `CGO_ENABLED=0`, the choosers return `desktop.ErrUnsupported`.
- **Elsewhere:** `desktop.ErrUnsupported`.

Every system answers the same way: dismissing a dialog returns `desktop.ErrCancelled`, and when `ctx` ends the call returns `ctx.Err()` at once and closes the dialog as if cancelled. Dialogs have no parent window, so the app keeps drawing while one is open. macOS panels take one set of extensions rather than a filter list, so there the filters' `*.ext` patterns are merged (any other pattern shows every file).

Native dialogs block waiting for user input, so call them inside `kit.Spawn`, never on the UI goroutine. On macOS that would deadlock: the panel runs on the main thread, which may be waiting for the UI goroutine's frame.

```go
v.openTask = kit.Spawn(v.app, func(ctx context.Context) ([]string, error) {
	return v.platform.OpenFile(ctx, desktop.FileOptions{Title: "Open Project"})
}, func(paths []string, err error) {
	if err == nil && len(paths) > 0 {
		v.loadProject(paths[0])
	}
})
```

In tests, pass a `*desktop.Fake` instead of `desktop.System()`. Set `fake.Open`, `fake.Save`, or `fake.Err`, and inspect `fake.Choices` (`[]desktop.Choice`) and `fake.Opened` after `h.RunTasks()`.
