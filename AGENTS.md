# giokit: agent guide

Cross-platform desktop UI toolkit for Go on Gio, following GPUI's philosophy (see
`docs/architecture.md`). Goal: an expansive, accessible, keyboard-first component
set plus tooling that lets agents verify UIs headlessly.

**Keep this file and `skills/giokit/SKILL.md` small:** every session pays for them.
Put detail in `docs/`, and add a pitfall here only as a one-line rule.

## Map

- **`kit/`:** primitives only, one concern per file: tokens (`theme`, `palette`,
  `color`, `tint`, `metrics`, `spacing`, `strings`, `tokens`), layout and drawing
  (`flex`, `flow`, `draw`, `icon`, `widgets` text, `styledtext`, `table`, `anim`),
  input (`clickable`, `hover`, `focus`, `dispatch`, `keymap`, `actions`, `inputs`,
  `menu` rows), ids (`id`, `semantic`), scrolling (`scroll`, `anchoredlist`,
  `pool`), layers (`overlay`, `root`), runner (`app`, `pacing`, `trace`).
- **`kit/ui/`:** every component, as a props struct built only from kit's
  exported API, one family per file (`button`, `toggles`, `fields`, `menu`,
  `combobox`, `datefield`, `navigation`, `overlays`, `display`; collections:
  `selection` shared by `list`, `tree`, `datatable`, plus `reorder`). `ui.go`
  holds the value/event contract. **`kit/x/`:** specialised widgets (`LogView`,
  `Lyrics`); **`kit/x/video`:** draws `paint.Stream`. **`kit/fonts/`:** bundled
  font sets. **`desktop/`:** native file choosers and links (`desktop.Platform`), with a `Fake`.
- **`kittest/`:** headless harness: `kittest.go` (lifecycle, `Settle`, `RunTasks`),
  `input`, `query`, `assert`, `audit`, `stale` (strict check), `a11y`,
  `contrast`, `golden`, `sheet`; `conform/` is the control contract suite. Its
  `kittest_test.go` recreates each historical bug and proves the harness
  catches it. Keep that list growing.
- **`examples/hello/`:** the README's minimal app and its test (keep them in sync).
- **`examples/gallery/`:** every component, plus its tests. `nav.go` groups
  demos into pages and pages into sidebar categories; "All components" shows
  every demo on one page. New components land as a demo with a test.
- **`gio/`:** our hard fork of Gio (`github.com/hhhapz/giokit/gio/...`), synced
  from upstream by `gio: upstream` snapshot commits (`scripts/gio-sync.sh`,
  `docs/gio-patches.md`). Never squash a snapshot commit.
- **`skills/giokit/SKILL.md`:** portable skill for projects that use giokit.
- **`internal/bench/`:** rendering baselines (editor/terminal text, video).
- **`media/`:** separate module (cgo, FFmpeg): video decoder for `paint.Stream`/
  `video.Video`, with hardware decode and optional zero-copy DMA-BUF import.
- **`docs/`:** `getting-started.md`, `architecture.md`, `components.md`,
  `testing.md`, `pitfalls.md`, `gio-patches.md`, `static-ffmpeg.md` (`-tags
  ffmpeg_static`, `scripts/ffmpeg-static.sh`).

## Commands

```sh
go test ./...                        # all tests, headless GPU (Metal/D3D11/EGL), no window
KITTEST_SHOTS=/tmp/shots go test ./... # + final frame per test, gallery contact sheet
go run ./examples/gallery [-shot x.png]
./examples/gallery/web/build.sh        # browser build (wasm) into that folder; it is the site
```

## Rules

The frame rules in `skills/giokit/SKILL.md` apply here too. Read them before
touching `kit/`. In short:
- Handle input, then draw.
- Read hover only inside the component's callback.
- Notify after a mid-frame mutation.
- The UI goroutine owns state.
- Ids on everything interactive.
- Use `Box`/`Fixed` for fixed sizes; spacing from the `kit.SpaceN` scale.
- Use vector icons.
- Use bundled fonts only.

Contributing:
- **Components go in `kit/ui`** as props structs
  (`ui.X{ID, ...}.Layout(gtx, th, &state)`, events from `state.Update(gtx)`),
  built only from kit's exported API (`th.Press`, `th.Field`, `th.MenuRow`,
  `kit.Layer`, ...). If a component needs something private, export a
  primitive instead. Colors come only from palette tokens; hover from the
  callback's `State`; text from `th.Strings`; sizes from `th.Metrics`.
- **Every component change ships with a gallery test and a conformance row**
  (`kit/ui/conform_test.go`, suite in `kittest/conform`): role, label, hover,
  click, Tab, Enter/Space, ring, disabled, every preset, scale and density;
  layer-opening controls use `conform.Composite`, focus-group collections
  `conform.Group`.
- **`internal/lint` ratchets:** geometry literals and English strings per file
  may only go down (`LINT_BASELINE=1` prints counts); color literals outside
  token files and `clip.Stroke` anywhere are forbidden.
- **Prove new checks work.** When a bug escapes, first add a broken-widget case to
  `kittest_test.go` that fails without the fix. Then generalize the detector into
  `Audit`/strict mode if it can be detected structurally. Then add the one-line
  rule to `SKILL.md`.
- **Never assume visual correctness.** Render it (`-shot`, `Snap`/`SaveSheet`) and
  look.
- **Out of reach headlessly:** compositor-drawn things like cursor images and
  output scaling. Say so and ask the user to check them.
- **API style:** small and explicit. No reflection, no global state beyond the
  theme you pass around. Prefer adding a token over hard-coding a color.
- **Doc comments state contracts:** every export says what it draws, reads and
  consumes; states give zero value, copying and goroutine ownership.
