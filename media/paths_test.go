package media

import (
	"errors"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kittest"
)

var clips = []struct {
	file   string
	w, h   int
	frames int
	red    bool // a solid red picture
}{
	{"h264.mp4", 640, 360, 60, false},
	{"h265.mp4", 640, 360, 60, false},
	{"av1.mp4", 640, 360, 60, false},
	{"odd.mp4", 650, 370, 30, false},
	{"red.mp4", 320, 240, 15, true},
}

var outputPaths = []struct {
	path Path
	opt  Options
}{
	{PathSoftware, Options{Hardware: HardwareOff}},
	{PathReadback, Options{Hardware: HardwareGPU}},
	{PathZeroCopy, Options{Hardware: HardwareGPU, DMABuf: true}},
}

// errSoftware and errNoImport are why a frame did not take the path asked
// for: the GPU decoder fell back to software for the stream, or the
// renderer could not import a DMA-BUF.
var (
	errSoftware = errors.New("frames came back in software")
	errNoImport = errors.New("the renderer cannot import DMA-BUFs")
)

// Every clip through every output path this machine has: software always,
// GPU readback and zero-copy where the platform's API and the renderer
// allow (skipped with the reason otherwise). Each path must report itself
// in Frame.Path, keep the picture size, and match the software picture.
func TestPaths(t *testing.T) {
	for _, c := range clips {
		t.Run(c.file, func(t *testing.T) {
			var ref *image.RGBA
			for _, p := range outputPaths {
				t.Run(p.path.String(), func(t *testing.T) {
					if p.path == PathZeroCopy && !zeroCopyBuild() {
						t.Skip("no DMA-BUF export on this platform")
					}
					first := decodePath(t, c.file, p.path, p.opt, c.w, c.h, c.frames)
					if p.path == PathSoftware {
						ref = first
					}
					if c.red {
						px := first.PixOffset(c.w/2, c.h/2)
						if got := first.Pix[px : px+4]; got[0] < 245 || got[1] > 10 || got[2] > 10 || got[3] != 255 {
							t.Errorf("red decodes to %v", got)
						}
					}
					if ref == nil {
						t.Fatal("no software reference")
					}
					// Readback goes through the same swscale conversion as
					// software. Zero-copy converts on the GPU (VPP), so it
					// is compared away from edges, where chroma upsampling
					// differs (both valid).
					switch p.path {
					case PathReadback:
						if diff := meanDiff(ref, first); diff > 1.5 {
							t.Errorf("first frame differs from software by %.2f per channel", diff)
						}
					case PathZeroCopy:
						if diff := flatDiff(ref, first); diff > 3 {
							t.Errorf("first frame on screen differs from software by %.2f per channel", diff)
						}
					}
				})
			}
		})
	}
}

// decodePath decodes testdata/name with opt, checks that every frame took
// path want at size w×h, and returns the first picture (for zero-copy, as
// the headless renderer draws it). It skips, with the reason, where a GPU
// path is unavailable.
func decodePath(t *testing.T, name string, want Path, opt Options, w, h, frames int) *image.RGBA {
	t.Helper()
	f, err := OpenFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := NewDecoder(f.Codec, opt)
	if err != nil {
		if want == PathSoftware {
			t.Fatal(err)
		}
		t.Skipf("no %v decoder for %v here: %v", PlatformHardware(), f.Codec, err)
	}
	defer d.Close()
	if want != PathSoftware && d.HardwareAPI() != PlatformHardware() {
		t.Fatalf("decoder API %v, want %v", d.HardwareAPI(), PlatformHardware())
	}

	var screen *kittest.Harness
	var s *paint.Stream
	if want == PathZeroCopy {
		s = paint.NewStream()
		screen, err = kittest.New(w, h, 1, func(gtx layout.Context) {
			defer clip.Rect{Max: s.Size()}.Push(gtx.Ops).Pop()
			s.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
		})
		if err != nil {
			t.Skipf("no headless renderer: %v", err)
		}
		defer screen.Release()
	}

	var first *image.RGBA
	var fail error
	n := 0
	emit := func(fr Frame) {
		k := n
		n++
		if fail != nil {
			fr.Release()
			return
		}
		if got := fr.Path(); got != want {
			fail = errors.New("path " + got.String())
			if got == PathSoftware {
				fail = errSoftware
			}
			fr.Release()
			return
		}
		sz := image.Pt(fr.Width, fr.Height)
		if fr.DMABuf != nil {
			sz = image.Pt(fr.DMABuf.Width, fr.DMABuf.Height)
		} else if fr.Image.Rect.Size() != sz {
			t.Errorf("frame %d: image %v for a %v frame", k, fr.Image.Rect, sz)
		}
		if sz != image.Pt(w, h) {
			t.Errorf("frame %d: %v, want %dx%d", k, sz, w, h)
		}
		switch {
		case k > 0:
			fr.Release()
		case fr.Image != nil:
			first = clone(fr.Image)
			fr.Release()
		default:
			s.Submit(fr.StreamFrame()) // the stream releases it
			screen.Settle()
			if s.ImportState() != paint.ImportSupported {
				fail = errNoImport
				return
			}
			first = clone(screen.Image())
		}
	}
	if c := f.Config(); c != nil {
		err = d.Decode(Packet{Data: c, Config: true}, emit)
	}
	for err == nil {
		var p Packet
		if p, err = f.Read(); err == nil {
			err = d.Decode(p, emit)
		}
	}
	if errors.Is(err, io.EOF) {
		err = d.Flush(emit)
	}
	if err != nil {
		if want == PathSoftware {
			t.Fatal(err)
		}
		t.Skipf("%v cannot decode %s here: %v", d.HardwareAPI(), name, err)
	}
	switch {
	case fail == nil:
	case want == PathZeroCopy && d.DMABufError() != nil:
		t.Skipf("no DMA-BUF export here: %v", d.DMABufError())
	case want != PathSoftware && fail == errSoftware:
		t.Skipf("%v cannot decode %s here: %v", d.HardwareAPI(), name, fail)
	case fail == errNoImport:
		t.Skip(fail)
	default:
		t.Fatalf("want %v frames: %v", want, fail)
	}
	if n != frames {
		t.Fatalf("%d frames, want %d", n, frames)
	}
	return first
}

// Forcing an API other than the platform's fails instead of decoding in
// software; forcing the platform's opens it or fails.
func TestForcedHardware(t *testing.T) {
	for _, hw := range []Hardware{HardwareVAAPI, HardwareD3D11VA, HardwareVideoToolbox, HardwareGPU} {
		native := hw == HardwareGPU || hw == PlatformHardware()
		d, err := NewDecoder(H264, Options{Hardware: hw})
		if err != nil {
			var e *Error
			if !errors.As(err, &e) {
				t.Errorf("%v: %v, want an *Error", hw, err)
			}
			if native {
				t.Logf("%v unavailable here: %v", hw, err)
			}
			continue
		}
		if !native {
			t.Errorf("%v opened on a %v platform", hw, PlatformHardware())
		}
		if got := d.HardwareAPI(); got != PlatformHardware() || !d.Hardware() {
			t.Errorf("%v: decoder API %v", hw, got)
		}
		d.Close()
		if d.HardwareAPI() != HardwareOff {
			t.Errorf("%v: API %v after Close", hw, d.HardwareAPI())
		}
	}
}

// Without a usable GPU device (as on CI runners), Auto still decodes, in
// software, while a forced GPU mode fails.
// When the hardware device cannot be created, HardwareAuto decodes in
// software and HardwareGPU fails. Device creation is failed on purpose:
// only VA-API takes a device path that can be made not to exist (D3D11VA
// reads an adapter index and falls back to the default adapter;
// VideoToolbox takes none) (audit 16, CP-07).
func TestAutoFallsBackToSoftware(t *testing.T) {
	skipIfEnvForced(t)
	failHardwareDevices(true)
	defer failHardwareDevices(false)
	d, err := NewDecoder(H264, Options{DMABuf: true})
	if err != nil {
		t.Fatalf("Auto without a device: %v", err)
	}
	defer d.Close()
	if d.HardwareAPI() != HardwareOff {
		t.Errorf("Auto without a device opened %v", d.HardwareAPI())
	}
	if g, err := NewDecoder(H264, Options{Hardware: HardwareGPU}); err == nil {
		g.Close()
		t.Error("HardwareGPU without a device opened")
	}
}

// A VA-API device path that does not exist fails like a missing device.
func TestMissingVAAPIDevice(t *testing.T) {
	skipIfEnvForced(t)
	if PlatformHardware() != HardwareVAAPI {
		t.Skipf("only VA-API takes a device path; this build uses %v", PlatformHardware())
	}
	bad := filepath.Join(t.TempDir(), "no-such-gpu")
	d, err := NewDecoder(H264, Options{DMABuf: true, Device: bad})
	if err != nil {
		t.Fatalf("Auto with a missing device: %v", err)
	}
	defer d.Close()
	if d.HardwareAPI() != HardwareOff {
		t.Errorf("Auto with a missing device opened %v", d.HardwareAPI())
	}
	if g, err := NewDecoder(H264, Options{Hardware: HardwareGPU, Device: bad}); err == nil {
		g.Close()
		t.Error("HardwareGPU with a missing device opened")
	}
}

// skipIfEnvForced skips a test of HardwareAuto's own policy when
// GIOKIT_VIDEO replaces it.
func skipIfEnvForced(t *testing.T) {
	if env := videoEnv(); applyEnv(Options{}, env) != applyEnv(Options{}, "") {
		t.Skipf("GIOKIT_VIDEO=%s overrides HardwareAuto", env)
	}
}

func TestApplyEnv(t *testing.T) {
	auto := Options{DMABuf: true}
	for _, c := range []struct {
		env      string
		in, want Options
	}{
		{"", auto, auto},
		{"auto", auto, auto},
		{"bogus", auto, auto},
		{"off", auto, Options{Hardware: HardwareOff, DMABuf: true}},
		{"software", auto, Options{Hardware: HardwareOff, DMABuf: true}},
		{"readback", auto, Options{Hardware: HardwareGPU}},
		{"zerocopy", Options{}, Options{Hardware: HardwareGPU, DMABuf: true}},
		{"vaapi", Options{}, Options{Hardware: HardwareVAAPI}},
		{"d3d11va", auto, Options{Hardware: HardwareD3D11VA, DMABuf: true}},
		{"videotoolbox", Options{}, Options{Hardware: HardwareVideoToolbox}},
		// An explicit choice in code wins.
		{"software", Options{Hardware: HardwareVAAPI}, Options{Hardware: HardwareVAAPI}},
		{"readback", Options{Hardware: HardwareGPU, DMABuf: true}, Options{Hardware: HardwareGPU, DMABuf: true}},
	} {
		if got := applyEnv(c.in, c.env); got != c.want {
			t.Errorf("GIOKIT_VIDEO=%q on %+v: %+v, want %+v", c.env, c.in, got, c.want)
		}
	}
	// The API names GIOKIT_VIDEO takes are Hardware.String's.
	for _, hw := range []Hardware{HardwareVAAPI, HardwareD3D11VA, HardwareVideoToolbox} {
		if got := applyEnv(Options{}, hw.String()); got.Hardware != hw {
			t.Errorf("GIOKIT_VIDEO=%s: %v", hw, got.Hardware)
		}
	}
}

// GIOKIT_VIDEO reaches NewDecoder: a child test process run with it set
// reports the API its HardwareAuto decoder opened.
func TestEnvOverride(t *testing.T) {
	if want := os.Getenv("GK_ENV_CHILD"); want != "" {
		d, err := NewDecoder(H264, Options{DMABuf: true})
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if got := d.HardwareAPI(); got.String() != want {
			t.Fatalf("GIOKIT_VIDEO=%s opened %v, want %s", os.Getenv("GIOKIT_VIDEO"), got, want)
		}
		return
	}
	for env, want := range map[string]Hardware{"software": HardwareOff, "readback": PlatformHardware()} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestEnvOverride$", "-test.count=1")
		cmd.Env = append(os.Environ(), "GIOKIT_VIDEO="+env, "GK_ENV_CHILD="+want.String())
		if out, err := cmd.CombinedOutput(); err != nil {
			if env == "readback" && strings.Contains(string(out), "open h264 decoder") {
				t.Logf("GIOKIT_VIDEO=readback: no GPU decoder here")
				continue
			}
			t.Errorf("GIOKIT_VIDEO=%s: %v\n%s", env, err, out)
		}
	}
}
