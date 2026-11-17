# giokit

Cross-platform desktop UI toolkit for Go, built on [Gio](https://gioui.org). It
follows GPUI's philosophy: state owned by the UI thread, views redrawn from state
every frame, and tests that simulate real input.

- **`kit/`:** theme, layout primitives, components, icons, app runner.
- **`kittest/`:** headless test harness. It drives real pointer and keyboard input
  through Gio's router, renders offscreen, and catches stale frames, unlabeled or
  tiny or occluded targets, and missing glyphs automatically.
- **`examples/gallery/`:** every component, with tests.

```sh
go test ./...
go run ./examples/gallery
./examples/gallery/web/build.sh  # browser build: copy examples/gallery/web/ to any static host
```

## Using it from an app

```
require github.com/hhhapz/giokit v0.0.0
replace github.com/hhhapz/giokit => /path/to/giokit   # until there is a tagged release
replace gioui.org => /path/to/giokit/third_party/gioui.org   # patched Gio; required
```

## For AI agents

`AGENTS.md` is the guide for working on this repo.
Projects that use giokit should load `skills/giokit/SKILL.md` (Agent Skills
format). Link or copy it into the agent's skills directory, for example
`.claude/skills/`, `.codex/skills/`, `.opencode/skills/` or `.agents/skills/`.
For agents without skills, reference it from that project's `AGENTS.md`.
Detailed docs are in `docs/`.

## Licence

MIT, see `LICENSE`. The vendored Gio in `third_party/gioui.org` keeps its
own licence (Unlicense OR MIT).
