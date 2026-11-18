package main

import (
	"fmt"
	"testing"

	"github.com/hhhapz/giokit/kittest"
)

func TestPatternsInteractions(t *testing.T) {
	g, h := newHarness(t)
	goTo(t, h, g, "forms")
	h.ExpectID("patterns:toolbar/save")
	h.ExpectID("patterns:toolbar/unavailable")
	h.ExpectID("patterns:form/name")
	expectLabel(t, h, "patterns:link", "Read the component guide")
	expectLabel(t, h, "patterns:form/name/label", "Name")
	expectLabel(t, h, "patterns:toolbar/save", "Save")
	h.Hover("patterns:toolbar/save")
	h.ClickID("patterns:toolbar/save")
	h.ExpectTextIn("patterns:form/name", "Required")
	h.ClickID("patterns:name")
	h.Type("A new item")
	h.ClickID("patterns:toolbar/save")
	if g.status != "Saved pattern: A new item" {
		t.Fatalf("save status: %q", g.status)
	}
	h.ClickID("patterns:toolbar/reset")
	if g.patterns.name.Text() != "" {
		t.Fatalf("reset left name %q", g.patterns.name.Text())
	}
	h.Hover("patterns:link")
	h.ClickID("patterns:link")
	if g.status != "Documentation link activated" {
		t.Fatalf("link status: %q", g.status)
	}
	h.ExpectClean()
	h.ExpectTabReachable()
	goTo(t, h, g, "status")
	h.ClickID("patterns:create")
	h.ExpectText("Item created")
	goTo(t, h, g, "disclosure")
	home := h.ExpectID("patterns:crumbs/home")
	guide := h.ExpectID("patterns:crumbs/guide")
	current := h.ExpectID("patterns:crumbs/components")
	if home.Min.Y != guide.Min.Y || home.Min.Y != current.Min.Y || home.Dy() != current.Dy() {
		t.Fatalf("breadcrumb segments are not aligned: home %v, guide %v, current %v", home, guide, current)
	}
	h.Hover("patterns:crumbs/home")
	h.ClickID("patterns:crumbs/home")
	if g.status != "Breadcrumb: Home" {
		t.Fatalf("breadcrumb status: %q", g.status)
	}
	h.ClickID("patterns:accordion/first")
	if g.patterns.open != "first" {
		t.Fatalf("first accordion section: %q", g.patterns.open)
	}
	h.ExpectText("The first section is open.")
	h.ClickID("patterns:accordion/second")
	if g.patterns.open != "second" {
		t.Fatalf("second accordion section: %q", g.patterns.open)
	}
	if _, ok := h.FindText("The first section is open."); ok {
		t.Fatal("closed accordion body is still visible")
	}
	h.ExpectClean()
	h.ExpectTabReachable()
}

func TestPatternsResponsive(t *testing.T) {
	for _, width := range []int{500, 600, 1100} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			g := newGallery()
			h := kittest.NewT(t, width, 720, g.Frame)
			h.Theme = g.th
			goTo(t, h, g, "forms")
			label := h.ExpectID("patterns:form/name/label")
			field := h.ExpectID("patterns:name")
			if width < 1100 && label.Max.Y > field.Min.Y {
				t.Errorf("width %d: form label is not above field: %v, %v", width, label, field)
			}
			if width == 1100 && label.Max.X > field.Min.X {
				t.Errorf("width %d: form label is not left of field: %v, %v", width, label, field)
			}
			if width == 500 {
				save := h.ExpectID("patterns:toolbar/save")
				locked := h.ExpectID("patterns:toolbar/unavailable")
				if locked.Min.Y <= save.Min.Y {
					t.Errorf("toolbar did not wrap at width %d: %v, %v", width, save, locked)
				}
			}
			h.ExpectClean()
		})
	}
}
