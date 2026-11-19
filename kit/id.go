package kit

import (
	"strings"

	"github.com/hhhapz/giokit/gio/layout"
)

// Part is the id of a named part inside the component with id, one segment
// per name: Part("lang", "opt", "3") is "lang/opt/3".
func Part(id string, names ...string) string {
	n := len(id)
	for _, s := range names {
		n += 1 + len(s)
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteString(id)
	for _, s := range names {
		b.WriteByte('/')
		writeSegment(&b, s)
	}
	return b.String()
}

// EscapeID escapes one path segment: "/" and "%" become %2F and %25.
func EscapeID(s string) string {
	if !strings.ContainsAny(s, "/%") {
		return s
	}
	var b strings.Builder
	writeSegment(&b, s)
	return b.String()
}

// writeSegment writes s to b, escaped as EscapeID does.
func writeSegment(b *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '/':
			b.WriteString("%2F")
		case '%':
			b.WriteString("%25")
		default:
			b.WriteByte(s[i])
		}
	}
}

// Scope lays out w with name appended to the id path, so ids inside need not
// know their context and two instances of a composite never collide.
//
// Ids are paths. A component gets a local id from its caller; the parts it
// creates inside (a field, an option, a row) are Part(id, name), and Scope
// prefixes everything laid out within it. Tag emits the full path:
//
//	kit.Scope(gtx, "invoice:42", func(gtx layout.Context) layout.Dimensions {
//		return ui.Button{ID: "pay", Text: "Pay", Style: ui.Solid}.Layout(gtx, th, &v.pay) // id "invoice:42/pay"
//	})
//
// "/" separates levels; ":" is free for "kind:record" names. Part and Scope
// escape their segments, so record ids may contain anything.
//
// Ids name things for tests and automation only. They never key state (that
// is the caller's struct or a Pool keyed by the local id), so moving a widget
// into a scope does not lose its state, and they are never the spoken label.
// Without a kit.Window frame, Scope adds nothing.
func Scope(gtx layout.Context, name string, w layout.Widget) layout.Dimensions {
	f := frameOf(gtx)
	if f == nil {
		return w(gtx)
	}
	prev := f.scope
	if prev == "" {
		f.scope = EscapeID(name)
	} else {
		f.scope = prev + "/" + EscapeID(name)
	}
	defer func() { f.scope = prev }()
	return w(gtx)
}

// Path is the full id for a local id at this point of the layout: the
// current Scope, then id. Empty ids stay empty.
func Path(gtx layout.Context, id string) string {
	if id == "" {
		return ""
	}
	if f := frameOf(gtx); f != nil && f.scope != "" {
		return f.scope + "/" + id
	}
	return id
}
