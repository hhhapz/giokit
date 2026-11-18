// Command hello is the smallest useful giokit app: a counter, a text field
// and a toast. Each frame handles input first, then draws from state.
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
