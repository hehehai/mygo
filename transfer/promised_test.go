package transfer

import (
	"context"
	"errors"
	"testing"

	"github.com/egoist/mygo/internal/promised"
)

func TestReceivePromised(t *testing.T) {
	if _, _, err := TextData("x").ReceivePromised(context.Background()); !errors.Is(err, ErrFormat) {
		t.Fatalf("no promise: %v", err)
	}
	token, p := promised.New(1, "/tmp/d")
	p.Add("/tmp/d/pic.jpg", nil)
	d := New(NewItem(Bytes(URIList, []byte("https://example.com/pic.jpg"))), NewItem(Bytes(PromisedFiles, []byte(token))))
	m, err := d.Materialize([]Format{URIList, PromisedFiles})
	if err != nil {
		t.Fatal(err)
	}
	files, dir, err := m.ReceivePromised(context.Background())
	if err != nil || dir != "/tmp/d" || len(files) != 1 {
		t.Fatalf("ReceivePromised = %v %q %v", files, dir, err)
	}
}
