package kit

import (
	"image"
	"testing"

	"github.com/hhhapz/giokit/gio/op/paint"
)

// TestImageSetNilReleases: clearing an Image must not keep the old image
// alive through its stored operation.
func TestImageSetNilReleases(t *testing.T) {
	var im Image
	im.Set(image.NewRGBA(image.Rect(0, 0, 4, 4)))
	if _, ok := im.Op(); !ok {
		t.Fatal("no image after Set")
	}
	im.Set(nil)
	if _, ok := im.Op(); ok {
		t.Error("image still set after Set(nil)")
	}
	if im.op != (paint.ImageOp{}) {
		t.Error("Set(nil) kept the old image operation")
	}
}
