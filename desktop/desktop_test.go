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
	if _, err := p.OpenFile(context.Background(), FileOptions{Title: "Again"}); err != nil {
		t.Fatal(err)
	}
	_ = p.OpenURL(context.Background(), "https://example.com")
	f := p.(*Fake)
	if len(f.Opened) != 1 || len(f.Choices) != 3 {
		t.Fatalf("recorded: %+v", f)
	}
	c := f.Choices
	if c[0].Open == nil || c[0].Open.Title != "Import" || c[1].Save == nil || c[1].Save.Name != "out.csv" || c[2].Open == nil || c[2].Open.Title != "Again" {
		t.Fatalf("choices out of order: %+v", c)
	}
	got[0] = "changed"
	if f.Open[0] != "/tmp/a.csv" {
		t.Fatal("OpenFile's answer aliases Fake.Open")
	}
}
