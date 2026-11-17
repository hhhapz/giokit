# giokit: agent guide

Cross-platform desktop UI toolkit for Go on Gio, following GPUI's philosophy (see
`docs/philosophy.md`). Goal: an expansive, accessible, keyboard-first component
set plus tooling that lets agents verify UIs headlessly.

**Keep this file and `skills/giokit/SKILL.md` small:** every session pays for them.
Put detail in `docs/`, and add a pitfall here only as a one-line rule.

## Map

- **`kit/`:** primitives only: theme tokens, layout, drawing, text, focus and
  key dispatch, scrolling, layer hosts (Popover, Modal, Layer), app runner.
- **`kit/ui/`:** every component, as a props struct built only from kit's
  exported API. **`kit/x/`:** specialised widgets (LogView, Lyrics).
- **`kittest/`:** headless harness (input, queries, strict stale check, audit,
  sheets, goldens). Its `kittest_test.go` recreates each historical bug and proves
  the harness catches it. Keep that list growing.
- **`examples/hello/`:** the README's minimal app and its test (keep them in sync).
- **`examples/gallery/`:** every component, plus its tests. `nav.go` groups
  demos into pages and pages into sidebar categories; "All components" shows
  every demo on one page. New components land as a demo with a test.
- **`third_party/gioui.org/`:** patched Gio v0.10.3 (`docs/gio-patches.md`),
  used by this repo's builds. kit, kit/ui, kit/x and kittest must also pass on
  stock Gio (`scripts/stock-gio.sh`): never use a fork-only API outside
  `kit/x/video` and `kit/forktrace`.
- **`skills/giokit/SKILL.md`:** portable skill for projects that use giokit.
- **`internal/bench`:** rendering baselines (editor/terminal text, video).
- **`media/`:** separate module (cgo, FFmpeg): video decoder for `paint.Stream`/
  `video.Video`, VA-API with zero-copy DMA-BUF import.
- **`docs/`:** `pitfalls.md` (full postmortems), `testing.md`, `components.md`, `performance.md`,
  `philosophy.md` (GPUI mapping), `gio-patches.md`.

## Commands

```sh
go test ./...                        # all tests, headless (EGL), no display
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
- Use Box/Fixed for fixed sizes; spacing from the `kit.SpaceN` scale.
- Use vector icons.
- Use bundled fonts only.

Contributing:
- **Components go in `kit/ui`** as props structs
  (`ui.X{ID, ...}.Layout(gtx, th, &state)`, events from `state.Update(gtx)`),
  built only from kit's exported API (`th.Press`, `th.Field`, `th.MenuRow`,
  `kit.Layer`, ...). If a component needs something private, export a
  primitive instead. Colours come only from palette tokens; hover from the
  callback's `State`; text from `th.Strings`; sizes from `th.Metrics`.
- **Every component change ships with a gallery test and a conformance row**
  (`kit/ui/conform_test.go`, suite in `kittest/conform`): role, label, hover,
  click, Tab, Enter/Space, ring, disabled, every preset, scale and density;
  layer-opening controls use `conform.Composite`.
- **`internal/lint` ratchets:** geometry literals and English strings per file
  may only go down (`LINT_BASELINE=1` prints counts); colours and
  `clip.Stroke` are forbidden outside token files.
- **Prove new checks work.** When a bug escapes, first add a broken-widget case to
  `kittest_test.go` that fails without the fix. Then generalize the detector into
  `Audit`/strict mode if it can be detected structurally. Then add the one-line
  rule to SKILL.md.
- **Never assume visual correctness.** Render it (`-shot`, `Snap`/`SaveSheet`) and
  look.
- **Out of reach headlessly:** compositor-drawn things like cursor images and
  output scaling. Say so and ask the user to check them.
- **API style:** small and explicit. No reflection, no global state beyond the
  theme you pass around. Prefer adding a token over hard-coding a color.
