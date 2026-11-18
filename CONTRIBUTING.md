# Contributing

Thanks for helping. The rules below are what keeps giokit reliable; they are
the same ones the agent guide (`AGENTS.md`) enforces.

## Before a change

- `go test ./...` passes (headless; no display needed).
- Read the frame rules in `skills/giokit/SKILL.md`: handle input, then draw;
  read hover inside the callback; ids on everything interactive.
- Anything that changes kit's primitives (focus, dispatch, layers, theme)
  is discussed in an issue first.

## A change

- **Small commits,** one component or family each.
- **Components live in `kit/ui`** as props structs built only from kit's
  exported API. If you need something private, export a primitive.
- **Every component ships with a gallery demo and test and a conformance
  row** (`kit/ui/conform_test.go`). Controls that open a layer use
  `conform.Composite`; focus-group collections use `conform.Group`.
- **Document the contract:** every exported identifier has a doc comment
  saying what it draws, what it reads and which events it consumes; state
  types say their zero value, whether they may be copied, and that the UI
  goroutine owns them.
- **No hard-coded colours, sizes or English:** colours from the palette,
  sizes from `th.Metrics` and the `kit.SpaceN` scale, text from
  `th.Strings`. `internal/lint` enforces this (its baselines may only go
  down: `LINT_BASELINE=1 go test ./internal/lint` prints them).
- **Look at it.** Render the gallery (`go run ./examples/gallery -shot
  out.png -page <page>`) or a test's frames (`KITTEST_SHOTS=dir go test`)
  and attach the picture to the change.

## When a bug escapes

1. Add a broken-widget case to `kittest/kittest_test.go` that fails without
   the fix.
2. Fix it, and generalise the detector into `kittest`'s audit or strict
   mode if the bug can be found structurally.
3. Add a one-line rule to `skills/giokit/SKILL.md` and the full story to
   `docs/pitfalls.md`.
