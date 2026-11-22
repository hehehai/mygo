package render

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestVideoSpecOfOptions: RateLimits and Colorimetry reach what the sinks
// write, BT.709 and no limits by default.
func TestVideoSpecOfOptions(t *testing.T) {
	comp := boxes(320, 180, 10)
	spec, err := newVideoSpec(comp, "a.mp4", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if spec.color != BT709 || spec.limits != nil {
		t.Errorf("by default: colorimetry %v, limits %v", spec.color, spec.limits)
	}
	limits := []RateLimit{{Bytes: 1_250_000, Per: time.Second}, {Bytes: 4_000_000, Per: 4 * time.Second}}
	spec, err = newVideoSpec(comp, "a.mp4", Options{RateLimits: limits, Colorimetry: SRGB})
	if err != nil {
		t.Fatal(err)
	}
	if spec.color != SRGB || !slices.Equal(spec.limits, limits) {
		t.Errorf("colorimetry %v, limits %v", spec.color, spec.limits)
	}
	// A field left zero is BT.709's.
	if spec, err := newVideoSpec(comp, "a.mov", Options{Colorimetry: Colorimetry{Transfer: 13}}); err != nil || spec.color != SRGB {
		t.Errorf("transfer 13 alone: %v, %v", spec.color, err)
	}
	for _, o := range []Options{
		{RateLimits: []RateLimit{{Bytes: 0, Per: time.Second}}},
		{RateLimits: []RateLimit{{Bytes: 1000, Per: 0}}},
		{Colorimetry: Colorimetry{Primaries: 9, Transfer: 16, Matrix: 9}},
		{Colorimetry: Colorimetry{Transfer: 8}},
	} {
		if _, err := newVideoSpec(comp, "a.mp4", o); err == nil {
			t.Errorf("%+v: no error", o)
		}
		if err := Video(context.Background(), comp, filepath.Join(t.TempDir(), "a.mp4"), o); err == nil {
			t.Errorf("Video of %+v: no error", o)
		}
	}

	// The leaky bucket of the encoders without VideoToolbox's limits: the
	// tightest rate, the least bytes.
	if r, b := vbv(limits); r != 6_000_000 || b != 2_500_000 {
		t.Errorf("vbv %d b/s, %d b", r, b)
	}
	if r, b := vbv(nil); r != 0 || b != 0 {
		t.Errorf("vbv of none: %d, %d", r, b)
	}
}

// TestFFmpegArgsOfOptions: ffmpeg is given the bucket and the transfer.
func TestFFmpegArgsOfOptions(t *testing.T) {
	comp := boxes(320, 180, 10)
	args := func(path string, o Options) string {
		spec, err := newVideoSpec(comp, path, o)
		if err != nil {
			t.Fatal(err)
		}
		s := &ffmpegSink{spec: spec}
		return strings.Join(append(s.videoArgs(), s.colorArgs()...), " ")
	}
	def := args("a.mp4", Options{})
	if strings.Contains(def, "-maxrate") || !strings.Contains(def, "-color_primaries bt709 -color_trc bt709 -colorspace bt709") {
		t.Errorf("by default: %s", def)
	}
	o := Options{RateLimits: []RateLimit{{Bytes: 1_250_000, Per: time.Second}}, Colorimetry: SRGB}
	for _, path := range []string{"a.mp4", "a.webm"} {
		got := args(path, o)
		if !strings.Contains(got, "-maxrate 7500000 -bufsize 2500000") || !strings.Contains(got, "-color_trc iec61966-2-1") {
			t.Errorf("%s: %s", path, got)
		}
	}
	if got := args("a.mov", Options{Codec: ProRes, RateLimits: o.RateLimits}); strings.Contains(got, "-maxrate") {
		t.Errorf("ProRes with limits: %s", got)
	}
}

// noise is a composition of still gray in its even seconds, and of cells
// of new colours each frame in its odd ones, which no encoder compresses
// well: peaks of the bitrate after seconds of almost none.
func noise(w, h, frames int) Composition {
	return Composition{
		Width: w, Height: h, FPS: 30, Frames: frames, Background: ui.RGB(128, 128, 128),
		View: func(c *ui.Context, f Frame) {
			if f.Index/30%2 == 0 {
				return
			}
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				const cell = 20
				x := uint32(f.Index) * 2654435761
				for y := float32(0); y < r.H; y += cell {
					for xx := float32(0); xx < r.W; xx += cell {
						x ^= x << 13
						x ^= x >> 17
						x ^= x << 5
						p.Fill(ui.Rect{X: r.X + xx, Y: r.Y + y, W: cell, H: cell}, ui.RGB(uint8(x), uint8(x>>8), uint8(x>>16)), 0)
					}
				}
			})
		},
	}
}

// mp4Video returns the sizes of the video track's samples, in the order
// of the file, and the colr box of its sample entry: its kind, nclx or
// nclc, and its primaries, transfer and matrix.
func mp4Video(t *testing.T, path string) (sizes []uint32, kind string, colr [3]uint16) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(b []byte, video *bool)
	walk = func(b []byte, video *bool) {
		for len(b) >= 8 {
			size := int(binary.BigEndian.Uint32(b))
			box := string(b[4:8])
			head := 8
			if size == 1 && len(b) >= 16 {
				size, head = int(binary.BigEndian.Uint64(b[8:])), 16
			} else if size == 0 {
				size = len(b)
			}
			if size < head || size > len(b) {
				return
			}
			body := b[head:size]
			switch {
			case box == "moov", box == "mdia", box == "minf", box == "stbl":
				walk(body, video)
			case box == "trak":
				var v bool
				walk(body, &v)
			case video == nil:
			case box == "hdlr" && len(body) >= 12:
				// The media's comes first; QuickTime's minf has another.
				if s := string(body[8:12]); s == "vide" || s == "soun" {
					*video = *video || s == "vide"
				}
			case box == "stsd" && *video && len(body) >= 8+8+78:
				// The first sample entry: a visual one, whose boxes follow
				// 78 bytes of its own.
				entry := body[8:]
				n := int(binary.BigEndian.Uint32(entry))
				if n < 8+78 || n > len(entry) {
					return
				}
				for c := entry[8+78 : n]; len(c) >= 8; {
					cn := int(binary.BigEndian.Uint32(c))
					if cn < 8 || cn > len(c) {
						break
					}
					if string(c[4:8]) == "colr" && cn >= 18 {
						kind = string(c[8:12])
						colr = [3]uint16{binary.BigEndian.Uint16(c[12:]), binary.BigEndian.Uint16(c[14:]), binary.BigEndian.Uint16(c[16:])}
					}
					c = c[cn:]
				}
			case box == "stsz" && *video && len(body) >= 12:
				if fixed := binary.BigEndian.Uint32(body[4:]); fixed != 0 {
					for range binary.BigEndian.Uint32(body[8:]) {
						sizes = append(sizes, fixed)
					}
					break
				}
				for s := body[12:]; len(s) >= 4; s = s[4:] {
					sizes = append(sizes, binary.BigEndian.Uint32(s))
				}
			}
			b = b[size:]
		}
	}
	walk(data, nil)
	return sizes, kind, colr
}

// peak returns the most bytes of n samples in a row.
func peak(sizes []uint32, n int) int {
	most, sum := 0, 0
	for i, s := range sizes {
		sum += int(s)
		if i >= n {
			sum -= int(sizes[i-n])
		}
		most = max(most, sum)
	}
	return most
}

// TestRateLimitsAndColorimetry encodes a video whose bitrate peaks, with
// the system's encoder and ffmpeg's: with limits, no second of it has
// more bytes than the limit of a second, and its colr box says sRGB's
// transfer when asked, BT.709's 1-1-1 by default.
//
// VideoToolbox takes a limit as a target it overshoots a little: on
// these frames, an M4 Mac's encoders put up to 14% more in a
// second than a lone limit of a second allows. A limit of a quarter
// second beside it keeps the seconds under theirs.
func TestRateLimitsAndColorimetry(t *testing.T) {
	dir := t.TempDir()
	const frames = 90
	encoders := []Encoder{System}
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		encoders = append(encoders, FFmpeg)
	}
	ran := 0
	for _, enc := range encoders {
		for _, codec := range []Codec{H264, HEVC} {
			for _, name := range []string{"v.mp4", "v.mov"} {
				if enc == FFmpeg && (codec == H264) != (name == "v.mp4") {
					// x264 and x265 do what they do in either container:
					// H.264 in MP4 and HEVC in QuickTime, in half the time.
					continue
				}
				label := map[Encoder]string{System: "system", FFmpeg: "ffmpeg"}[enc] + " " + codec.String() + " " + filepath.Ext(name)
				// 1280×720 at 6 Mb/s, and a limit of 8 Mb a second, for
				// VideoToolbox; a quarter of the frames and the limit, at
				// half the bitrate, for the encoders of the CPU, which take
				// minutes of frames this large on a busy machine.
				w, h, bitrate := 1280, 720, 6_000_000
				second := RateLimit{Bytes: 1_000_000, Per: time.Second}
				if enc != System || runtime.GOOS != "darwin" {
					w, h, bitrate, second.Bytes = w/2, h/2, bitrate/2, second.Bytes/4
				}
				limits := []RateLimit{second, {Bytes: second.Bytes / 4, Per: second.Per / 4}}
				encode := func(o Options) ([]uint32, string, [3]uint16, error) {
					o.Encoder, o.Codec, o.Bitrate, o.Silent = enc, codec, bitrate, true
					path := filepath.Join(dir, strings.ReplaceAll(label, " ", "-")+name)
					if err := Video(context.Background(), noise(w, h, frames), path, o); err != nil {
						return nil, "", [3]uint16{}, err
					}
					sizes, kind, colr := mp4Video(t, path)
					if len(sizes) != frames {
						t.Errorf("%s: %d frames, want %d", label, len(sizes), frames)
					}
					if probe, err := exec.LookPath("ffprobe"); err == nil && o.Colorimetry == SRGB {
						// What the stream says, as players read it.
						out, err := exec.Command(probe, "-v", "error", "-select_streams", "v:0", "-show_entries",
							"stream=color_primaries,color_transfer,color_space", "-of", "csv=p=0", path).Output()
						if got := strings.TrimSpace(string(out)); err != nil || got != "bt709,iec61966-2-1,bt709" {
							t.Errorf("%s: ffprobe says %q (%v)", label, got, err)
						}
					}
					return sizes, kind, colr, nil
				}
				sizes, kind, colr, err := encode(Options{})
				if errors.Is(err, errors.ErrUnsupported) {
					t.Logf("%s: %v", label, err)
					continue
				}
				if err != nil {
					t.Errorf("%s: %v", label, err)
					continue
				}
				ran++
				if colr != [3]uint16{1, 1, 1} {
					t.Errorf("%s: by default colr %s %v, want 1-1-1", label, kind, colr)
				}
				unlimited := peak(sizes, 30)

				sizes, _, _, err = encode(Options{RateLimits: []RateLimit{second}})
				if err != nil {
					t.Errorf("%s with a limit: %v", label, err)
					continue
				}
				alone := peak(sizes, 30)

				sizes, kind, colr, err = encode(Options{RateLimits: limits, Colorimetry: SRGB})
				if err != nil {
					t.Errorf("%s with limits: %v", label, err)
					continue
				}
				limited := peak(sizes, 30)
				t.Logf("%s: the most bytes of a second %d without limits, %d with one of %d a second (%.2f of it), %d with one of a quarter second beside it (%.2f)",
					label, unlimited, alone, second.Bytes, float64(alone)/float64(second.Bytes), limited, float64(limited)/float64(second.Bytes))
				if unlimited <= second.Bytes {
					t.Errorf("%s: %d bytes in a second without limits: the frames test no limit of %d", label, unlimited, second.Bytes)
				}
				if alone >= unlimited {
					t.Errorf("%s: a limit of a second changes nothing: %d bytes, %d without", label, alone, unlimited)
				}
				if limited > second.Bytes {
					t.Errorf("%s: %d bytes in a second, over the limit of %d", label, limited, second.Bytes)
				}
				if colr != [3]uint16{1, 13, 1} {
					t.Errorf("%s: colr %s %v, want 1-13-1", label, kind, colr)
				}
			}
		}
	}
	if ran == 0 {
		t.Skip("no encoder of H.264 or HEVC here")
	}
}
