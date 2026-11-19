package desktop

import (
	"slices"
	"testing"
)

func TestGlobExt(t *testing.T) {
	for pattern, want := range map[string]string{
		"*.csv":      "csv",
		"*.tar.gz":   "tar.gz",
		"*":          "",
		"*.*":        "",
		"*.":         "",
		"data-*.csv": "",
		"*.c?":       "",
		"*.[ch]":     "",
		"csv":        "",
		"*.a/b":      "",
	} {
		got, ok := globExt(pattern)
		if got != want || ok != (want != "") {
			t.Errorf("globExt(%q) = %q, %v; want %q", pattern, got, ok, want)
		}
	}
}

func TestWinFilters(t *testing.T) {
	got := winFilters([]Filter{
		{Name: "Tables", Patterns: []string{"*.csv", "*.tsv"}},
		{Name: "", Patterns: []string{"*.txt"}},
		{Name: "Everything"},
	})
	want := []winFilter{{"Tables", "*.csv;*.tsv"}, {"*.txt", "*.txt"}, {"Everything", "*.*"}}
	if !slices.Equal(got, want) {
		t.Fatalf("winFilters = %q, want %q", got, want)
	}
	if got := winFilters(nil); len(got) != 0 {
		t.Fatalf("winFilters(nil) = %q", got)
	}
}

func TestDefaultExt(t *testing.T) {
	for _, c := range []struct {
		filters []Filter
		want    string
	}{
		{nil, ""},
		{[]Filter{{Patterns: []string{"*", "*.csv"}}, {Patterns: []string{"*.txt"}}}, "csv"},
		{[]Filter{{Patterns: []string{"*"}}, {Patterns: []string{"*.txt"}}}, ""},
	} {
		if got := defaultExt(c.filters); got != c.want {
			t.Errorf("defaultExt(%v) = %q, want %q", c.filters, got, c.want)
		}
	}
}

func TestMacExtensions(t *testing.T) {
	for _, c := range []struct {
		filters []Filter
		want    []string
	}{
		{nil, nil},
		{[]Filter{{Patterns: []string{"*.csv", "*.tsv"}}, {Patterns: []string{"*.csv", "*.txt"}}}, []string{"csv", "tsv", "txt"}},
		// Patterns macOS cannot express show every file.
		{[]Filter{{Patterns: []string{"*.csv"}}, {Patterns: []string{"*"}}}, nil},
		{[]Filter{{Patterns: []string{"report-*.csv"}}}, nil},
		{[]Filter{{Patterns: []string{"*.csv"}}, {Name: "All"}}, nil},
	} {
		if got := macExtensions(c.filters); !slices.Equal(got, c.want) {
			t.Errorf("macExtensions(%v) = %q, want %q", c.filters, got, c.want)
		}
	}
}
