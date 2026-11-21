package ui

import (
	gocontext "context"
	"testing"

	"github.com/egoist/mygo/internal/promised"
	"github.com/egoist/mygo/transfer"
)

// A DropData target listing PromisedFiles takes a drop of promised files
// and reads them by the token the drop's data carries.
func TestPromisedFilesDrop(t *testing.T) {
	var got []transfer.Drop
	tt := coreNewTester(func(c *context) {
		zone := coreBox(c).Size(100, 100)
		opts := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList, transfer.PromisedFiles}, Operations: transfer.Copy}
		if d, ok := coreDropData(zone, opts); ok {
			got = append(got, d)
		}
	}, 200, 200)
	token, p := promised.New(1, "/tmp/d")
	p.Add("/tmp/d/a.jpg", nil)
	if !tt.DropDataAt(20, 20, transfer.New(transfer.NewItem(transfer.Bytes(transfer.PromisedFiles, []byte(token)))), transfer.Copy) {
		t.Fatal("refused")
	}
	tt.Frame()
	if len(got) != 1 {
		t.Fatalf("drops %v", got)
	}
	files, dir, err := got[0].Data.ReceivePromised(gocontext.Background())
	if err != nil || dir != "/tmp/d" || len(files) != 1 || files[0] != "/tmp/d/a.jpg" {
		t.Fatalf("ReceivePromised = %v %q %v", files, dir, err)
	}
}
