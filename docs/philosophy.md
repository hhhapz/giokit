# Philosophy and roadmap

giokit is **immediate mode, with GPUI's ownership and testing discipline**. GPUI
itself is a hybrid: retained entities with observers and subscriptions, plus
elements rebuilt every frame from them. giokit has only the second half. State
lives in the app's own structs, owned by the UI goroutine, and the whole window
is laid out every frame. From GPUI it takes the mechanisms that pay off without
retained state: scoped identity and a key dispatch tree (designed in
`design/phase1.md`). It does not take entities or observers for now.

## GPUI ideas, mapped to giokit

| GPUI | giokit today | Direction |
|---|---|---|
| Main thread owns entities; views render from state each frame | UI goroutine owns plain structs (no entities); `Frame` = handle, then layout; whole window every frame | Keep. Never draw from snapshots. Revisit entities only if large multi-pane apps need partial redraw. |
| `cx.notify()` | `kit.Notify(gtx)`; `kit.Clickable` notifies on click | Every stateful component notifies on change. |
| `cx.spawn` / background executor | `kit.Spawn(app, work, done)`, `app.Post` | Add cancellation (context tied to the view). |
| Elements with a fluent style API (`div().flex().p_2()`) | Functional helpers (`Row`, `Pad`, `Surface`) | Consider a small style struct; avoid reflection. |
| Actions + keymap + focus handles | `Actions`, context-scoped `Keymap`, CommandPalette and focus rings | Add a router, multistroke chords and richer focus handles. |
| Theme as data | Palette, light/dark presets, density and type-scale tokens | Add theme tooling and per-component tokens. |
| `TestAppContext` / `VisualTestContext` simulate input | `kittest` (semantic queries, strict stale check, focus/Tab audits, a11y snapshot) | Add richer semantic roles and values. |
| Text system with font fallback | Bundled Go fonts only (deterministic) | Bundle an icon font or more vector icons; maybe a UI font with wider coverage. |

## Principles

- **Deterministic rendering:** the same pixels on every machine. Bundled fonts
  only; vector icons.
- **Everything addressable:** every interactive or scrollable element carries a
  semantic id. That id serves tests, screen readers and automation at once.
- **Input bubbles:** Gio already delivers pointer and scroll events to the
  innermost handler that registered for them, and scroll falls through to the
  parent when a handler's range is exhausted. Components *handle* an event by
  registering for it and *propagate* by not registering (or by narrowing the
  range). Keys are first-come in the frame, so focused widgets read theirs
  during layout and the window keymap runs last.
- **Keyboard parity:** anything clickable is reachable and operable by keyboard
  (on the roadmap).
- **Test structurally, not visually:** prefer semantic and frame-stability checks
  that every app inherits. Keep goldens rare.
- **Small, explicit API:** functions over frameworks. Add a token rather than
  hard-coding.
- **Token-conscious docs:** the rules stay in SKILL.md; the detail lives here.

## Roadmap

See `roadmap.md`: findings (cgo per platform, screen readers, shaders), pillars,
milestones and open decisions.
