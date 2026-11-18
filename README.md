# giokit

> [!WARNING]
> **Work in progress:** `giokit` is under active development and is not ready for production use. There are no tagged releases yet, consuming it requires a `replace` directive in `go.mod`, and APIs may still change as remaining platform and packaging issues are resolved.

`giokit` is a cross-platform desktop UI toolkit for Go built on [Gio](https://gioui.org) and modeled after [GPUI](https://www.gpui.rs/).

State lives in plain Go structs owned by the UI goroutine. Each frame handles pending input first and then lays out the window from that state. Components are plain structs paired with explicit state values, styled from theme tokens, and tested headlessly.

![The giokit gallery](docs/img/gallery.png)

## Example

```go
package main

import (
	"fmt"
	"log"
	"os"

	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/hhhapz/giokit/kit"
	"github.com/hhhapz/giokit/kit/ui"
)

type view struct {
	th         *kit.Theme
	count      int
	inc, greet kit.Clickable
	name       widget.Editor
	toasts     ui.Toasts
}

func (v *view) Frame(gtx layout.Context) {
	// 1. Handle input.
	if v.inc.Clicked(gtx) {
		v.count++
	}
	if v.greet.Clicked(gtx) {
		v.toasts.Push(kit.StatusGood, "Hello, "+v.name.Text(), "")
	}

	// 2. Draw from state.
	th := v.th
	kit.Fill(gtx, th.Bg, func(gtx layout.Context) layout.Dimensions {
		return ui.ToastHost{Page: func(gtx layout.Context) layout.Dimensions {
			return kit.Flex{Axis: layout.Vertical, Gap: kit.Space3, Pad: kit.Space6}.Layout(gtx, th,
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return th.Heading(gtx, fmt.Sprintf("Clicked %d times", v.count))
				}),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.Button{ID: "inc", Text: "Click me", Style: ui.Solid}.Layout(gtx, th, &v.inc)
				}),
				kit.Rigid(kit.Fixed(240, func(gtx layout.Context) layout.Dimensions {
					return ui.TextField{ID: "name", Placeholder: "Your name"}.Layout(gtx, th, &v.name)
				})),
				kit.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ui.Button{ID: "greet", Text: "Greet"}.Layout(gtx, th, &v.greet)
				}),
			)
		}}.Layout(gtx, th, &v.toasts)
	})
}

func main() {
	v := &view{th: kit.Dark()}
	a := kit.NewApp("hello", "Hello", 480, 320)
	go func() {
		if err := a.Run(v.Frame); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	kit.Main()
}
```

Testing it headlessly with `kittest` (`examples/hello/main_test.go`):

```go
func TestHello(t *testing.T) {
	v := &view{th: kit.Dark()}
	h := kittest.NewT(t, 480, 320, v.Frame)
	h.Theme = v.th

	h.ClickID("inc")
	h.ClickID("inc")
	h.ExpectText("Clicked 2 times")

	h.ClickID("name")
	h.Type("Ada")
	h.ClickID("greet")
	h.ExpectText("Hello, Ada")

	h.ExpectClean()
	h.ExpectTabReachable()
}
```

## Running Examples and Tests

```sh
go run ./examples/hello
go run ./examples/gallery
go run ./examples/music
go test ./...
```

## Installation

```go
require github.com/hhhapz/giokit v0.0.0
replace github.com/hhhapz/giokit => /path/to/giokit
```

`giokit` works with stock Gio. An optional patched Gio fork in `third_party/gioui.org` adds streaming video support (`kit/x/video` and `media`), desktop editor keybindings, and Linux/Wayland fixes (see [docs/gio-patches.md](docs/gio-patches.md)):

```go
replace gioui.org => /path/to/giokit/third_party/gioui.org
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Architecture](docs/architecture.md)
- [Components and API](docs/components.md)
- [Testing](docs/testing.md)
- [Pitfalls](docs/pitfalls.md)
- [Gio Patches](docs/gio-patches.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT, see [LICENSE](LICENSE). The vendored Gio in `third_party/gioui.org` keeps its upstream license (Unlicense OR MIT).
