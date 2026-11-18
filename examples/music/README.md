# Music Lab

`examples/music` is a sample media player UI built with `giokit`.

```sh
go run ./examples/music
go run ./examples/music -shot /tmp/music.png
go test ./examples/music
```

It exercises track selection, playback scrubbing (`ui.Slider`), a drag-and-keyboard reorderable queue (`ui.ReorderList` and `kit.AnchoredList`), context menus (`ui.ContextMenu`), time-synced lyrics (`x.Lyrics`), album artwork (`ui.Artwork`), and theme switching.
