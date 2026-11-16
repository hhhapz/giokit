# Music Lab

A no-audio desktop app sketch that exercises media-oriented giokit APIs in one
screen. The generated cover bitmaps and timed lyric lines are local demo data.

```sh
go run ./examples/music
go run ./examples/music -shot /tmp/music.png
go run ./examples/music -shot /tmp/music-small.png -width 590 -height 860
go test ./examples/music
```

Select a track, use Play/Pause to advance the mock clock, seek with the slider,
right-click a queue row for actions, and reorder with a drag handle, its arrow
keys, or the adjacent buttons. Manual lyric scrolling pauses follow; Follow
resumes it. Switch theme exercises palette tokens. The window stacks the queue
under the hero below 780dp.

## What building it exposed

- An image needs stable state: `Image.Set` creates a Gio image op once, and
  `Artwork` renders it across frames. Alternative text needs an image semantic
  role; the Gio patch and contrast audit now distinguish it from painted text.
- Reordering needs stable IDs and a viewport anchor. `ReorderList` owns its
  current order, and `AnchoredList` preserves the first visible item when IDs
  move. Fixed-height rows make drag targeting predictable; variable-height
  rows, edge auto-scroll and cross-list drops still need design.
- Styled runs use Gio X's text shaper via `gioui.org/x/styledtext`. The wrapper
  supplies theme roles and one semantic label. Inline links, selection,
  mentions and keyboard interaction are still a separate accessibility task.
- Timed lyrics are a view of app-owned playback position. `Lyrics` highlights
  a line and manages following; it has no API or audio service dependency.
- A real player would need asynchronous image decoding, cache policy, actual
  audio playback, a lyrics service, persistence, and platform media controls.
  Those belong beside the toolkit rather than inside its rendering loop.

The design follows Gio's [widget](https://gioui.org/doc/architecture/widget),
[input](https://gioui.org/doc/architecture/input) and
[layout](https://gioui.org/doc/architecture/layout) model: persistent state is
kept outside the frame, input is consumed before drawing, and the frame emits
the current UI. Gio's [common errors](https://gioui.org/doc/learn/common-errors)
specifically call out persisting list state across frames.
