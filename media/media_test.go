package media

import (
	"errors"
	"image"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// decodeFile decodes every packet of testdata/name, the way a live source
// feeds the decoder: the Config packet first, then frames.
// each returns true to keep the frame (the caller releases it later).
func decodeFile(t *testing.T, name string, opt Options, each func(Frame) bool) (*File, *Decoder, int) {
	t.Helper()
	f, err := OpenFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	d, err := NewDecoder(f.Codec, opt)
	if err != nil {
		t.Skipf("%v: %v", f.Codec, err)
	}
	t.Cleanup(d.Close)
	n := 0
	emit := func(fr Frame) {
		n++
		if each != nil && each(fr) {
			return
		}
		fr.Release()
	}
	if c := f.Config(); c != nil {
		if err := d.Decode(Packet{Data: c, Config: true}, emit); err != nil {
			t.Fatal(err)
		}
	}
	for {
		p, err := f.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Decode(p, emit); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	if err := d.Flush(emit); err != nil {
		t.Fatal(err)
	}
	return f, d, n
}

func TestDecodeSoftware(t *testing.T) {
	for _, c := range []struct {
		file   string
		codec  Codec
		w, h   int
		frames int
	}{
		{"h264.mp4", H264, 640, 360, 60},
		{"h265.mp4", H265, 640, 360, 60},
		{"av1.mp4", AV1, 640, 360, 60},
		{"odd.mp4", H264, 650, 370, 30},
	} {
		t.Run(c.file, func(t *testing.T) {
			last := int64(-1)
			f, d, n := decodeFile(t, c.file, Options{Hardware: HardwareOff}, func(fr Frame) bool {
				if fr.Image == nil || fr.Hardware {
					t.Fatalf("want a software RGBA frame, got %+v", fr)
				}
				if fr.Width != c.w || fr.Height != c.h || fr.Image.Rect.Size() != image.Pt(c.w, c.h) {
					t.Fatalf("frame %dx%d image %v, want %dx%d", fr.Width, fr.Height, fr.Image.Rect, c.w, c.h)
				}
				if fr.PTS <= last {
					t.Fatalf("pts %d after %d: low-delay output must be in order", fr.PTS, last)
				}
				last = fr.PTS
				return false
			})
			if f.Codec != c.codec || d.Hardware() {
				t.Fatalf("codec %v hw %v", f.Codec, d.Hardware())
			}
			if n != c.frames {
				t.Fatalf("%d frames, want %d", n, c.frames)
			}
		})
	}
}

// The test pattern's solid red clip must come out red: wrong matrix or range
// handling shows up as a tint or as washed-out colour.
func TestColourAccuracy(t *testing.T) {
	for _, hw := range []Hardware{HardwareOff, HardwareVAAPI} {
		var got []byte
		decodeFile(t, "red.mp4", Options{Hardware: hw}, func(fr Frame) bool {
			if got == nil {
				p := fr.Image.PixOffset(fr.Width/2, fr.Height/2)
				got = append([]byte{}, fr.Image.Pix[p:p+4]...)
			}
			return false
		})
		if got == nil {
			continue
		}
		if got[0] < 245 || got[1] > 10 || got[2] > 10 || got[3] != 255 {
			t.Errorf("hardware %v: red decodes to %v", hw, got)
		}
	}
}

// The GPU decoder must produce the same picture as the software one.
func TestHardwareMatchesSoftware(t *testing.T) {
	for _, file := range []string{"h264.mp4", "h265.mp4", "av1.mp4", "odd.mp4"} {
		t.Run(file, func(t *testing.T) {
			var sw []*image.RGBA
			decodeFile(t, file, Options{Hardware: HardwareOff}, func(fr Frame) bool {
				sw = append(sw, clone(fr.Image))
				return false
			})
			i := 0
			var worst float64
			_, d, n := decodeFile(t, file, Options{Hardware: HardwareVAAPI}, func(fr Frame) bool {
				if !fr.Hardware {
					t.Fatal("VA-API decoder produced a software frame")
				}
				worst = max(worst, meanDiff(sw[i], fr.Image))
				i++
				return false
			})
			if !d.Hardware() || n != len(sw) {
				t.Fatalf("hw %v, %d frames vs %d", d.Hardware(), n, len(sw))
			}
			if worst > 1.5 {
				t.Fatalf("worst frame differs by %.2f per channel on average", worst)
			}
		})
	}
}

func TestDMABufExport(t *testing.T) {
	var held []Frame
	released := false
	got, fallback := 0, 0
	_, d, n := decodeFile(t, "h264.mp4", Options{Hardware: HardwareVAAPI, DMABuf: true}, func(fr Frame) bool {
		if fr.DMABuf == nil {
			if fr.Image == nil {
				t.Fatal("frame with neither image nor DMA-BUF")
			}
			fallback++
			if !released { // hand the held surfaces back: export resumes
				for _, h := range held {
					h.Release()
				}
				released = true
			}
			return false
		}
		got++
		b := fr.DMABuf
		if b.FD < 0 || b.Width != 640 || b.Height != 360 || b.Pitch < 640*4 {
			t.Fatalf("bad descriptor %+v", b)
		}
		if b.Fourcc != fourcc("AB24") {
			t.Fatalf("fourcc %q, want AB24 (RGBA bytes)", fourccString(b.Fourcc))
		}
		// Hold the first frames without releasing them, as a slow renderer
		// would: once every surface is taken, frames fall back to RGBA
		// instead of failing or overwriting a held surface.
		if !released {
			held = append(held, fr)
			return true
		}
		return false
	})
	if err := d.DMABufError(); err != nil {
		t.Skipf("no DMA-BUF export here: %v", err)
	}
	// 6 surfaces: the first 6 frames export, then frames fall back to RGBA
	// until the held ones are released, then export resumes for the rest.
	if got <= 6 || fallback != 1 || got+fallback != n {
		t.Fatalf("%d DMA-BUF frames, %d fallbacks, %d total", got, fallback, n)
	}
}

func clone(m *image.RGBA) *image.RGBA {
	c := image.NewRGBA(m.Rect)
	copy(c.Pix, m.Pix)
	return c
}

func meanDiff(a, b *image.RGBA) float64 {
	var sum int64
	for i := range a.Pix {
		d := int64(a.Pix[i]) - int64(b.Pix[i])
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return float64(sum) / float64(len(a.Pix))
}

func fourcc(s string) uint32 {
	return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
}

func fourccString(v uint32) string {
	return string([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

func TestAutoPolicy(t *testing.T) {
	skipIfEnvForced(t)
	d, err := NewDecoder(H264, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Hardware() {
		t.Error("Auto without DMA-BUF output should decode in software (copy-back is slower)")
	}
	d2, err := NewDecoder(H264, Options{DMABuf: true})
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	if _, err := NewDecoder(H264, Options{Hardware: HardwareVAAPI}); err == nil && !d2.Hardware() {
		t.Error("Auto with DMA-BUF output should use VA-API when it is available")
	}
}

// FFmpeg errors carry one "media:" prefix and say what failed.
func TestErrorContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.mp4")
	_, err := OpenFile(path)
	var e *Error
	if !errors.As(err, &e) || e.Code >= 0 {
		t.Fatalf("OpenFile(missing): %v, want an *Error", err)
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "media: open "+path+": ") || strings.Count(msg, "media:") != 1 {
		t.Errorf("error %q, want one prefix and the operation", msg)
	}
}

// Using a Decoder or File after Close reports ErrClosed instead of passing
// freed C state to FFmpeg. Frames handed out before Close stay valid.
func TestAfterClose(t *testing.T) {
	f, err := OpenFile(filepath.Join("testdata", "h264.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDecoder(f.Codec, Options{Hardware: HardwareOff})
	if err != nil {
		t.Fatal(err)
	}
	var kept []Frame
	emit := func(fr Frame) { kept = append(kept, fr) }
	if err := d.Decode(Packet{Data: f.Config(), Config: true}, emit); err != nil {
		t.Fatal(err)
	}
	p, err := f.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Decode(p, emit); err != nil {
		t.Fatal(err)
	}
	f.Close()
	f.Close()
	if _, err := f.Read(); !errors.Is(err, ErrClosed) {
		t.Errorf("File.Read after Close: %v, want ErrClosed", err)
	}
	d.Close()
	d.Close()
	if err := d.Decode(p, emit); !errors.Is(err, ErrClosed) {
		t.Errorf("Decode after Close: %v, want ErrClosed", err)
	}
	if err := d.Flush(emit); !errors.Is(err, ErrClosed) {
		t.Errorf("Flush after Close: %v, want ErrClosed", err)
	}
	if d.Hardware() {
		t.Error("Hardware after Close")
	}
	d.DisableDMABuf()
	if len(kept) == 0 {
		t.Fatal("no frame decoded before Close")
	}
	for _, fr := range kept {
		_ = fr.Image.Pix[len(fr.Image.Pix)-1]
		fr.Release()
	}
}

// A device rotating or resizing its display sends new parameter sets
// mid-stream: one decoder must follow, on every output path.
func TestResolutionChange(t *testing.T) {
	for _, opt := range []Options{{Hardware: HardwareOff}, {Hardware: HardwareVAAPI}, {Hardware: HardwareVAAPI, DMABuf: true}} {
		d, err := NewDecoder(H264, opt)
		if err != nil {
			t.Logf("%+v: %v", opt, err)
			continue
		}
		var sizes []image.Point
		emit := func(fr Frame) {
			sz := image.Pt(fr.Width, fr.Height)
			if fr.DMABuf != nil {
				sz = image.Pt(fr.DMABuf.Width, fr.DMABuf.Height)
			} else if fr.Image.Rect.Size() != sz {
				t.Fatalf("image %v for a %v frame", fr.Image.Rect, sz)
			}
			if len(sizes) == 0 || sizes[len(sizes)-1] != sz {
				sizes = append(sizes, sz)
			}
			fr.Release()
		}
		for _, name := range []string{"h264.mp4", "odd.mp4", "h264.mp4"} {
			f, err := OpenFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			if c := f.Config(); c != nil {
				if err := d.Decode(Packet{Data: c, Config: true}, emit); err != nil {
					t.Fatal(err)
				}
			}
			for {
				p, err := f.Read()
				if err != nil {
					break
				}
				if err := d.Decode(p, emit); err != nil {
					t.Fatalf("%+v %s: %v", opt, name, err)
				}
			}
			f.Close()
		}
		d.Close()
		want := []image.Point{{640, 360}, {650, 370}, {640, 360}}
		if len(sizes) != 3 || sizes[0] != want[0] || sizes[1] != want[1] || sizes[2] != want[2] {
			t.Errorf("%+v: sizes %v, want %v", opt, sizes, want)
		}
	}
}
