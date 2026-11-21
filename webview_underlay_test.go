package mygo

import (
	"image"
	"image/color"
	"testing"
)

// TestUnderlaySameImage: SetUnderlay sends a picture again only when it
// is another one; a picture of an uncomparable type is always another.
func TestUnderlaySameImage(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 2, 2))
	b := image.NewRGBA(image.Rect(0, 0, 2, 2))
	u := image.NewUniform(color.Black)
	for _, c := range []struct {
		x, y image.Image
		want bool
	}{{nil, nil, true}, {a, a, true}, {a, b, false}, {a, nil, false}, {nil, a, false}, {u, u, true}} {
		if got := sameImage(c.x, c.y); got != c.want {
			t.Errorf("sameImage(%p, %p) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
