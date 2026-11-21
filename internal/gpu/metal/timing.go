//go:build darwin

package metal

import (
	"fmt"
	"os"
	"time"
)

// Frame timing (MYGO_METAL_TIMING): MyGo's frame statistics
// (MYGO_FRAME_STATS) give a frame's present as one number; with
// MYGO_METAL_TIMING set, the renderer writes a line a frame to stderr:
//
//	metal-timing: frame N drawable D encode E wait W upload K B KB U sync S
//
// D the milliseconds the frame waited for its drawable (nextDrawable, or
// the drawable fetched ahead), E encoding it, W of that waiting for a frame
// in flight to complete, K images uploaded (B kilobytes) in U milliseconds,
// S whether it presented with the Core Animation transaction. A frame
// built while its drawable is being fetched writes "drawable pending", and
// once drawn a "metal-pending: frame N drawable D" line (another prefix,
// so that scripts reading metal-timing's fields by place are not misled).
// Each
// frame's command buffer, once its slot comes round again, writes
//
//	metal-gpu: frame N gpu MS end S
//
// the GPU's time on it (GPUEndTime less GPUStartTime) and when it ended
// (seconds, the host's clock), whose gaps tell the frames the GPU finished
// late. Unset, nothing is written (a few clock reads a frame).
var metalTiming = os.Getenv("MYGO_METAL_TIMING") != ""

// timingState is what a frame measures for MYGO_METAL_TIMING.
type timingState struct {
	drawable, wait, upload time.Duration
	uploadBytes            int64
	// slotFrame is the frame each slot's command buffer drew.
	slotFrame [ringSize]uint64
}

// imageBytes is the bytes of a w×h texture of RGBA8.
func imageBytes(w, h int) int64 { return int64(w) * int64(h) * 4 }

// upload makes the texture of an image's pixels, counting it.
func (r *Renderer) upload(w, h int, pix []byte) (tex id) {
	r.timeUpload(len(pix), func() { tex = r.newTexture(w, h, pixelFormatRGBA8Unorm, usageShaderRead, pix, w*4) })
	return tex
}

// timeUpload runs fn, an upload of n bytes, counting it.
func (r *Renderer) timeUpload(n int, fn func()) {
	r.img.uploads++
	if !metalTiming {
		fn()
		return
	}
	t0 := time.Now()
	fn()
	r.tm.upload += time.Since(t0)
	r.tm.uploadBytes += int64(n)
}

// beginTiming starts a frame's timing.
func (r *Renderer) beginTiming() {
	r.tm.drawable, r.tm.wait, r.tm.upload, r.tm.uploadBytes = 0, 0, 0, 0
}

// timeDrawable runs fn, which waits for a drawable, timing it.
func (r *Renderer) timeDrawable(fn func() id) id {
	if !metalTiming {
		return fn()
	}
	t0 := time.Now()
	d := fn()
	r.tm.drawable += time.Since(t0)
	return d
}

// logFrame writes the timing of the frame encoded since start.
func (r *Renderer) logFrame(start time.Time, pending, sync bool) {
	if !metalTiming {
		return
	}
	drawable := fmt.Sprintf("%.2f", ms(r.tm.drawable))
	if pending {
		drawable = "pending"
	}
	fmt.Fprintf(os.Stderr, "metal-timing: frame %d drawable %s encode %.2f wait %.2f upload %d %dKB %.2f sync %v\n", r.frame,
		drawable, ms(time.Since(start)-r.tm.drawable), ms(r.tm.wait), r.img.uploads, r.tm.uploadBytes>>10, ms(r.tm.upload), sync)
}

// logPending writes the wait for the drawable of a frame drawn pending.
func (r *Renderer) logPending(drawable time.Duration) {
	if metalTiming {
		fmt.Fprintf(os.Stderr, "metal-pending: frame %d drawable %.2f\n", r.frame, ms(drawable))
	}
}

// waitTimed waits for cb, a frame in flight in slot, to complete, timing
// the wait, and writes the GPU's time on it.
func (r *Renderer) waitTimed(cb id, slot int) {
	if !metalTiming {
		send(cb, "waitUntilCompleted")
		return
	}
	t0 := time.Now()
	send(cb, "waitUntilCompleted")
	r.tm.wait += time.Since(t0)
	start, end := msgFloat(cb, sel("GPUStartTime")), msgFloat(cb, sel("GPUEndTime"))
	fmt.Fprintf(os.Stderr, "metal-gpu: frame %d gpu %.2f end %.4f\n", r.tm.slotFrame[slot], (end-start)*1000, end)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
