package main

import "github.com/hhhapz/giokit/kit/ui"

// choices makes tab, select and suggestion items whose ids are their
// labels. That suits the gallery's option names, which are fixed, unique
// and not translated; an app with translated labels keeps separate ids.
func choices(labels []string) []ui.Choice {
	items := make([]ui.Choice, len(labels))
	for i, l := range labels {
		items[i] = ui.Choice{ID: l, Label: l}
	}
	return items
}
