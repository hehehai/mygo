package gpu

import (
	"math"

	"github.com/egoist/mygo/internal/scene"
)

// frameClip returns the rectangle and the radii of its corners in the
// frame of a clip whose transform only scales it, by positive amounts,
// and moves it, which maps it to a rectangle: the same both ways when its
// corners are rounded (they stay circular). Its shape is then cut in the
// shader as a clip without a transform, where another transform's clip
// is masked: an element posed every frame of an animation (AnimatePose,
// a hero's flight) would rasterize the mask of each clip in it on the CPU
// every frame. A clip a gradient or an image masks keeps its transform.
func frameClip(c *scene.ClipShape) (scene.Rect, [4]float32, bool) {
	m := c.Transform
	if !m.Set || m.Behind || m.B != 0 || m.C != 0 || m.G != 0 || m.H != 0 || !(m.A > 0) || !(m.D > 0) || c.Masks() {
		return scene.Rect{}, [4]float32{}, false
	}
	radii := c.Radii
	if radii != [4]float32{} {
		if math.Abs(float64(m.A-m.D)) > 1e-5*float64(max(m.A, m.D)) {
			return scene.Rect{}, [4]float32{}, false // elliptical corners
		}
		for i := range radii {
			radii[i] *= m.A
		}
	}
	r := c.Rect
	return scene.Rect{X: m.A*r.X + m.X, Y: m.D*r.Y + m.Y, W: m.A * r.W, H: m.D * r.H}, radii, true
}

// frameClipOp is frameClip of op, an OpPushClip.
func frameClipOp(op *scene.Op) (scene.Rect, [4]float32, bool) {
	c := scene.ClipShape{Rect: op.Rect, Radii: op.Radii, Transform: op.Transform, Paint: op.Paint}
	if op.Image != nil {
		return scene.Rect{}, [4]float32{}, false
	}
	return frameClip(&c)
}

// framePush returns op, an OpPushClip, in the frame when frameClip cuts it
// there, and false otherwise.
func framePush(op *scene.Op) (scene.Op, bool) {
	r, radii, ok := frameClipOp(op)
	if !ok {
		return scene.Op{}, false
	}
	o := *op
	o.Rect, o.Radii, o.Transform = r, radii, scene.Affine{}
	return o, true
}

// reshapesOwn reports whether what op draws is reshaped by more than its
// own transform: a glyph's of its own, which a clip posed as op is does
// not hold as drawing without a transform.
func (b *Builder) reshapesOwn(s *scene.Scene, op *scene.Op) bool {
	o := *op
	o.Transform = scene.Affine{}
	return scene.Reshaped(s, &o)
}
