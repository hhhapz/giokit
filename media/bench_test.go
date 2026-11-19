package media

import (
	"errors"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hhhapz/giokit/gio/layout"
	"github.com/hhhapz/giokit/gio/op/clip"
	"github.com/hhhapz/giokit/gio/op/paint"

	"github.com/hhhapz/giokit/kittest"
)

// Decode cost per frame at the tablet's resolution (2960x1848), 60 fps H.264
// as an Android encoder sends it. Needs the ffmpeg CLI to make the clip.
//
//	go test -run x -bench . ./media

var (
	clipOnce sync.Once
	clipPath string
	clipErr  error
)

func tabletClip(b *testing.B) string {
	clipOnce.Do(func() {
		// Cached across runs: making the clip takes longer than the bench.
		dir := filepath.Join(os.TempDir(), "giokit-media-bench")
		clipPath = filepath.Join(dir, "tablet.mp4")
		if _, err := os.Stat(clipPath); err == nil {
			return
		}
		if clipErr = os.MkdirAll(dir, 0o755); clipErr != nil {
			return
		}
		clipErr = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
			"-f", "lavfi", "-i", "testsrc2=size=2960x1848:rate=60", "-t", "2",
			"-c:v", "libx264", "-preset", "veryfast", "-bf", "0", "-g", "60", "-b:v", "8M",
			"-pix_fmt", "yuv420p", clipPath).Run()
	})
	if clipErr != nil {
		b.Skip(clipErr)
	}
	return clipPath
}

func benchDecode(b *testing.B, opt Options) {
	path := tabletClip(b)
	f, err := OpenFile(path)
	if err != nil {
		b.Fatal(err)
	}
	var pkts []Packet
	for {
		p, err := f.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			b.Fatal(err)
		}
		pkts = append(pkts, p)
	}
	f.Close()
	d, err := NewDecoder(H264, opt)
	if err != nil {
		b.Skip(err)
	}
	defer d.Close()
	n, dma := 0, 0
	emit := func(fr Frame) {
		n++
		if fr.DMABuf != nil {
			dma++
		}
		fr.Release()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Restart at a key frame each pass; all packets are decoded.
		for _, p := range pkts {
			if err := d.Decode(p, emit); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.StopTimer()
	if opt.DMABuf && dma == 0 {
		b.Skipf("no DMA-BUF frames: %v", d.DMABufError())
	}
	b.ReportMetric(float64(b.Elapsed().Microseconds())/float64(n)/1000, "ms/frame")
}

func BenchmarkTabletSoftware(b *testing.B)  { benchDecode(b, Options{Hardware: HardwareOff}) }
func BenchmarkTabletVAAPIRGBA(b *testing.B) { benchDecode(b, Options{Hardware: HardwareVAAPI}) }
func BenchmarkTabletVAAPIDMA(b *testing.B) {
	benchDecode(b, Options{Hardware: HardwareVAAPI, DMABuf: true})
}

// Decode, submit and render each frame at 2960x1848: the whole per-frame
// cost of showing the tablet, for each path.
func benchRender(b *testing.B, opt Options) {
	path := tabletClip(b)
	f, err := OpenFile(path)
	if err != nil {
		b.Fatal(err)
	}
	var pkts []Packet
	for {
		p, err := f.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			b.Fatal(err)
		}
		pkts = append(pkts, p)
	}
	f.Close()
	d, err := NewDecoder(H264, opt)
	if err != nil {
		b.Skip(err)
	}
	defer d.Close()
	s := paint.NewStream()
	h, err := kittest.New(2960, 1848, 1, func(gtx layout.Context) {
		defer clip.Rect{Max: s.Size()}.Push(gtx.Ops).Pop()
		s.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	})
	if err != nil {
		b.Skip(err)
	}
	defer h.Release()
	n, dma := 0, 0
	emit := func(fr Frame) {
		n++
		if fr.DMABuf != nil {
			dma++
		}
		s.Submit(fr.StreamFrame())
		h.Settle()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range pkts {
			if err := d.Decode(p, emit); err != nil {
				b.Fatal(err)
			}
		}
	}
	_ = h.Pixel(image.Pt(1, 1)) // wait for the GPU
	b.StopTimer()
	if opt.DMABuf && (dma == 0 || s.ImportState() != paint.ImportSupported) {
		b.Skipf("no zero-copy path: %v", d.DMABufError())
	}
	b.ReportMetric(float64(b.Elapsed().Microseconds())/float64(n)/1000, "ms/frame")
}

func BenchmarkTabletRenderSoftware(b *testing.B) { benchRender(b, Options{Hardware: HardwareOff}) }
func BenchmarkTabletRenderZeroCopy(b *testing.B) {
	benchRender(b, Options{Hardware: HardwareVAAPI, DMABuf: true})
}
