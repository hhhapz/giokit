module github.com/hhhapz/giokit

go 1.24.0

require (
	gioui.org v0.10.3
	gioui.org/x v0.10.3
	github.com/godbus/dbus/v5 v5.2.2
	golang.org/x/image v0.26.0
)

require (
	gioui.org/shader v1.0.9 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/exp/shiny v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

// Patched Gio (see docs/gio-patches.md). Consumers must add the same replace,
// pointing at this directory: replace directives are not inherited.
replace gioui.org => ./third_party/gioui.org
