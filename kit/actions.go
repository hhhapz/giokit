package kit

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Action is one named command shared by shortcuts, menus, and palettes.
type Action struct {
	ID, Title   string
	Category    string // groups actions in menus and palettes ("File", "View"); searched too
	Description string // a longer explanation for palettes and help
	Run         func()
	Enabled     func() bool // nil means enabled
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

// Search returns the enabled actions matching query, best first: the title
// starting with it, then a word of the title starting with it, then the
// title, category or id containing it, then its letters in order ("gtp"
// finds "Go to page"). An empty query lists every enabled action in
// registration order.
func (a *Actions) Search(query string) []Action {
	query = strings.ToLower(strings.TrimSpace(query))
	type hit struct {
		act   Action
		score int
	}
	var hits []hit
	for _, act := range a.items {
		if act.Enabled != nil && !act.Enabled() {
			continue
		}
		if sc := matchScore(query, act); sc > 0 {
			hits = append(hits, hit{act, sc})
		}
	}
	slices.SortStableFunc(hits, func(x, y hit) int { return y.score - x.score })
	out := make([]Action, len(hits))
	for i, h := range hits {
		out[i] = h.act
	}
	return out
}

// matchScore ranks how well query (lower case) matches act; 0 is no match.
func matchScore(query string, act Action) int {
	if query == "" {
		return 1
	}
	title := strings.ToLower(act.Title)
	switch {
	case strings.HasPrefix(title, query):
		return 5
	case strings.Contains(" "+title, " "+query):
		return 4
	case strings.Contains(title, query):
		return 3
	case strings.Contains(strings.ToLower(act.Category), query) || strings.Contains(strings.ToLower(act.ID), query):
		return 2
	case subsequence(query, title):
		return 1
	}
	return 0
}

// subsequence reports whether q's runes appear in s in order.
func subsequence(q, s string) bool {
	r := []rune(q)
	i := 0
	for _, c := range s {
		if i < len(r) && c == r[i] {
			i++
		}
	}
	return i == len(r)
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
