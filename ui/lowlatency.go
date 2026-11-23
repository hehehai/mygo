package ui

import "github.com/egoist/mygo/internal/pacing"

// SetLowLatency has the windows' frames shown as soon as they are drawn
// (on), or queued for the display's refreshes as by default (off). On,
// a change shows a refresh or two sooner while frames keep coming (an
// animation, a video playing: on macOS its drawable no longer waits
// behind those presented before it, CAMetalLayer's displaySyncEnabled
// being off, frames still paced by the display link), but frames come
// less evenly, as they reach the window server at any time within a
// refresh: a video editor sets it while paused and editing, and clears
// it to play. It is process-wide and changes each window's next frame; a
// window drawing nothing keeps its layer until it draws. macOS only: on
// Windows, Linux, iOS and in browsers it is a no-op, as it is while
// MYGO_UNCAPPED=1 or MYGO_METAL_DISPLAY_SYNC=0 keep display sync off.
// See "Low latency" in docs/ui/rendering.md.
func SetLowLatency(on bool) { pacing.LowLatency.Store(on) }
