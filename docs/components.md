# giokit reference

Three packages. `C`/`D`/`W` alias `layout.Context`/`Dimensions`/`Widget`.

- **`kit`**: primitives: theme, layout, drawing, text, interaction, focus and
  key dispatch, scrolling, layers, app runner. Everything a component is
  built from.
- **`kit/ui`**: the components, built only from kit's exported API.
- **`kit/x`**: specialised widgets (log view, lyrics).

## App

- `NewApp(id, title, w, h)`: `id` is the Wayland app_id / X11 class.
- `Run(frame)`: does not return. `OnClose(err)` runs when the window closes,
  before the process exits (save, close databases; `defer` in main never runs).
  `Quit()` closes the window. `OnPanic` recovers panics in posted callbacks.
- `Post(f)`: runs `f` on the UI goroutine and triggers a frame.
- `Spawn(app, work, done) *Task`: runs `work(ctx)` off-thread, then `done` on
  the UI goroutine. `task.Cancel()` cancels `ctx` and drops the result (done
  never runs after it), for superseded queries and closed views. With a nil
  app it runs inline, for tests.
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
- **Metrics:** `th.Metrics` (`DefaultMetrics`): icon, checkbox, switch and
  icon-button sizes, hit target, pane minimum, menu/list/overlay/toast/reading/label
  widths. In tokens as group `size`.
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

## Component shape

```go
// handle pass: events first
if i, ok := v.lang.Update(gtx); ok { v.langIdx = i }
if v.save.Clicked(gtx) { save() }
// layout: props are values, rebuilt every frame
ui.Select{ID: "lang", Options: langs, Selected: v.langIdx}.Layout(gtx, th, &v.lang)
ui.Button{ID: "save", Text: "Save", Style: ui.Solid}.Layout(gtx, th, &v.save)
```

- **Props** (the struct) carry the value: controlled. **State** (the pointer)
  holds interaction state only: focus, scroll, hover timing, last frame's
  rows. Buttons and toggles take a plain `*kit.Clickable`.
- **Events** come from `state.Update(gtx)` in the handle pass. A click,
  key or drag is recorded during the frame and reported by the next
  `Update`; components call `Notify`, so that frame follows at once.
- **Ids** are props (`ID`); sub-parts are `ID/part` (`kit.Part`).
  **Labels** (spoken names) default to the visible text; icon-only controls
  require one.
- **Size:** buttons and field-like controls (TextField, NumberField, Select,
  Combobox, DateField) take `Size` (`kit.ControlSmall/Medium/Large`;
  `ControlDefault` is the theme's), and line up at each size.
- **Custom content:** strings are the convenience; `Select.Option`,
  `SelectableList.Item`, `Tree.Node` and `DataTable.Cell` draw anything
  instead (the strings still sort and name the items), via
  `kit.MenuRow.Content` for menu-like rows.
- Every component has a conformance row (`kit/ui/conform_test.go`): role,
  label, hover, click, Tab, Enter/Space, a visible ring, disabled, presets,
  scale, density, extreme styles; menus and selects run the composite
  contract (open, key in, pick once, dismiss, focus returns).

## kit: primitives

**Layout.** `kit.Flex{Axis, Gap, Align, Justify, Pad, PadX, PadY, Bg, Border,
Radius, W, H, MinW, MaxW, MinH, MaxH, FillW, FillH}.Layout(gtx, th, children...)`
is the styled container (gaps between children, padding, optional surface,
size limits; zero values: horizontal, Start, packed). Spacing comes from the
scale: `kit.SpaceN` is N×4dp (`Space0p5`…`Space12`, half steps `0p5` `1p5`
`2p5` `3p5`), passed through `th.Space` so Density applies; the lint rejects
numbers. Also `Row` (middle-aligned), `Col`, `Rigid`, `Flexed`, `Grow`,
`Gap(w, h)`, `Pad(v, h, w)`, `Fixed(dp, w)`, `Stretch(w)`, `Box(gtx, size,
surface, w)`, `Flow(gtx, hgap, vgap, children...)` (wrapping row).

**Drawing.** `Surface{Bg, Border, Radius, Focus, Stroke, Ring}.Layout`;
`th.Styled(Surface{…})` fills theme stroke and ring widths. `Fill`,
`Border(gtx, rect, r, width, c)` (a filled ring, never `clip.Stroke`),
`HLine`, `VLine`, `th.FocusRing(gtx, size, r)`, `th.Card`, `th.Bar`,
`th.Pill`, `th.FloatingPanel` (overlay surface and shadow). Colours:
`th.StateColor(base, st)` (control fills: hover half way to a press),
`th.Highlight(lit)` (rows and cells), `th.Tint`, `th.StatusColors(s)`,
`LerpColor`, `WithAlpha`. `th.Progress(v)` (v<0 indeterminate),
`th.Spinner(size)`. **Icons:** `kit.IconCheck`, `IconChevron{Down,Right,Left,Up}`,
`IconPlus`, `IconMinus`, `IconX`, `IconArrowDown`, `IconSearch`, `IconMenu`,
`IconInfo`, `IconAlert`, `IconCircleCheck`, `IconCircleX`, `IconSun`,
`IconLayout`; type `Icon(gtx, size, color)`.

**Text.** `th.Text(gtx, s, size, color, weight)`, `Paragraph`, `ParagraphLine`,
`Heading`, `Caption` (uppercase), `Code` (one line), `CodeBlock` (wraps),
`th.StyledText(gtx, ...TextSpan)`.

**Interaction.** `Clickable` (`Clicked(gtx)` in the handle pass, `Click()`,
`Focus`, `FocusQuiet`, `FocusOnClick`, `Anim()` a per-control tween);
`th.Press(gtx, c, id, func(gtx, State) D)` makes any drawing a pressable,
focusable target (`State{Hovered, Pressed, Focused, Hover}`); `Hover`;
`th.Disable(gtx, cond, w)` (no input, out of the Tab order, semantics say
disabled, drawn at `DisabledAlpha`); `th.WithLabel`.

**Roles.** `kit.SetRole(gtx, id, kit.RoleTab)`: WAI-ARIA roles beyond
Gio's five classes (listbox/option, tree/treeitem, grid/row/gridcell,
tablist/tab, menu/menuitem, combobox, dialog, slider, link, img, status,
separator…). Every kit component sets them; kittest reads them
(`h.Role`, `h.ByRole`) and the conformance suite requires one; a desktop
screen-reader bridge would export them.

**Ids and semantics.** `Tag(gtx, id)`, `TagLabel(gtx, id, label)`,
`TagArea(gtx, id, w)` (names a region whose own handler takes no pointer
gestures), `Group(gtx, id, label, w)`, `Part(id, parts...)`,
`Scope(gtx, name, w)`, `Path`.

**Focus and keys.** `Focus`, `FocusGroup` (`Join`, `Layout`, `Active`,
`FocusHandle`), `KeyContext(gtx, name, w)`, `OnAction(gtx, act, fn)` with
`ActMoveUp/Down`, `ActFirst/Last`, `ActExpand/Collapse`, `ActConfirm`,
`ActCancel`; `Keymap` (`MustBind`, `BindActions`, `Attach`, `Hint`); `Actions`.
See "Keys, focus and actions" below.

**Scrolling.** `th.List(gtx, l, id, n, item)` (overlay scrollbar, gliding
wheel; ids `id`, `id/scrollbar`, `id/thumb`), `List.ScrollIntoView(i)`,
`th.AnchoredList(gtx, a, id, ids, item)` (keeps the first visible id in place
when items are inserted).

**Layers.** `Popup` + `th.Popover(gtx, p, id, anchor, content)`: anchored
panels placed against window edges, a backdrop (`id/backdrop`) that catches
outside presses, Escape closes the innermost (`Show`, `ShowAt`, `Hide`,
`Toggle`, `Opened`, `Opening`, `DidDismiss`, `FocusOnOpen`, `PanelFocus`;
`MatchWidth`, `KeepFocus`). `Dialog` + `th.Modal(gtx, d, id, page, dialog)`:
the page is laid out disabled and inert while open, Escape and the backdrop
close unless `Persistent`. `kit.Layer(gtx, offset, w)`: draw above
everything (tooltips, toasts); components never call `op.Defer` themselves.

**Building blocks.** `th.Field(gtx, st, w)` (field chrome for any content),
`th.EditorField(gtx, e, id, EditorField{Placeholder, Label, Icon, Lines})`
(an editor as a field; the tagged area is the editor's semantic node),
`th.MenuRow(gtx, c, id, MenuRow{Icon, Label, Hint, Danger, Checked,
Highlighted})`, `th.MenuRowWidth`, `th.ControlPad(size, v)`, `kit.Table{Cols}`
(aligned columns: `Cells`, `Header`, `Row`), `Image` (`Set`, `Op`),
`th.Required(id, value)` → `Validation`.

## kit/ui: components

| Family | Components | Notes |
|---|---|---|
| Buttons | `Button{ID, Text, Icon, Label, Style, Size}`, `IconButton{ID, Icon, Label, Active}`, `Link{ID, Text}`, `Toolbar{ID, Actions}` | Styles `ui.Secondary` (default), `Primary`, `Ghost`, `Danger`, `Solid`, or your own `ButtonStyle` func. `Size` overrides the theme's control size. `*kit.Clickable` state. |
| Toggles | `Checkbox{ID, Label, Checked}`, `Switch{ID, Label, On}`, `Radio{ID, Label, Selected}`, `Slider{ID, Label, Value}` + `SliderState.Update` | Flip your value on click. The slider takes drag, arrows/Home/End, and the wheel while focused. |
| Fields | `TextField{ID, Placeholder, Label, Icon, Password, Size}`, `TextArea{ID, Placeholder, Label, Lines}`, `NumberField{ID, Placeholder, Min, Max, Step}` + `NumberFieldState.Update`, `FieldError{Message}`, `Form{ID, Fields}` | Text fields take a `*widget.Editor`. Forms stack labels below `th.NarrowWidth`. |
| Pickers | `Select{ID, Options, Selected, Option}`, `Combobox{ID, Placeholder, Options}`, `DateField{ID, Value, Placeholder, Format, SundayFirst}` | Select: field `id/field`, options `id/<i>`, list `id/list`. Combobox: options `id/opt/<i>`, typing filters, focus stays in the field. DateField: day, month and year views (title `id/view` zooms out), `id/prev`/`id/next` step a month, year or decade; days `id/YYYY-MM-DD`, months `id/YYYY-MM`, years `id/YYYY`. |
| Menus | `Menu{ID, Items, Trigger}`, `ContextMenu{ID, Items, Content}` + `MenuState.Update`, `CommandPalette{ID, Actions, Page}` | `MenuItem{ID, Label, Icon, Shortcut, Danger, Disabled, Divider}`; items `id/<ItemID>`. The open menu is one focus group: Down/Up enter, Enter picks, Escape/Tab close, focus returns to the trigger. |
| Collections | `SelectableList{ID, Items, Selected, Item}`, `Tree{ID, Roots, Selected, Node}`, `DataTable{ID, Columns, Rows, Selected, Cell}`, `ReorderList{ID, Items, Item}` | One Tab stop each; Up/Down/Home/End move the selection, which `Update` reports; typing jumps to the item starting with the letters (type-ahead; a table matches its first column). `Multiple` + `Chosen` and `UpdateMulti` (lists, trees, tables) select several: Ctrl/Cmd-click toggles, Shift-click and Shift+Up/Down select ranges, Ctrl/Cmd+A all. Tree: Right/Left expand and collapse (WAI-ARIA), `TreeState.SetExpanded`. DataTable: headers `id/sort/<col>` sort. ReorderList: `Update` reports a move, `ui.Move(items, from, to)` applies it. |
| Navigation | `Tabs{ID, Labels, Selected, Style}`, `Disclosure{ID, Title, Trailing, Open, Body}` (+ `Collapsible`), `Accordion{ID, Sections, Open}`, `Breadcrumbs{ID, Path}`, `NavItem{ID, Label, Trailing, Active}`, `Stepper{ID, Label, Value}` | Tabs are one Tab stop: Left/Right move, `TabsSegmented` draws a pill. |
| Overlays | `Tooltip{Text, Content}`, `Toasts` + `ToastHost{Page}`, `DialogContent{Title, Body, Actions}` | Tooltips show after `TooltipDelay`. `toasts.Push(status, title, body)`; close button `toast:<n>/close`; auto-dismiss after `ToastDuration` on the frame clock. Dialogs are kit's `Modal` with `DialogContent` inside. |
| Display | `Badge{Status, Text}`, `Banner{Status, Title, Body}`, `Kbd{Chord}`, `Stat{Title, Value, ValueColor}`, `EmptyState{ID, Title, Body, Action}`, `Artwork{ID, Label, Size}` | Status: `kit.StatusInfo/Good/Warn/Bad`. Artwork draws a `*kit.Image`. |
| Layout | `Split{ID, Axis, Min, A, B}` + `SplitState{Ratio}` | Drag the divider, or focus it and use arrows. |

## kit/x: specialised widgets

| Widget | Notes |
|---|---|
| `x.LogView{ID, Buffer}` + `x.LogViewState{Filter, MinLevel, NoTime}` | Virtualized, monospace, coloured by level; follows the tail, `id/jump` returns to it. `x.LogBuffer` is a fixed-capacity ring (`Append`, `At`, `Len`). |
| `video.Video{ID, Stream, Fit, Label}.Layout(gtx, th, &s)` (`video.VideoState`) | Draws a `paint.Stream` (video, camera, remote screen) filling its constraints; `FitContain/Cover/Fill/None`. No input of its own: `s.ToFrame(pt)` maps a widget point to frame pixels (false in letterbox bars), `s.FromFrame(q)` places overlays on video spots, `s.Content()`, `s.Frame()`. Set the stream's `OnFrame` to `app.Invalidate`. Default label `th.Strings.Video`. | Package `kit/x/video`; needs the Gio fork (`paint.Stream`). |
| `x.Lyrics{ID, Lines, At}` + `x.LyricsState{Follow}` | Timed lines; follows the active line and pauses on manual scroll. `x.ActiveLyric`. |

**Desktop integration (`github.com/hhhapz/giokit/desktop`):** `OpenFile`,
`SaveFile` and `OpenURL`, through the `desktop.Platform` interface:
`desktop.System()` in the app (XDG portals over D-Bus on Linux;
`ErrUnsupported` elsewhere for now), a `*desktop.Fake` in tests (it records
calls and returns what you set). Calls block: run them in `kit.Spawn`.
`ErrCancelled` when the user dismisses a chooser.

`kit.Root(gtx)` marks the window for placement; `kit.App` and kittest call
it. Call it first in your frame only if you drive Gio's window loop yourself.

## Keys, focus and actions

`kit.App` and kittest run every frame inside a `kit.Window`
(`gtx = w.Begin(gtx)`, frame, `w.End(gtx)`). After layout, `End` resolves the
keys no widget claimed: from the focused widget outward through
`kit.KeyContext(gtx, name, w)` nodes, the nearest `Keymap.Attach`ed binding or
`kit.OnAction(gtx, action, fn)` handler runs. The default keymap binds
arrows, Home/End, Enter and Escape to `ActMoveUp/Down`, `ActFirst/Last`,
`ActExpand/Collapse`, `ActConfirm` and `ActCancel`, plus Shift+Up/Down
(`ActExtendUp/Down`) and Ctrl/Cmd+A (`ActSelectAll`) for multiple selection.
`Clickable.Modifiers()` gives the keys held during the last click. Popups lay out inside
their opener's context, so Escape closes the innermost layer first; a modal
dialog also catches keys when nothing is focused.

`kit.FocusGroup` makes a composite one Tab stop with an active item
(`Join` members in the handle pass, `Layout` the container, move `Active`
from `OnAction`). SelectableList, Tree, DataTable, Menu and Select use it;
the gallery's sidebar (`navKeys`) shows it from outside kit. kittest's
`FocusedID` reports the group's active item. Ids are paths: `kit.Part`,
`kit.Scope`.

## Actions and commands

`Actions.Register(Action{ID, Title, Category, Description, Run, Enabled})`
gives commands stable IDs. `Actions.Search(query)` feeds pickers, ranked:
title prefix, a word start, anywhere in the title, category or id, then the
letters in order ("gtp" finds "Go to page"); the palette shows each
category beside its title. `Actions.Run(id)` checks Enabled.
`Keymap.BindActions(actions, defaults, overridesJSON)` binds them to shortcuts;
the JSON object maps action IDs to replacement chords (empty unbinds).
`Keymap.Attach(gtx)` inside a `kit.KeyContext` gives a focused view
precedence over window-wide shortcuts. Wrap the root with
`ui.CommandPalette{ID, Actions, Page}.Layout(gtx, th, &palette)` and bind an
action that calls `palette.Open()`.

The gallery's "Lists, trees, tables" page combines the new list, tree, table, form and
command APIs with a small editable in-memory model.

**Icons:** + `ChevronLeft/Up`, `X`, `ArrowDown`, `Search`, `Menu`, `Info`,
`Alert`, `CircleCheck`, `CircleX`, `Sun`, `Layout`.
