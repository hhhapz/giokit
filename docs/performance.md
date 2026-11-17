# Performance: the cost model

giokit redraws the whole UI every frame it draws, and draws only when
something changed (input, an animation, `Notify`, a posted callback). A
settled UI schedules no frames: kittest's `Settle` proves a page goes idle,
and `kit.Loop` is the only way to animate continuously.

## What a frame costs

Measured 2026-11-17 on the development machine (16 threads, Linux, Mesa),
gallery pages at 1100×720 dp:

| Part | Typical page | Heaviest page | Benchmark |
|---|---|---|---|
| Layout (input handling, layout, recording ops) | 0.12–0.2 ms, ~200 allocs, ~10 KB | 0.45 ms (logs: 2,000 log lines filtered), 0.36 ms (overview), ~700 allocs | `BenchmarkLayout/<page>` |
| Whole frame (layout + input routing + GPU render, headless) | 0.65–0.9 ms | 1.6 ms | `BenchmarkFrame/<page>` |
| Video frame upload, 2960×1848 | see `internal/bench` | | `BenchmarkVideo*` |

Run them with
`go test ./examples/gallery -run '^$' -bench 'Layout|Frame' -benchmem`.
A frame budget is 16.7 ms at 60 Hz (6.9 ms at 144 Hz), so layout uses a
few percent of it.

## What scales with what

- **Visible items, not total items.** Lists (`th.List`, every collection)
  lay out only the rows on screen. A component that walks all its data each
  frame is the exception, and says so: `ui.DataTable` sorts and
  `ui.SelectableList` copies ids every frame (fine to a few thousand rows;
  beyond that keep a sorted model yourself), `x.LogView` filters
  incrementally (only new lines are scanned).
- **Allocations** are mostly closures and flex-child slices built during
  layout: roughly one or two per component per frame, all short-lived. They
  do not grow with the data.
- **Text shaping** is cached by Gio's shaper; unchanged text is cheap.
  Labels lay out twice (measure, then at that size) so their bounds hug the
  text; the second pass hits the cache.
- **Images**: `paint.NewImageOp` uploads a texture once per new image;
  changing images (video) must go through `kit/x/video` and `paint.Stream`,
  which reuse one texture.

## Rules that keep it cheap

- Never `Notify` every frame; animate with `kit.Tween` (finite) or
  `kit.Loop` (continuous).
- Keep per-row state in a `kit.Pool` keyed by id, and `Sweep` it, so state
  for rows that left does not accumulate.
- Do expensive work (I/O, parsing, search) in `kit.Spawn`, and cancel the
  `Task` when its result is no longer wanted.
