package desktop

import (
	"slices"
	"strings"
)

// This file converts Filters to the forms the native choosers take. It has
// no build constraint so that every conversion is tested on every system.

// globExt returns the extension that a "*.ext" pattern matches ("*.csv"
// gives "csv", "*.tar.gz" gives "tar.gz"), and false for any other pattern
// ("*", "data-*.csv", "*.c?").
func globExt(pattern string) (string, bool) {
	ext, ok := strings.CutPrefix(pattern, "*.")
	if !ok || ext == "" || strings.ContainsAny(ext, "*?[]{}\\/") {
		return "", false
	}
	return ext, true
}

// winFilter is one COMDLG_FILTERSPEC before conversion to UTF-16: the name
// shown in the type list and its semicolon-separated patterns.
type winFilter struct {
	Name string
	Spec string
}

// winFilters converts filters to IFileDialog::SetFileTypes entries. A filter
// without patterns matches every file ("*.*"); a filter without a name is
// shown as its patterns.
func winFilters(filters []Filter) []winFilter {
	out := make([]winFilter, 0, len(filters))
	for _, f := range filters {
		spec := strings.Join(f.Patterns, ";")
		if spec == "" {
			spec = "*.*"
		}
		name := f.Name
		if name == "" {
			name = spec
		}
		out = append(out, winFilter{Name: name, Spec: spec})
	}
	return out
}

// defaultExt returns the extension of the first filter's first "*.ext"
// pattern, or "" when it has none. A save dialog appends it to a name typed
// without one.
func defaultExt(filters []Filter) string {
	if len(filters) == 0 {
		return ""
	}
	for _, p := range filters[0].Patterns {
		if ext, ok := globExt(p); ok {
			return ext
		}
	}
	return ""
}

// macExtensions returns the extensions an NSOpenPanel or NSSavePanel should
// allow for filters, without duplicates and in order. macOS panels take one
// set of content types, not a list of filters, so the filters are merged.
// It returns nil, meaning every file, when filters is empty or any pattern
// is not of the form "*.ext": macOS cannot express it, and hiding files the
// caller asked for is worse than showing extra ones.
func macExtensions(filters []Filter) []string {
	var exts []string
	for _, f := range filters {
		if len(f.Patterns) == 0 {
			return nil
		}
		for _, p := range f.Patterns {
			ext, ok := globExt(p)
			if !ok {
				return nil
			}
			if !slices.Contains(exts, ext) {
				exts = append(exts, ext)
			}
		}
	}
	return exts
}
