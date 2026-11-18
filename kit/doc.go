// Package kit is a toolkit layer over Gio for dense, keyboard-first desktop
// tools. It holds the primitives components are built from: theme tokens
// (Theme, Palette, Metrics, Strings), layout and drawing helpers, text,
// focus and key dispatch, scrolling, overlay layers and the App runner. The
// components themselves are in package kit/ui, built only from this
// package's exported API.
//
// A Theme carries every color, size and string a component uses. Build one
// with Dark, Light, Generate or ThemeFromTokens, and restyle a subtree with
// Theme.Derive.
//
// Frames follow a few rules, which the components rely on:
//
//   - Handle input before drawing: read a frame's clicks, keys and edits,
//     update state, then lay out from that state. Hover and press state is
//     read only inside a component's own layout callback, never before it.
//   - A state change made while a frame is drawn must schedule another
//     frame. Clickable does this for clicks; call Notify for anything else.
//   - The UI goroutine owns all app state and themes. Work on other
//     goroutines hands its results back with App.Post or Spawn.
//   - Everything interactive has a stable id (see Tag, Part and Scope), by
//     which tests and automation find it. Ids never key state and are
//     never the spoken label.
package kit
