package kit

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Action is one named application command shared by shortcuts, menus and
// palettes ("file.save", "view.zoom_in"). It is distinct from ActionID,
// which names what a focused control does with a key (move, expand) and
// is resolved through KeyContexts.
type Action struct {
	ID          string      // stable identifier; keymap files refer to it
	Title       string      // display name, shown in menus and palettes
	Category    string      // groups actions in menus and palettes ("File", "View"); searched too
	Description string      // a longer explanation for palettes and help
	Run         func()      // the command, on the UI goroutine; must not be nil
	Enabled     func() bool // nil means enabled
}

// Actions is a registry of commands, in registration order. The zero value
// is an empty registry. Register every action up front; menus, palettes and
// Keymap.BindActions read it. An Actions belongs to the UI goroutine; Run
// and Enabled functions are called there.
type Actions struct {
	items []Action
	index map[string]int
}

// Register adds action. It fails when the id, title or Run is missing, or
// when the id is already registered.
func (a *Actions) Register(action Action) error {
	if action.ID == "" || action.Title == "" || action.Run == nil {
		return fmt.Errorf("kit: action %q needs an id, title, and Run", action.ID)
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

// MustRegister is Register for actions written in code; it panics on error.
func (a *Actions) MustRegister(action Action) {
	if err := a.Register(action); err != nil {
		panic(err)
	}
}

// List returns every action, enabled or not, in registration order. It
// returns a copy: changing it does not change a.
func (a *Actions) List() []Action { return slices.Clone(a.items) }

// Get returns the action with this id.
func (a *Actions) Get(id string) (Action, bool) {
	i, ok := a.index[id]
	if !ok {
		return Action{}, false
	}
	return a.items[i], true
}

// Run runs the action with this id if it exists and is enabled, and reports
// whether it ran.
func (a *Actions) Run(id string) bool {
	if !a.enabled(id) {
		return false
	}
	a.items[a.index[id]].Run()
	return true
}

// enabled reports whether the action with this id exists and is enabled.
func (a *Actions) enabled(id string) bool {
	act, ok := a.Get(id)
	return ok && (act.Enabled == nil || act.Enabled())
}

// Search ranking, best first. A higher score sorts earlier.
const (
	scoreTitlePrefix = 5 // the title starts with the query
	scoreWordPrefix  = 4 // a word of the title does
	scoreTitle       = 3 // the title contains it
	scoreCategoryID  = 2 // the category or id contains it
	scoreLetters     = 1 // the title has its letters in order; or the query is empty
)

// Search returns the enabled actions matching query, best first: the title
// starting with it, then a word of the title starting with it, then the
// title, category or id containing it, then its letters in order ("gtp"
// finds "Go to page"). An empty query lists every enabled action in
// registration order. Matching ignores case.
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
		return scoreLetters
	}
	title := strings.ToLower(act.Title)
	switch {
	case strings.HasPrefix(title, query):
		return scoreTitlePrefix
	case strings.Contains(" "+title, " "+query):
		return scoreWordPrefix
	case strings.Contains(title, query):
		return scoreTitle
	case strings.Contains(strings.ToLower(act.Category), query) || strings.Contains(strings.ToLower(act.ID), query):
		return scoreCategoryID
	case subsequence(query, title):
		return scoreLetters
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
type UnknownActionsError struct {
	IDs []string // the unknown ids, in the overrides' sorted order
}

// Error lists the unknown ids.
func (e *UnknownActionsError) Error() string {
	return fmt.Sprintf("kit: unknown actions in shortcuts (ignored): %s", strings.Join(e.IDs, ", "))
}

// BindActions binds the actions registered in a to chords: defaults maps
// action ids to chords, and overrides, a JSON object of the same shape (the
// user's preferences; nil for none), takes precedence. An empty chord in
// either leaves an action unbound.
//
// BindActions can be called again when the overrides change: it first
// removes every binding it made earlier for a's actions, so a shortcut the
// user cleared stops working. Bindings made with Bind are kept unless an
// action takes their chord.
//
// A binding runs its action only while the action is enabled. While it is
// disabled, the binding does not take its chord: the key bubbles on to
// outer keymaps and action handlers, as if unbound.
//
// Invalid JSON, invalid chords and two actions sharing a chord are errors
// and change nothing, so a bad preference file cannot silently change
// shortcuts. Override ids that name no action are skipped: the rest is
// bound and an *UnknownActionsError is returned, which callers should log
// rather than treat as fatal.
func (k *Keymap) BindActions(a *Actions, defaults map[string]string, overrides []byte) error {
	var custom map[string]string
	if len(overrides) > 0 {
		if err := json.Unmarshal(overrides, &custom); err != nil {
			return fmt.Errorf("kit: decode shortcut overrides: %w", err)
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
		keys   chord
	}
	var binds []planned
	seen := map[chord]string{}
	for _, act := range a.items {
		s := defaults[act.ID]
		if o, ok := custom[act.ID]; ok {
			s = o
		}
		if s == "" {
			continue
		}
		c, err := parseChord(s)
		if err != nil {
			return fmt.Errorf("kit: shortcut for %q: %w", act.ID, err)
		}
		if prior := seen[c]; prior != "" {
			return fmt.Errorf("kit: %q and %q share shortcut %q", prior, act.ID, FormatChord(c.name, c.mods))
		}
		seen[c] = act.ID
		binds = append(binds, planned{action: act, keys: c})
	}
	k.binds = slices.DeleteFunc(k.binds, func(b Binding) bool {
		_, ours := a.index[b.Action]
		return b.Action != "" && ours
	})
	for _, p := range binds {
		id := p.action.ID
		k.add(Binding{
			Title:   p.action.Title,
			Action:  id,
			Run:     func() { a.Run(id) },
			keys:    p.keys,
			enabled: func() bool { return a.enabled(id) },
		})
	}
	if len(unknown) > 0 {
		return &UnknownActionsError{IDs: unknown}
	}
	return nil
}
