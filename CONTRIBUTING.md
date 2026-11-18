# Contributing

## Running Tests

`go test ./...` runs the full test suite headlessly without opening a window. On a Linux machine or container without a GPU, set `LIBGL_ALWAYS_SOFTWARE=1`.

Before changing components or primitives, read [docs/architecture.md](docs/architecture.md) and the rules in [skills/giokit/SKILL.md](skills/giokit/SKILL.md):
- Handle input first, update state, and draw last.
- Read hover and press state only inside the component's layout callback.
- Put a stable semantic ID on every interactive or scrollable widget.

If a change alters `kit`'s core primitives (focus, key dispatch, overlay layers, or theming), open an issue to discuss it first.

## Guidelines

- Keep commits small, covering one component, family, or primitive at a time.
- Components live in `kit/ui` (or `kit/x` for specialized widgets) as props structs built from `kit`'s exported API. If a component needs low-level support that `kit` does not expose, export a general primitive from `kit`.
- Every interactive component in `kit/ui` needs a demo and test in `examples/gallery` and a conformance entry in `kit/ui/conform_test.go`.
- Use theme tokens instead of literals: colors from `th.Palette`, dimensions from `th.Metrics` and `kit.SpaceN`, and built-in strings from `th.Strings`. Draw outlines with `kit.Border` rather than `clip.Stroke`.
- Document exported types, functions, and methods: what they draw, what state they read, and which events they consume.
- Render visual changes (`go run ./examples/gallery -shot out.png -page <page>` or `KITTEST_SHOTS=/tmp/shots go test ./...`) and inspect the output.

## Fixing a Bug

1. Add a regression test to `kittest/kittest_test.go` that fails without the fix.
2. Fix the bug, and if the failure can be caught automatically from semantics, pixels, or frame stability, add it to `kittest`'s `Audit` or strict mode.
3. Add a one-line rule to `skills/giokit/SKILL.md` and document the issue in [docs/pitfalls.md](docs/pitfalls.md).
