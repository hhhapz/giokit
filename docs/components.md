# kit reference

Everything is in package `kit`. `C`/`D`/`W` alias `layout.Context`/`Dimensions`/`Widget`.

## App

- `NewApp(id, title, w, h)`: `id` is the Wayland app_id / X11 class.
- `Run(frame)`: does not return.
- `Post(f)`: runs `f` on the UI goroutine and triggers a frame.
- `Spawn(app, work, done)`: runs `work` off-thread, then `done` on the UI
  goroutine. With a nil app it runs inline, for tests.
- `Notify(gtx)`: requests a redraw (GPUI `cx.notify`).
- `Pool[T]`: per-item widget state keyed by a stable id. `Get(id)`, `Each(fn)`.

## Theme

- `Dark()`, or `New(Palette)`.
- **Tokens:** `Bg Panel Surface Hover Border Divider Fg Muted Dim Accent AccentBg
  AccentBorder Good GoodBg Warn Bad`.
  - `Hover` is the table-row hover fill (it differs from both `Bg` and `Panel`,
    so hover shows on a card too).
  - Use `Warn` for "needs attention", and `Good` for done.
- **Sizes:** `SizeCaption` 11, `SizeSmall` 12, `SizeBody` 13, `SizeText` 14,
  `SizeStat` 20, `SizeTitle` 22.
- **Weights:** `Regular Medium SemiBold Bold`.
- **Shape and style knobs** (all exported in `th.Tokens()`):
  - `Radius`: control corners; `RadiusSm()` (checkboxes, menu rows, tabs),
    `RadiusLg()` (cards, menus, dialogs, capped at 16), `RadiusFull()`
    (badges; square when `Radius` is 0). A huge `Radius` gives pill controls.
  - `Stroke` (border width, 1) and `Ring` (focus ring width, 2). Components
    build surfaces with `th.Styled(Surface{...})`, which applies both.
  - `Controls`: `ControlSmall/Medium/Large`. Buttons, fields, selects, tabs and
    menu rows share the height at each size (`TestControlsLineUp`).
  - `Elevation`: shadow under floating panels, 0 flat, 1 soft, 2 strong.
  - `HeadingWeight`; `Type.Scaled(f)` scales every text size.
  - `Palette.WithContrast(dark, level)`: high contrast for any palette (text
    toward AAA, borders and focus at 3:1).
- **Fonts:** bundled only. `kit/fonts` has `Go()` (default), `Inter()` (with
  JetBrains Mono), `Plex()` (IBM Plex Sans and Mono) and `Atkinson()`
  (Atkinson Hyperlegible, with JetBrains Mono), all SIL OFL. Apply with
  `th.UseFonts(fonts.Inter())` once, not per frame (it builds a shaper). Only
  the sets an app calls are linked in. Tokens store the set's name in
  `th.Fonts`; reapply with `fonts.ByName`.
- **Escape hatches:**
  - `MissingGlyphs(s)`: runes the bundled fonts can't draw.
  - `Material()`: the underlying material theme (for editors and the like).

## Layout primitives

- **Flex:**
  - `Row(gtx, …)`: middle-aligned. `Col(gtx, …)`. Children via `Rigid`,
    `Flexed(w, …)`.
  - `Grow`: the flexible spacer.
  - `Gap(w, h)`: a fixed spacer.
- **Sizing:**
  - `Fixed(dp, w)`: exact width.
  - `Stretch(w)`: full width.
  - `Pad(v, h, w)`: insets.
- **Outlines:** `Border(gtx, rect, radius, width, color)`, a filled ring (never `clip.Stroke`).
- **Backgrounds:**
  - `Surface{Bg, Border, Radius}.Layout`: sized to its content.
  - `Fill(gtx, color, w)`.
  - `Box(gtx, size, surface, w)`: a fixed square with centered content.
- **Rules:** `HLine(c)`, `VLine(c)`.
- **Color:** `Lighten(c, d)`.

## Interaction

- `Clickable`: `Clicked(gtx)` (call before layout), `Press(gtx, id, fn(gtx, State))`.
- `Hover`: `Layout(gtx, id, fn(gtx, hovered))`.
- `Tag(gtx, id)`: attaches a semantic description to the current area.
- `th.Disable(gtx, disabled, w)`: lays out `w` disabled. No input reaches it
  (clicks, hover, keys, wheel, drags); it leaves the Tab order; the
  semantics say disabled; it draws at `th.DisabledAlpha` (token
  `disabled-opacity`, default 0.45). Wraps one control or a whole section.
  `Clicked` reports nothing for a disabled `Clickable`, and drops focus it
  had. Custom components get it for free through `Press`; custom input
  handlers should skip the pointer cursor when `!gtx.Enabled()`.
- `TagLabel(gtx, id, label)` attaches a stable test id and a separate
  human-readable semantic label. `IconButtonLabel` supplies that label for
  icon-only buttons. The Gio fork still exposes the id as Android content
  description; a full platform split is a backlog item.

## Components (methods on `*Theme`)

| Component | Notes |
|---|---|
| `Text(gtx, s, size, color, weight)`, `Caption(gtx, s)` | Single line. Caption is uppercase. |
| `Pill(gtx, s, fg, bg)` | Status badge. |
| `Button(gtx, c, id, label, Primary\|Secondary)` | |
| `Checkbox(gtx, c, id, on)` | Vector check. |
| `IconButton(gtx, c, id, icon, active)` | 22dp. Quiet until `active` or hovered. |
| `Stepper(gtx, minus, plus, id, value, color, active)` | Ids `id-`, `id+`. |
| `NavItem(gtx, c, id, label, trailing, color, active)` | Sidebar row. |
| `Stat(title, value, color)` | Returns a widget. Use in `Flexed(1, …)`. |
| `List(gtx, l, id, n, item)` | Overlay scrollbar. `id` names the scroll region. |
| `Bar(gtx, vpad, hpad, w)` | Panel strip. |
| `Table{Cols, Inset, RowPad}` | `.Header(gtx, th)`, `.Row(gtx, th, hover, id, last, cell(gtx, col, hovered))`, `.Cells`. |

**Icons:** `IconCheck`, `IconChevronDown`, `IconChevronRight`, `IconMinus`,
`IconPlus`. Their type is `Icon(gtx, size, color)`. To add one, write a polyline on
a 24-unit grid with `stroke`.

## Overlays (`kit/overlay.go`, `kit/menu.go`)

Panels float above the page (`op.Defer`), placed against their anchor's
window-space bounds from the last frame (patched `Source.Bounds`): below,
flipped above when there's no room, shifted inside the window. A backdrop
(`<id>/backdrop`) catches presses outside: they close the panel and go
nowhere else. Escape closes the innermost layer. Open and pick in the handle
pass, as with every Clickable.

| Component | Notes |
|---|---|
| `Popup` + `Popover(gtx, p, id, anchor, content)` | The primitive. `p.Show/Hide/Toggle/ShowAt(pt)`, `Opened()`, `DidDismiss()`; `MatchWidth`, `KeepFocus`. Panel id `id`. |
| `Menu(gtx, m, id, trigger, []MenuItem)` | Draw the trigger with `&m.Trigger`; `m.Clicked(gtx)` toggles and returns picks. Items `id:<ItemID>`. `MenuItem{ID, Label, Icon, Shortcut, Danger, Disabled, Divider}`. Down/Up enter and move, Enter picks, Escape/Tab close; focus returns to the trigger. Sized to its widest item. |
| `ContextArea(gtx, m, id, w, items)` | Right-click in `w` opens the menu at the pointer. Area id `id/area`. |
| `Select(gtx, s, id, options, selected)` + `s.Changed(gtx)` | Field `id/field`, options `id/<index>`, list `id/list` (scrolls past 300dp). Opens on the selected option. |
| `Dialog` + `Modal(gtx, d, id, page, dialog)` | Near the root. While open the page is laid out disabled (no input, no focus: Tab stays in the dialog) and inert to screen readers; scrim token `Scrim`. Escape/backdrop close unless `Persistent`; `DidDismiss()`. `DialogContent(gtx, title, body, actions...)`. |
| `DatePicker` + `DateField(gtx, d, id, value, placeholder)` + `d.Changed(gtx)` | Calendar popup; field `id/field`, days `id/YYYY-MM-DD`, `id/prev`/`id/next`. Arrows move by day/week, PageUp/Down by month. Weeks start Monday (`SundayFirst`). |
| `TextArea(gtx, e, id, placeholder, lines)` | Multi-line field, at least `lines` tall. |
| `Combobox` + `Combobox(gtx, cb, id, placeholder, options)` | Text field with suggestions (substring match, up to 8); Down/Up choose, Enter/click take, Escape closes; focus stays in the field. Read `cb.Editor.Text()`; options `id/opt/<i>`. |
| `Collapsible` + `Disclosure(gtx, c, id, title, trailing, body)` | Section with a toggling header (chevron, caption, count pill). |
| `Flow(gtx, hgap, vgap, children...)` | Left-to-right layout that wraps (chips, diff tokens). |
| `Toasts` + `ToastHost(gtx, ts, w)` | Wrap the window. `ts.Push(status, title, body)`; bottom right, newest last; close button `toast:<n>/close`; auto-dismiss after `ToastDuration` (frame clock: kittest's `Wait` drives it). |

**Desktop integration (`github.com/hhhapz/giokit/desktop`):** `OpenFile(ctx, FileOptions)` shows
the native file chooser (XDG portal over D-Bus); it blocks, so call it off the
UI goroutine. `ErrCancelled` when dismissed.

`kit.Root(gtx)` marks the window for placement; `kit.App` and kittest call it.
Call it first in your frame only if you drive Gio's window loop yourself.

## App structure (see examples/gallery)

```go
type view struct{ th *kit.Theme; app *kit.App; rows kit.Pool[rowState]; /* model */ }
func (v *view) Frame(gtx kit.C) { v.handle(gtx); v.layout(gtx); /* save if dirty */ }
```

## v0.2 additions

- **Theme:**
  - Tokens: `th.Type.{Caption…Title}`, `Radius`, `Density` (`th.Space(dp)`),
    `Motion` (`th.Dur(d)`), `UI`/`Mono` typefaces.
  - Constructors: `Light()`, `Presets()`, `Generate(Seed)`,
    `GeneratePalette(Seed)`.
  - `th.Derive(func(*Theme))` restyles a subtree.
  - `ThemeFromTokens(json, base)` / `th.Tokens()` for DTCG.
  - Color helpers: `OKLCH`, `Contrast`, `Hex`, `ParseHex`, `LerpColor`,
    `WithAlpha`.
  - `Palette.Legible(dark)` moves each text color in lightness, only as far
    as needed, to meet WCAG AA on every surface it's drawn on (including
    pressed button tints, `PressTint`): Fg 7:1 (`ContrastText`), Muted 6.5:1
    (`ContrastMuted`), Dim, accent and status 4.5:1 (`ContrastAA`). All
    presets and `Dark()` go through it; token files don't (the audit flags
    them).
- **Animation:** `Tween.To(gtx, target, d)` / `ToEase` / `From`, easings
  (`EaseOut`, `EaseInOut`, `EaseOutBack`), `Loop(gtx, period)`,
  `Transition.Layout(gtx, th, key, w)`. `State` gains `Focused` and `Hover`
  (animated 0..1).
- **Keyboard:** see "Keys, focus and actions" below. `Clickable.FocusOnClick` (NavItem sets it: click focuses quietly). `Keymap.MustBind(chord, title, fn)`, `Attach(gtx)` (anywhere in the frame), `Hint(title)`, `Bindings()`; `ParseChord`, `FormatChord`.
- **Components:**

| Component | Notes |
|---|---|
| `Button` kinds `Secondary Primary Ghost Danger Solid`, `IconLabelButton` | focus ring, animated hover |
| `Switch(c, id, label, on)`, `Radio(c, id, label, selected)` | animated |
| `TextField(e, id, placeholder)`, `TextFieldIcon` | wraps `widget.Editor` |
| `Slider(s *kit.Slider, id)` | drag, arrows/Home/End; the wheel while focused (a click focuses it quietly) |
| `Tabs(s *kit.Tabs, id, labels, sel)` | `s.Style = TabsSegmented`; ids `id:label`; `s.Clicked(gtx)` |
| `Progress(v)` (v<0 indeterminate), `Spinner(size)` | `Loop`-driven |
| `Badge(status, text)`, `Banner(status, title, body)`, `Kbd(chord)` | `StatusInfo/Good/Warn/Bad` |
| `Tooltip(tip, text, w)` | shows after `TooltipDelay`, drawn with `op.Defer` |
| `Card`, `Heading`, `Paragraph`, `Code`, `CodeBlock` | `Code` is one line; `CodeBlock` wraps. |
| `Split(s *kit.Split, id, a, b)` | `Axis`, `Ratio`, `Min`; drag or focus + arrows |
| `LogView(v, buf, id)` + `LogBuffer` | ring buffer; `Filter`, `MinLevel`; follows the tail; `id/jump` |
| `List(l *kit.List, id, n, item)` | glides wheel steps; ids `id` (scroll region), `id/scrollbar`, `id/thumb` |
| `CheckboxLabel(c, id, label, on)` | label is part of the target |
| `SelectableList(s, id, []Choice)` | Stable choice IDs, selection, arrow navigation, virtual scrolling; `s.Changed()` |
| `Tree(tr, id, []TreeNode)` | Nested nodes, expand/collapse, arrow navigation and selection; `tr.SetExpanded`, `tr.Changed()` |
| `DataTable(dt, id, []DataRow)` | Uses `dt.Table.Cols` for widths; sortable sticky header, virtual rows, selection; `dt.Changed()` |
| `NumberField(n, id, placeholder)` | Number editor with step buttons, arrows, optional Min/Max; `n.Value`, `n.Error`, `n.Changed()` |
| `PasswordField(e, id, placeholder)` | Masked single-line editor; caller owns the editor and its text |
| `Required(id, value)` / `FieldError(gtx, validation)` | Minimal form validation and error text |

## App patterns

| Component | Notes |
|---|---|
| `Form(id, []FormField)` | Labeled fields in aligned columns; labels stack above controls below 440dp. `FormField` carries ID, label, help, error and a control widget. Controls retain their own IDs and input state. |
| `Toolbar(id, []ToolbarAction)` | Labeled actions wrap onto more lines as space narrows. Each action uses a `*Clickable`, optional icon, button kind and disabled state. Read clicks in the handle pass. |
| `Link(c, id, label)` | Underlined, keyboard-focusable text action. The caller handles `c.Clicked` and chooses the destination. Gio has no Link semantic class, so the semantic class is Button with the supplied label. |
| `EmptyState(id, title, body, action)` | Centered message and optional action widget. |
| `Accordion(a, id, []AccordionSection)` | One open section at a time; headers use Disclosure's click, keyboard and semantic behavior. Set `a.Open` to open a section programmatically. |
| `Breadcrumbs(id, []Breadcrumb)` | Clickable earlier locations and a plain-text current location; whole segments wrap at narrow widths. |

The gallery's Forms page exercises these together at wide and narrow widths.

## Media and live content

| Component | Notes |
|---|---|
| `Image.Set(image.Image)` + `th.Artwork(gtx, image, id, label, size)` | Persistent Gio image op, square crop, rounded theme-aware placeholder and semantic image label. Decode off-thread with `kit.Spawn`; call `Set` on the UI goroutine. Setting nil shows the placeholder. |
| `th.StyledText(gtx, ...TextSpan)` | Wrapping themed spans with size, strong and muted roles, built on `gioui.org/x/styledtext`. Static text; inline links and selection remain separate work. |
| `ReorderList{Items: []Choice}` + `th.ReorderList(gtx, state, id, item)` | Fixed-height 56dp virtual rows with stable IDs. Drag the handle, use its Up/Down keys, or use the focusable arrow buttons. `Changed()` reports old/new indexes for persistence. No auto-scroll while dragging yet. |
| `AnchoredList` + `th.AnchoredList(gtx, state, id, ids, item)` | Preserves the first visible item and pixel offset when stable IDs are inserted or reordered. Caller keeps one state across frames. |
| `Lyrics` + `th.Lyrics(gtx, state, id, lines, position)` | Timed lines sorted by start time; follows the active line and pauses follow on manual scrolling. The app decides when to resume follow. |

`examples/music` uses all five together. Run `go run ./examples/music`, or
`go run ./examples/music -shot /tmp/music.png -width 590 -height 860` to render
the compact layout. Its playback clock and artwork are generated locally; there
is no audio engine or lyrics service.

## Keys, focus and actions

`kit.App` and kittest run every frame inside a `kit.Window`
(`gtx = w.Begin(gtx)`, frame, `w.End(gtx)`). After layout, `End` resolves the
keys no widget claimed: from the focused widget outward through
`kit.KeyContext(gtx, name, w)` nodes, the nearest `Keymap.Attach`ed binding or
`kit.OnAction(gtx, action, fn)` handler runs. The default keymap binds
arrows, Home/End, Enter and Escape to `ActMoveUp/Down`, `ActFirst/Last`,
`ActExpand/Collapse`, `ActConfirm` and `ActCancel`. Popups lay out inside
their opener's context, so Escape closes the innermost layer first; a modal
dialog also catches keys when nothing is focused.

`kit.FocusGroup` makes a composite one Tab stop with an active item
(`Join` members in the handle pass, `Layout` the container, move `Active`
from `OnAction`). SelectableList, Tree, DataTable, Menu and Select use it;
the gallery's sidebar (`navKeys`) shows it from outside kit. kittest's
`FocusedID` reports the group's active item. Ids are paths: `kit.Part`,
`kit.Scope`.

## Actions and commands

`Actions.Register(Action{ID, Title, Run, Enabled})` gives commands stable IDs.
`Actions.Search(query)` feeds pickers; `Actions.Run(id)` checks Enabled.
`Keymap.BindActions(actions, defaults, overridesJSON)` binds them to shortcuts;
the JSON object maps action IDs to replacement chords (empty unbinds).
`Keymap.Attach(gtx)` inside a `kit.KeyContext` gives a focused view
precedence over window-wide shortcuts. Wrap the root with
`th.CommandPalette(gtx, palette, id, actions, page)` and bind an action that
calls `palette.Open()`.

The gallery's "Lists, trees, tables" page combines the new list, tree, table, form and
command APIs with a small editable in-memory model.

**Icons:** + `ChevronLeft/Up`, `X`, `ArrowDown`, `Search`, `Menu`, `Info`,
`Alert`, `CircleCheck`, `CircleX`, `Sun`, `Layout`.
