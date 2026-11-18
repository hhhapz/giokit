package kit

import "strings"

// Validation is the result of checking one field.
type Validation struct {
	FieldID string // the field's id
	Message string // the error to show; empty means valid
}

// Required reports value as missing when it is empty after trimming space,
// with the theme's localized message (Strings.Required). Callers combine it
// with their own domain rules.
func (t *Theme) Required(id, value string) Validation {
	if strings.TrimSpace(value) == "" {
		return Validation{FieldID: id, Message: t.Strings.Required}
	}
	return Validation{FieldID: id}
}
