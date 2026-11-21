package gputest

import (
	"math"

	"github.com/egoist/mygo/internal/scene"
)

// TransformedScene exercises all primitives and effects under affine
// composition, with nested transformed clips and an inline glyph transform.
func TransformedScene() *scene.Scene {
	s := Scene()
	s.Clear = scene.Color{R: 100, G: 120, B: 160, A: 80}
	m := scene.Translation(25, 20).Mul(scene.Rotation(4)).Mul(scene.Scaling(0.8, 0.9))
	for i := range s.Ops {
		s.Ops[i].Transform = m
	}
	if len(s.Glyphs) > 0 {
		s.Glyphs[0].Transform = scene.Rotation(15)
	}
	return s
}

// PerspectiveScene exercises all primitives and effects seen in
// perspective: the scene turned 20 degrees about its vertical center line,
// seen from 900 pixels, as RotationY and Perspective make it, with a
// skewed glyph and a nested clip turned the other way.
func PerspectiveScene() *scene.Scene {
	s := Scene()
	s.Clear = scene.Color{R: 100, G: 120, B: 160, A: 80}
	turn := func(degrees, d, cx, cy float32) scene.Affine {
		sin, cos := math.Sincos(float64(degrees) * math.Pi / 180)
		m := scene.Projective(float32(cos), 0, 0, 1, 0, 0, float32(sin)/d, 0)
		return scene.Translation(cx, cy).Mul(m).Mul(scene.Translation(-cx, -cy))
	}
	m := turn(20, 900, float32(s.Width)/2, float32(s.Height)/2)
	for i := range s.Ops {
		s.Ops[i].Transform = m
	}
	if len(s.Glyphs) > 0 {
		s.Glyphs[0].Transform = scene.Matrix(1, 0, 0.3, 1, 0, 0)
	}
	return s
}

// BehindOriginScene is a card far from its drawing's origin, turned over
// in perspective around its center, which the frame shows at (160, 370):
// the origin is behind the viewer, past the turned plane's horizon
// (scene.Affine.Behind), the card in front (#1406). A rounded clip, its
// shadow, a border and a gradient turn with it; red shows at the frame's
// center.
func BehindOriginScene() *scene.Scene {
	s := &scene.Scene{Width: 320, Height: 740, Clear: scene.Color{R: 246, G: 247, B: 249, A: 255}}
	sin, cos := math.Sincos(-60 * math.Pi / 180)
	turn := scene.Projective(1, 0, 0, float32(cos), 0, 0, 0, float32(-sin)/600)
	m := scene.Translation(160, 370).Mul(turn).Mul(scene.Translation(-1050, -1050))
	r4 := func(v float32) [4]float32 { return [4]float32{v, v, v, v} }
	red, blue := scene.Color{R: 220, G: 40, B: 40, A: 255}, scene.Color{R: 40, G: 60, B: 220, A: 255}
	s.Ops = []scene.Op{
		{Kind: scene.OpShadow, Rect: scene.Rect{X: 1000, Y: 1004, W: 100, H: 100}, Radii: r4(12), Blur: 6, Color: scene.Color{A: 120}, Transform: m},
		{Kind: scene.OpPushClip, Rect: scene.Rect{X: 1000, Y: 1000, W: 100, H: 100}, Radii: r4(12), Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 990, Y: 990, W: 120, H: 120}, Color: blue, Color2: red, Paint: scene.PaintLinear,
			Gradient: [4]float32{1000, 1000, 1100, 1000}, Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 1030, Y: 1030, W: 40, H: 40}, Radii: r4(8), Color: red,
			Border: scene.Uniform(3), BorderColor: scene.Color{R: 250, G: 250, B: 250, A: 255}, Transform: m},
		{Kind: scene.OpPopClip, Transform: m},
	}
	return s
}

// AxisClipScene is content under a hard clip that a pose scales and moves
// (a scroll view in a panel that is entering): the clip edges fall in the middle
// of pixels (the CPU renderer samples a pixel on a 4x4 grid, so its
// coverage is exact there).
func AxisClipScene() *scene.Scene {
	s := &scene.Scene{Width: 200, Height: 160, Clear: scene.Color{R: 246, G: 247, B: 249, A: 255}}
	m := scene.Translation(7.5, 5.5).Mul(scene.Scaling(.5, .5))
	red, blue := scene.Color{R: 220, G: 40, B: 40, A: 255}, scene.Color{R: 40, G: 60, B: 220, A: 255}
	s.Ops = []scene.Op{
		{Kind: scene.OpFill, Rect: scene.Rect{X: 5, Y: 5, W: 120, H: 100}, Radii: [4]float32{14, 14, 14, 14}, Color: blue, Transform: m},
		{Kind: scene.OpPushClip, Rect: scene.Rect{X: 10, Y: 12, W: 80, H: 60}, Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{W: 200, H: 160}, Color: red, Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 40, Y: 30, W: 90, H: 70}, Radii: [4]float32{20, 20, 20, 20}, Color: blue, Transform: m},
		{Kind: scene.OpPopClip, Transform: m},
	}
	return s
}

// TranslatedScenes are the reference scene moved by translations, by
// whole pixels and between them (not by halves: the centers of pixels
// would fall on the seam of a conic gradient, which renderers round
// apart), which renderers fold into the geometry
// of its fills, shadows, images and (by whole pixels) glyphs, its clips
// and effects keeping theirs (scene.TranslationFolder).
func TranslatedScenes() []*scene.Scene {
	scene.FoldTranslations() // as package ui does
	var ss []*scene.Scene
	for _, m := range []scene.Affine{scene.Translation(12, 7), scene.Translation(-6.25, 4.25)} {
		s := Scene()
		s.Clear = scene.Color{R: 100, G: 120, B: 160, A: 80}
		for i := range s.Ops {
			s.Ops[i].Transform = m
		}
		ss = append(ss, s)
	}
	return ss
}

// PosedScene is a card an animation poses (AnimatePose, a hero's
// flight: scaled and moved, nothing else): its shadow, its rounded
// background, a rounded clip holding a header, a scroll view's hard
// clip with a picture and text in it, and a picture zoomed within it
// (a transform of its own, which keeps the rounded clip's corners);
// beside it, content the pose does not move. The clips' edges fall in
// the middle of pixels, where the CPU renderer's coverage is exact.
func PosedScene() *scene.Scene {
	r4 := func(r float32) [4]float32 { return [4]float32{r, r, r, r} }
	s := &scene.Scene{Width: 320, Height: 240, Clear: scene.Color{R: 236, G: 238, B: 242, A: 255}}
	s.MaskAtlas, s.Text, s.Glyphs = glyphContent()
	m := scene.Translation(40, 24).Mul(scene.Scaling(0.5, 0.5))
	zoom := m.Mul(scene.Translation(150, 150)).Mul(scene.Scaling(1.7, 1.7)).Mul(scene.Translation(-150, -150))
	card := scene.Rect{X: 21, Y: 21, W: 360, H: 280}
	blue, red, ink := scene.Color{R: 40, G: 60, B: 220, A: 255}, scene.Color{R: 220, G: 40, B: 40, A: 255}, scene.Color{R: 20, G: 20, B: 30, A: 255}
	s.Ops = []scene.Op{
		{Kind: scene.OpFill, Rect: scene.Rect{X: 200, Y: 150, W: 110, H: 80}, Radii: r4(10), Color: scene.Color{R: 90, G: 160, B: 90, A: 255}},
		{Kind: scene.OpShadow, Rect: scene.Rect{X: card.X, Y: card.Y + 8, W: card.W, H: card.H}, Radii: r4(16), Color: scene.Color{A: 90}, Blur: 18, Cast: card, CastRadii: r4(16), Transform: m},
		{Kind: scene.OpFill, Rect: card, Radii: r4(16), Color: scene.Color{R: 250, G: 250, B: 252, A: 255}, Border: scene.Uniform(2), BorderColor: scene.Color{A: 60}, Transform: m},
		{Kind: scene.OpPushClip, Rect: card, Radii: r4(16), Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 0, W: 400, H: 70}, Color: blue, Transform: m},
		{Kind: scene.OpPushClip, Rect: scene.Rect{X: 41, Y: 81, W: 320, H: 200}, Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 30, Y: 70, W: 340, H: 240}, Color: red, Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 60, Y: 100, W: 140, H: 100}, Radii: r4(20), Color: blue, Transform: m},
		{Kind: scene.OpGlyphs, Start: 0, End: int32(len(s.Glyphs)), Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 220, Y: 200, W: 120, H: 90}, Radii: r4(12), Color: ink, Transform: zoom},
		{Kind: scene.OpPopClip, Transform: m},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 300, Y: 240, W: 120, H: 120}, Color: scene.Color{R: 240, G: 180, B: 30, A: 255}, Transform: m},
		{Kind: scene.OpPopClip, Transform: m},
	}
	return s
}
