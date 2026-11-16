package kit

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Action is one named command shared by shortcuts, menus, and palettes.
type Action struct {
	ID, Title string
	Run       func()
	Enabled   func() bool // nil means enabled
}

type Actions struct {
	items []Action
	index map[string]int
}

func (a *Actions) Register(action Action) error {
	if action.ID == "" || action.Title == "" || action.Run == nil {
		return fmt.Errorf("kit: action needs an id, title, and Run")
	}
	if a.index == nil {
		a.index = make(map[string]int)
	}
	if _, exists := a.index[action.ID]; exists {
		return fmt.Errorf("kit: duplicate action %q", action.ID)
	}
	a.index[action.ID] = len(a.items)
	a.items = append(a.items, action)
	return nil
}

func (a *Actions) MustRegister(action Action) {
	if err := a.Register(action); err != nil {
		panic(err)
	}
}
func (a *Actions) List() []Action { return append([]Action(nil), a.items...) }
func (a *Actions) Get(id string) (Action, bool) {
	i, ok := a.index[id]
	if !ok {
		return Action{}, false
	}
	return a.items[i], true
}
func (a *Actions) Run(id string) bool {
	act, ok := a.Get(id)
	if !ok || act.Enabled != nil && !act.Enabled() {
		return false
	}
	act.Run()
	return true
}
func (a *Actions) Search(query string) []Action {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []Action
	for _, act := range a.items {
		if act.Enabled != nil && !act.Enabled() {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(act.Title), query) || strings.Contains(strings.ToLower(act.ID), query) {
			out = append(out, act)
		}
	}
	return out
}

// UnknownActionsError lists override ids that name no registered action,
// typically a keymap written by a newer (or older) version of the app.
// BindActions returns it after binding everything else, so callers can log it
// and carry on.
type UnknownActionsError struct{ IDs []string }

func (e *UnknownActionsError) Error() string {
	return fmt.Sprintf("kit: unknown actions in shortcuts (ignored): %s", strings.Join(e.IDs, ", "))
}

// BindActions binds action IDs to chords. A JSON object maps IDs to user
// overrides; an empty chord unbinds an action. Invalid JSON, invalid chords
// and two actions sharing a chord are errors and bind nothing, so a bad
// preference file cannot silently change shortcuts. Override ids that name no
// action are skipped: the rest is bound and an *UnknownActionsError is
// returned, which callers should log rather than treat as fatal.
func (k *Keymap) BindActions(a *Actions, defaults map[string]string, overrides []byte) error {
	var custom map[string]string
	if len(overrides) > 0 {
		if err := json.Unmarshal(overrides, &custom); err != nil {
			return err
		}
	}
	var unknown []string
	for id := range custom {
		if _, ok := a.Get(id); !ok {
			unknown = append(unknown, id)
		}
	}
	slices.Sort(unknown)
	type planned struct {
		action Action
		chord  string
	}
	var binds []planned
	seen := map[string]string{}
	for _, act := range a.List() {
		chord := defaults[act.ID]
		if s, ok := custom[act.ID]; ok {
			chord = s
		}
		if chord == "" {
			continue
		}
		name, mods, err := ParseChord(chord)
		if err != nil {
			return err
		}
		normalized := FormatChord(name, mods)
		if prior := seen[normalized]; prior != "" {
			return fmt.Errorf("kit: %q and %q share shortcut %q", prior, act.ID, normalized)
		}
		seen[normalized] = act.ID
		binds = append(binds, planned{act, chord})
	}
	for _, b := range binds {
		act, chord := b.action, b.chord
		id := act.ID
		if err := k.Bind(chord, act.Title, func() { a.Run(id) }); err != nil {
			return err
		}
	}
	if len(unknown) > 0 {
		return &UnknownActionsError{IDs: unknown}
	}
	return nil
}
