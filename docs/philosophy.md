# Philosophy

giokit is **immediate mode, with GPUI's ownership and testing discipline**. GPUI
itself is a hybrid: retained entities with observers and subscriptions, plus
elements rebuilt every frame from them. giokit has only the second half. State
lives in the app's own structs, owned by the UI goroutine, and the whole window
is laid out every frame. From GPUI it takes the mechanisms that pay off without
retained state: scoped identity and a key dispatch tree. It does not take
entities or observers for now.

## GPUI ideas, mapped to giokit

| GPUI | giokit today | Direction |
|---|---|---|
| Main thread owns entities; views render from state each frame | UI goroutine owns plain structs (no entities); `Frame` = handle, then layout; whole window every frame | Keep. Never draw from snapshots. Revisit entities only if large multi-pane apps need partial redraw. |
| `cx.notify()` | `kit.Notify(gtx)`; `kit.Clickable` and every component notify when they record an event for the next `Update` | Keep; kittest's strict mode catches a missing Notify. |
| `cx.spawn` / background executor | `kit.Spawn(app, work, done)` with a cancellable `Task` (its context is cancelled and `done` is dropped), `app.Post`; kittest's `h.App()` + `RunTasks` | Cancellation is explicit (`Task.Cancel`); tie it to a view's lifetime only if apps need it. |
| Elements with a fluent style API (`div().flex().p_2()`) | Plain structs: `kit.Flex{Axis, Gap, Pad, Bg, Border, Radius, W, MinW...}` as the styled container, helpers (`Row`, `Col`, `Pad`, `Fixed`, `Box`), components as props structs | Keep structs; no fluent builder, no reflection. |
| Actions + keymap + focus handles | `Actions` registry with stable IDs; `Keymap` with JSON overrides, attached per `KeyContext`; a key dispatch tree (`OnAction`, keys bubble from the focused widget); `Focus`, `FocusGroup`, `FocusVisible`; `ui.CommandPalette` | Single-stroke chords only; add multi-stroke chords (Ctrl+K Ctrl+S) when an app needs them. |
| Theme as data | `Palette`, `TypeScale`, `Metrics`, `Strings` and shape/motion knobs; presets and OKLCH `Generate` with contrast enforced; DTCG token files (`ThemeFromTokens`, `Tokens`); `Derive` restyles a subtree | Add a token when a component needs one; prefer palette roles over per-component tokens. |
| `TestAppContext` / `VisualTestContext` simulate input | `kittest`: real input through Gio's router, semantic queries by id and WAI-ARIA role, strict stale check, audits (labels, targets, occlusion, contrast, glyphs), focus traps, the `conform` suite | A screen-reader bridge that exports kit's roles is not built yet. |
| Text system with font fallback | Bundled font sets only (`kit/fonts`: Go, Inter, Plex, Atkinson), system fonts off, vector icons; the missing-glyph audit | Add font sets, not system fallback. |

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
  (enforced by the conformance suite).
- **Test structurally, not visually:** prefer semantic and frame-stability checks
  that every app inherits. Keep goldens rare.
- **Small, explicit API:** functions over frameworks. Add a token rather than
  hard-coding.
- **Token-conscious docs:** the rules stay in SKILL.md; the detail lives here.
