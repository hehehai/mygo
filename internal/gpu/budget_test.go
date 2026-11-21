package gpu

import (
	"slices"
	"testing"
)

type img struct {
	last  uint64
	bytes int64
}

func TestImageBudgetTrims(t *testing.T) {
	var b ImageBudget
	images := map[uint64]*img{}
	var released []uint64
	add := func(key, last uint64, bytes int64) {
		images[key] = &img{last, bytes}
		b.Add(bytes)
	}
	trim := func(frame uint64) {
		TrimImages(&b, images, frame, func(i *img) uint64 { return i.last }, func(i *img) int64 { return i.bytes },
			func(key uint64, i *img) {
				released = append(released, key)
				b.Add(-i.bytes)
				delete(images, key)
			})
	}
	for k := uint64(1); k <= 10; k++ {
		add(k, k, 100)
	}
	// A texture the budget does not count (a GPU source's) never goes.
	add(99, 0, 0)
	// No bound: nothing goes.
	trim(10)
	if len(images) != 11 || b.Kept() != 1000 {
		t.Fatal(len(images), b.Kept())
	}
	// 450 bytes: the oldest go first, until under it.
	b.Set(450, 0)
	trim(10)
	if !slices.Equal(released, []uint64{1, 2, 3, 4, 5, 6}) || b.Kept() != 400 || len(images) != 5 {
		t.Fatalf("released %v, kept %d", released, b.Kept())
	}
	// Those drawn in the last KeepFrames frames stay, even past the budget.
	released = nil
	b.Set(100, 0)
	trim(10) // 7 was drawn 3 frames ago
	if len(released) != 0 {
		t.Fatalf("released %v", released)
	}
	trim(11)
	if !slices.Equal(released, []uint64{7}) || b.Kept() != 300 {
		t.Fatalf("recent released: %v, kept %d", released, b.Kept())
	}
	if images[99] == nil {
		t.Fatal("a texture counting no bytes was released")
	}
	b.Reset()
	if b.Kept() != 0 {
		t.Fatal(b.Kept())
	}
}

func TestImageBudgetAdmits(t *testing.T) {
	var b ImageBudget
	b.BeginFrame()
	for range 100 {
		if !b.Admit() {
			t.Fatal("no bound refused")
		}
	}
	if b.Deferred() {
		t.Fatal("deferred without a bound")
	}
	b.Set(0, 2)
	b.BeginFrame()
	got := []bool{b.Admit(), b.Admit(), b.Admit(), b.Admit()}
	if !slices.Equal(got, []bool{true, true, false, false}) || !b.Deferred() {
		t.Fatalf("%v %v", got, b.Deferred())
	}
	b.BeginFrame()
	if !b.Admit() || b.Deferred() {
		t.Fatal("a new frame")
	}
	b.Set(-5, -1)
	if b.Bytes != 0 || b.Uploads != 0 {
		t.Fatal(b)
	}
}

func TestImageBudgetAdmitAll(t *testing.T) {
	var b ImageBudget
	b.Set(0, 2)
	b.AdmitAll()
	b.BeginFrame() // the frame after the textures went: all of them
	for range 10 {
		if !b.Admit() {
			t.Fatal("refused after AdmitAll")
		}
	}
	if b.Deferred() {
		t.Fatal("deferred after AdmitAll")
	}
	b.BeginFrame() // and that frame only
	got := []bool{b.Admit(), b.Admit(), b.Admit()}
	if !slices.Equal(got, []bool{true, true, false}) || !b.Deferred() {
		t.Fatalf("the frame after: %v %v", got, b.Deferred())
	}
}
