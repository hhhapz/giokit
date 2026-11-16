# giokit: agent guide

Cross-platform desktop UI toolkit for Go on Gio, following GPUI's philosophy (see
`docs/philosophy.md`). Goal: an expansive, accessible, keyboard-first component
set plus tooling that lets agents verify UIs headlessly.

**Keep this file and `skills/giokit/SKILL.md` small:** every session pays for them.
Put detail in `docs/`, and add a pitfall here only as a one-line rule.

## Map

- **`kit/`:** theme tokens, layout primitives, components, icons, app runner,
  `Spawn`/`Notify`.
- **`kittest/`:** headless harness (input, queries, strict stale check, audit,
  sheets, goldens). Its `kittest_test.go` recreates each historical bug and proves
  the harness catches it. Keep that list growing.
- **`examples/gallery/`:** every component, plus its tests. `nav.go` groups
  demos into pages and pages into sidebar categories; "All components" shows
  every demo on one page. New components land as a demo with a test.
- **`third_party/gioui.org/`:** patched Gio v0.10.3 (`docs/gio-patches.md`).
  Re-apply the patch when bumping Gio.
- **`skills/giokit/SKILL.md`:** portable skill for projects that use giokit.
- **`internal/bench`:** rendering baselines (editor/terminal text).
- **`docs/`:** `pitfalls.md` (full postmortems), `testing.md`, `components.md`,
  `philosophy.md` (GPUI mapping), `roadmap.md`, `gio-patches.md`.
- **`docs/design/`:** accepted or proposed designs. `phase1.md` (ids, focus and
  dispatch, primitives, props) governs new component work.
- **`audit/`:** the 2026-11-16 architecture audit and its progress (`09-plan.md`).

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
- Use Box/Fixed for fixed sizes.
- Use vector icons.
- Use bundled fonts only.

Contributing:
- **Components are `*Theme` methods.** They take an `id` plus `*Clickable`/`*Hover`
  state, and get their color only from palette tokens. They take hover state from
  the callback's `State`.
- **Every component change ships with a gallery test and a conformance row**
  (`kit/conform_test.go`, suite in `kittest/conform`): role, label, hover,
  click, Tab, Enter/Space, ring, disabled, every preset, scale and density.
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

## Roadmap

`docs/roadmap.md`: v0.2 and most M1/M3 work shipped; M2 actions/palette and
M4 list/tree/table subsets are live. Next: router, typeahead, column resize,
and deeper form helpers. Fork-dependent work is in its Backlog.
