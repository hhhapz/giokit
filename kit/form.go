package kit

import (
	"strings"
)

// Validation describes one field error. Empty Message means valid.
type Validation struct{ FieldID, Message string }

// Required validates trimmed text; callers can combine it with domain rules.
func (t *Theme) Required(id, value string) Validation {
	if strings.TrimSpace(value) == "" {
		return Validation{FieldID: id, Message: t.Strings.Required}
	}
	return Validation{FieldID: id}
}
