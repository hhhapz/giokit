package desktop

import (
	"context"
	"testing"
)

// Code written against Platform runs against a Fake in tests.
func TestFake(t *testing.T) {
	var p Platform = &Fake{Open: []string{"/tmp/a.csv"}, Save: "/tmp/out.csv"}
	got, err := p.OpenFile(context.Background(), FileOptions{Title: "Import"})
	if err != nil || len(got) != 1 || got[0] != "/tmp/a.csv" {
		t.Fatalf("OpenFile: %v %v", got, err)
	}
	if s, _ := p.SaveFile(context.Background(), SaveOptions{Name: "out.csv"}); s != "/tmp/out.csv" {
		t.Fatalf("SaveFile: %q", s)
	}
	_ = p.OpenURL(context.Background(), "https://example.com")
	f := p.(*Fake)
	if len(f.Opened) != 1 || len(f.Choices) != 2 {
		t.Fatalf("recorded: %+v", f)
	}
}
