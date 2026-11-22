package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Codec is how Video compresses the frames.
type Codec uint8

const (
	// DefaultCodec is H264, or VP9 for WebM.
	DefaultCodec Codec = iota
	// H264 is H.264 (AVC), which every player plays: in MP4 or QuickTime.
	H264
	// HEVC is H.265, about as good at two thirds of the bitrate, which
	// newer systems play: in MP4 or QuickTime. Windows encodes it with the
	// HEVC Video Extensions.
	HEVC
	// ProRes is Apple ProRes 422 HQ, for editing: in QuickTime, on macOS,
	// or with ffmpeg.
	ProRes
	// ProRes4444 is Apple ProRes 4444, with the alpha of a Transparent
	// composition: in QuickTime, on macOS, or with ffmpeg.
	ProRes4444
	// VP9 is VP9 in WebM, with the alpha of a Transparent composition
	// with ffmpeg: on Linux with GStreamer's vpx plugin, or with ffmpeg.
	VP9
)

func (c Codec) String() string {
	switch c {
	case H264:
		return "H.264"
	case HEVC:
		return "HEVC"
	case ProRes:
		return "ProRes 422 HQ"
	case ProRes4444:
		return "ProRes 4444"
	case VP9:
		return "VP9"
	}
	return "Codec(" + strconv.Itoa(int(c)) + ")"
}

// alpha reports whether the codec keeps alpha.
func (c Codec) alpha() bool { return c == ProRes4444 || c == VP9 }

// Encoder chooses what encodes a Video.
type Encoder uint8

const (
	// System encodes with the system's media framework, in hardware
	// where it can: AVFoundation (VideoToolbox) on macOS, Media
	// Foundation on Windows, GStreamer on Linux.
	System Encoder = iota
	// FFmpeg pipes the frames to ffmpeg (Options.FFmpeg), for what the
	// system has no encoder of, as WebM on macOS and Windows, or a Linux
	// without GStreamer's encoders.
	FFmpeg
)

// A RateLimit caps the video's bytes over a span of time: at most Bytes
// in any Per of the video, as {Bytes: 1_500_000, Per: time.Second} keeps
// each second of it under 12 Mb, where Options.Bitrate is only the
// average. It is VideoToolbox's kVTCompressionPropertyKey_DataRateLimits.
type RateLimit struct {
	Bytes int
	Per   time.Duration
}

// bitsPerSecond is the rate the limit allows, in bits a second.
func (l RateLimit) bitsPerSecond() float64 { return float64(l.Bytes) * 8 / l.Per.Seconds() }

// vbv returns the limits as a single leaky bucket, as x264's VBV and
// Media Foundation's peak constrained VBR take them: its rate in bits a
// second and its size in bits, 0, 0 for no limits. What a span of time
// T takes of a bucket of rate r and size b is at most b + r·T: a bucket
// of a quarter of a limit's bytes, drained at three quarters of its rate,
// keeps every span of its Per to its Bytes. Of several limits, the
// least of each.
func vbv(limits []RateLimit) (maxrate, bufsize int) {
	if len(limits) == 0 {
		return 0, 0
	}
	for i, l := range limits {
		r, b := int(l.bitsPerSecond()*3/4), l.Bytes*8/4
		if i == 0 || r < maxrate {
			maxrate = r
		}
		if i == 0 || b < bufsize {
			bufsize = b
		}
	}
	return max(maxrate, 1), max(bufsize, 8)
}

// Colorimetry is what a video says its frames' colours are, by the code
// points of ITU-T H.273 (ISO/IEC 23091-2): the colour primaries, the
// transfer characteristics and the matrix coefficients, which an MP4 or
// a QuickTime movie carries in its colr box, and H.264 and HEVC in their
// streams. It changes no pixel: it tells players how to show them. A
// field left zero is BT.709's 1.
//
// Video writes BT709 and SRGB, H.264's 1-1-1 and 1-13-1.
type Colorimetry struct {
	Primaries, Transfer, Matrix uint8
}

// The colorimetries Video writes.
var (
	// BT709 is HD video's, 1-1-1: the default.
	BT709 = Colorimetry{Primaries: 1, Transfer: 1, Matrix: 1}
	// SRGB is BT.709's with the transfer of sRGB, IEC 61966-2-1, 1-13-1:
	// that of the pixels a view draws, as browsers show them.
	SRGB = Colorimetry{Primaries: 1, Transfer: 13, Matrix: 1}
)

// The H.273 code points of the colorimetries Video writes.
const (
	h273BT709 = 1
	h273SRGB  = 13
)

// orDefault returns c with its zero fields BT.709's.
func (c Colorimetry) orDefault() Colorimetry {
	if c.Primaries == 0 {
		c.Primaries = h273BT709
	}
	if c.Transfer == 0 {
		c.Transfer = h273BT709
	}
	if c.Matrix == 0 {
		c.Matrix = h273BT709
	}
	return c
}

func (c Colorimetry) String() string {
	return fmt.Sprintf("%d-%d-%d", c.Primaries, c.Transfer, c.Matrix)
}

// srgb reports whether the transfer is sRGB's, of a colorimetry Video
// writes.
func (c Colorimetry) srgb() bool { return c.Transfer == h273SRGB }

// container is the format of a video file.
type container uint8

const (
	mp4 container = iota
	mov
	webm
)

// videoSpec is what a video sink writes.
type videoSpec struct {
	path      string
	container container
	codec     Codec
	// w and h are the size of the frames, num/den their rate.
	w, h     int
	num, den int64
	// bitrate and audioBitrate are bits a second; alpha keeps the frames'
	// alpha, premultiplied.
	bitrate, audioBitrate int
	alpha                 bool
	// limits cap the bitrate over spans of time (Options.RateLimits),
	// color is what the frames' colours are, with no zero field.
	limits []RateLimit
	color  Colorimetry
	// sound is the sound track, interleaved stereo at 48 kHz, nil for
	// none.
	sound  []int16
	ffmpeg string
}

// Video renders the composition into a video file at path: an MP4 (.mp4,
// .m4v), a QuickTime movie (.mov), or a WebM (.webm), with the sound its
// frames play (Frame.Audio) in AAC, or Opus in WebM. The file appears
// when the video is whole: until then the frames go into a file beside
// it, which an error or ctx's end removes.
func Video(ctx context.Context, comp Composition, path string, o Options) error {
	if err := comp.validate(); err != nil {
		return err
	}
	spec, err := newVideoSpec(comp, path, o)
	if err != nil {
		return err
	}
	from, to, err := comp.span(o)
	if err != nil {
		return err
	}
	if !o.Silent {
		if spec.sound, err = soundtrack(ctx, comp, from, to); err != nil {
			return err
		}
	}
	dir, base := filepath.Split(path)
	ext := filepath.Ext(base)
	tmp := filepath.Join(dir, "."+strings.TrimSuffix(base, ext)+".render-"+strconv.Itoa(os.Getpid())+ext)
	os.Remove(tmp)
	spec.path = tmp
	err = run(ctx, comp, o, func(s *session, d drawer) (sink, error) {
		if o.Encoder == FFmpeg {
			return newFFmpegSink(ctx, spec)
		}
		return newVideoSink(spec, d)
	})
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// newVideoSpec returns what Video writes of comp into path, without its
// sound: the format, the codec, and the encoder's settings of o.
func newVideoSpec(comp Composition, path string, o Options) (videoSpec, error) {
	spec := videoSpec{w: comp.Width, h: comp.Height, codec: o.Codec, bitrate: o.Bitrate, audioBitrate: o.AudioBitrate, ffmpeg: o.FFmpeg}
	spec.num, spec.den = comp.rate()
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".mp4", ".m4v":
		spec.container = mp4
	case ".mov":
		spec.container = mov
	case ".webm":
		spec.container = webm
	default:
		return videoSpec{}, fmt.Errorf("render: no video format for %q: name an .mp4, .mov or .webm", ext)
	}
	if spec.codec == DefaultCodec {
		spec.codec = H264
		if spec.container == webm {
			spec.codec = VP9
		}
	}
	switch {
	case spec.container == webm && spec.codec != VP9,
		spec.container != webm && spec.codec == VP9:
		return videoSpec{}, fmt.Errorf("render: %v goes into WebM alone", VP9)
	case spec.container == mp4 && (spec.codec == ProRes || spec.codec == ProRes4444):
		return videoSpec{}, fmt.Errorf("render: %v goes into QuickTime (.mov)", spec.codec)
	case spec.codec > VP9:
		return videoSpec{}, fmt.Errorf("render: unknown codec %v", spec.codec)
	}
	if spec.codec != ProRes && spec.codec != ProRes4444 && (spec.w%2 != 0 || spec.h%2 != 0) {
		// YUV 4:2:0 halves the chroma's size.
		return videoSpec{}, fmt.Errorf("render: %v takes an even Width and Height, not %d×%d", spec.codec, spec.w, spec.h)
	}
	spec.alpha = comp.Transparent && spec.codec.alpha()
	if spec.bitrate <= 0 {
		bpp := 0.2 // bits a pixel a frame: H.264 of motion graphics
		if spec.codec == HEVC || spec.codec == VP9 {
			bpp = 0.12
		}
		spec.bitrate = max(int(float64(spec.w*spec.h)*float64(spec.num)/float64(spec.den)*bpp), 500_000)
	}
	if spec.audioBitrate <= 0 {
		spec.audioBitrate = 192_000
	}
	for _, l := range o.RateLimits {
		if l.Bytes <= 0 || l.Per <= 0 {
			return videoSpec{}, fmt.Errorf("render: a RateLimit of %d bytes in %v: both must be more than zero", l.Bytes, l.Per)
		}
	}
	spec.limits = o.RateLimits
	spec.color = o.Colorimetry.orDefault()
	if spec.color.Primaries != h273BT709 || spec.color.Matrix != h273BT709 || (spec.color.Transfer != h273BT709 && spec.color.Transfer != h273SRGB) {
		return videoSpec{}, fmt.Errorf("render: no video of colorimetry %v: Video writes %v (BT709) and %v (SRGB)", spec.color, BT709, SRGB)
	}
	return spec, nil
}
