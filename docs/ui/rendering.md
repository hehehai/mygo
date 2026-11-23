# Rendering

MyGo draws on the GPU with Metal on macOS, with Direct3D 11 on Windows, or
with WARP, Windows' own software renderer, where no GPU driver works, and
with OpenGL on Linux, in the GtkGLArea GTK shows, and with WebGL 2 in
[browsers](browsers.md), with the shader of Linux. A shader computes rounded
rectangles, borders, gradients and shadows from the distance to their
edges, so they stay sharp at any size and scale, and text comes from a
glyph atlas that only uploads what changes.

With a GPU, every frame draws on it. On Linux, the first window of
native UI loads OpenGL: Mesa, the driver, takes some 50 MB, which stay,
about as much as the rest of a small app. Where OpenGL would not run on a
GPU, as in virtual machines or in WSL (where `GALLIUM_DRIVER=d3d12` gives
Mesa the GPU), Linux draws on the CPU: about a millisecond for a whole
large window on a high-density display, on several cores, and less than a
tenth of one for what typically changes, such as a button under the
pointer, since it redraws, and has the compositor take, only that. Set
`MYGO_GPU=0` to use the CPU renderer everywhere, for instance to compare,
and on Linux `MYGO_GPU=1` to draw with OpenGL even where it runs on the
CPU.

Set `MYGO_FRAME_STATS=1` to log each frame that takes longer than 8 ms, or
`MYGO_FRAME_STATS=4` for another threshold in milliseconds (`all` logs
every frame). Each line tells how long building the frame (and in how many
passes), laying it out, painting and presenting it took, whether it was
drawn on the GPU or in memory (where a window has no GPU renderer), what
the process allocated and how many texts it laid out
meanwhile, and whether the garbage collector ran, with the time goroutines
spent helping it mark:

```
mygo: frame 412 took 17.3 ms: build 3.1 (2 passes), layout 1.2, paint 0.8, present 11.5 (drawn on the GPU), other 0.7; meanwhile the process made about 2312 allocations, 171.0 KB, 12 text layouts; GC: 1 cycles ended, 4.1 ms of assists
```

Allocations and the collector are the whole process's, other goroutines'
included, as `runtime/metrics` counts them: a span at a time, so that
small counts read low. While the variable is unset, frames measure
nothing. Frames that follow each other wait for the display's next
refresh, and present may wait for it too; to see how fast frames could
come, set `MYGO_UNCAPPED=1` as well ([Frame pacing](#frame-pacing)).

MyGo draws a frame only when something changes: input, `Invalidate`,
`After`, or an animation that moves. An idle window draws nothing, and two
seconds after its last frame it frees the frame it drew in memory.

A window animating on macOS keeps three frames for the display, one
more than the two its frames take turns in: with two, a 120 Hz display
holds both often enough that frames wait for one, and some go undrawn.
The third costs about 4 bytes a pixel of the window (8 with colors
outside sRGB) until the window idles.

## Frame pacing

Frames that follow each other, as an animation's, draw at the display's
refreshes, so every scene that takes less than a refresh shows the
display's rate: 120 frames a second on a 120 Hz display, light or heavy.
Set `MYGO_UNCAPPED=1` to present frames without waiting for the display,
to measure how much of a refresh a scene leaves, to compare two builds at
any load, or for the least latency, as ink, remote desktops or games
want. The next frame then starts as soon as the last was presented,
while something animates or asks for frames; a window where nothing
changes still draws nothing. With `MYGO_FRAME_STATS` set too, MyGo logs
how many frames came each second, and each frame logged says uncapped:

```
mygo: 2481 frames a second, uncapped: 0.40 ms a frame
```

The default stays paced: uncapped frames cost the CPU and the GPU all
they can take, and may show torn, which apps don't want. What each
platform does:

| Platform | Uncapped |
|---|---|
| Windows | The swap chain allows tearing where the system can (`DXGI_FEATURE_PRESENT_ALLOW_TEARING`) and every frame presents with sync interval 0, waiting neither for the swap chain's frame latency object nor for DWM's next composition. Frames show torn only where the window takes the display, as a borderless window covering the screen; elsewhere DWM still shows the newest frame at each refresh. Without tearing, frames still present at once, a new one taking the place of one DWM has not shown yet. A window over a material (DirectComposition) has no tearing. While the window changes size, until a frame half a second after the last change, frames present paced, as without `MYGO_UNCAPPED`, and log `paced while resizing`: uncapped frames queued for DWM as it resized the window, stalling a present for seconds. A frame of a new size waits for DWM to show it at most two refreshes, and every frame then presents with `DXGI_PRESENT_DO_NOT_WAIT`: one that Present cannot take within a refresh presents after the next composition, or gives way to the next frame, while the window keeps taking its messages, so that no frame waits on DWM for long. One still not taken after 100 ms presents without the flag, waiting for DXGI on another thread while the window keeps taking its messages, and the next frame draws once it did. After that, frames stay paced, with `DXGI_PRESENT_DO_NOT_WAIT`, until DXGI takes one at once with the resize settled, and the buffers resize, to settle or to grow, on another thread too: DXGI took no frame with the flag again until a call waited for it, and such a call on the main thread held one frame for about a second. A frame that comes while such a call waits is put off, the window showing the last frame. A frame Present refused logs `Present still drawing N times`, and `presenting once DXGI can` when it waits; a frame put off logs `put off while Present waits on another thread` (or `ResizeBuffers`), the frame after it `after Present waited on another thread`, and one paced after the resize settled `paced until DXGI takes a frame at once`. |
| macOS | The display link no longer paces frames: each starts in the run loop's next pass, after the input that came, into a `CAMetalLayer` whose `displaySyncEnabled` is off. The window server still composites a window once a refresh, showing its newest frame, without tearing: frames above the display's rate are drawn and measured, not all shown. |
| Linux | GTK's frame clock starts the next frame as soon as the last was painted, and on X11 the GLX swap interval of the window is 0 (GTK's EGL on Wayland swaps with 0 already). A compositor that paces windows still does: on X11 one sending `_NET_WM_FRAME_TIMINGS`, and on Wayland the frame callbacks of frames drawn in memory. |
| Browsers | Ignored: `requestAnimationFrame` paces frames, and nothing presents sooner. |

A game's steps keep their rate uncapped: [`game.View`](../plugins/game.md)
runs `Update` 60 times a second of time passed, however many frames
come, and `Draw` with how far each frame is into the next step.

### Low latency

Paced frames on macOS queue for the display: a drawable presented waits
behind those presented before it, so while frames keep coming, as a
video plays or an animation runs, a change shows a refresh or two after
its frame presented. `ui.SetLowLatency(true)` turns the windows'
`CAMetalLayer.displaySyncEnabled` off from their next frame, frames still
paced by the display link: each drawable goes to the window server as
soon as the GPU drew it, and the window server composites the newest at
each refresh. A change then shows sooner, but frames come less evenly,
reaching the window server at any time within a refresh.
`ui.SetLowLatency(false)` turns display sync back on, as by default.

It is meant to be switched while the app runs, not set once: a video
editor sets it while paused and editing, and clears it to play. In a
video editor on a 60 Hz MacBook Air, display sync off while paused took
an edit from its patch to the display in 48 ms rather than 79 (p95) as
an agent made it, and in 40 ms rather than 50 as the user did; off
during playback, an edit showed in 64 ms rather than 96, but playback
fell to 56 frames a second, about 100 of 10 s of frames coming more than
25 ms apart.

The setting is process-wide and takes effect at the next frame each
window draws; a window that draws nothing keeps its layer as it was until
it does. It has no effect while `MYGO_UNCAPPED=1` or
`MYGO_METAL_DISPLAY_SYNC=0` keep display sync off, and none on Windows,
Linux, iOS or in browsers, where it is a no-op.

### Metal latency diagnostics

These variables, read once as the program starts, change how macOS
windows present, to measure where a frame's latency goes (`go run
./internal/inputlatency`). Unset, nothing changes; they are diagnostics,
not settings for apps to ship with.

| Variable | Effect |
|---|---|
| `MYGO_METAL_DRAWABLES=2` | The layer's `maximumDrawableCount` (2 or 3, default 3): one frame less queued, at the cost of frames waiting for a drawable on a 120 Hz display ([above](#rendering)). |
| `MYGO_METAL_NO_PREFETCH=1` | No drawable fetched on another thread after a frame presents: each frame takes its own when it draws. |
| `MYGO_METAL_DISPLAY_SYNC=0` | `displaySyncEnabled` off for every frame, as `ui.SetLowLatency(true)` for the whole run, frames still paced by the display link. |
