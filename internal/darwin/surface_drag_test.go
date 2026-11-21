//go:build darwin && !ios

package darwin

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
)

// Format translation needs Foundation/AppKit, but no application or window.
func TestDataDragFormatMappings(t *testing.T) {
	load()
	withPool(func() {
		registerDataDragClasses()
		for _, f := range []transfer.Format{transfer.Text, transfer.HTML, transfer.PNG, transfer.URIList, transfer.FileList, "application/vnd.mygo.test+json"} {
			typ := macType(f)
			if got := macFormat(typ); got != f {
				t.Errorf("%s -> %s -> %s", f, typ, got)
			}
			item := autorelease(send(send(class("NSPasteboardItem"), "alloc"), "init"))
			if !sendBool(item, "setData:forType:", uintptr(nsData([]byte("a\r\nb"))), uintptr(nsString(typ))) {
				t.Errorf("pasteboard rejected type %s", typ)
			}
		}
		if macType(transfer.URIList) == "public.url" {
			t.Fatal("URI lists were narrowed to a single URL")
		}
		payload := []byte("https://example.com/a\r\nhttps://example.com/b")
		src := &macDataSource{r: platform.DragRequest{Data: transfer.New(transfer.NewItem(transfer.Bytes(transfer.URIList, payload)))}}
		item := src.pasteboardItems()[0]
		defer func() {
			for _, p := range src.providers {
				delete(macDragProviders, p)
				release(p)
			}
		}()
		actual := goBytes(send(item, "dataForType:", uintptr(nsString(macType(transfer.URIList)))))
		if !bytes.Equal(actual, payload) {
			t.Errorf("URI-list provider: %q", actual)
		}
		first := goBytes(send(item, "dataForType:", uintptr(nsString("public.url"))))
		if string(first) != "https://example.com/a" {
			t.Errorf("public URL fallback: %q", first)
		}
	})
}

// Finder offers only the generic operation for files dragged with Command
// held: a destination takes them as a copy and answers generic.
func TestDataDragGenericOperation(t *testing.T) {
	for _, c := range []struct {
		mask uint
		want transfer.Operation
	}{{1 | 16, transfer.Copy | transfer.Move}, {4, transfer.Copy}, {1 | 4, transfer.Copy}, {2, transfer.None}, {0, transfer.None}, {^uint(0), transfer.Copy | transfer.Move}} {
		if got := goOperations(c.mask); got != c.want {
			t.Errorf("mask %d: %v, want %v", c.mask, got, c.want)
		}
	}
	if got := destinationOperation(transfer.Copy, 4); got != 4 {
		t.Errorf("a copy for a generic source: %d, want 4", got)
	}
	if got := destinationOperation(transfer.Copy, 1|16); got != 1 {
		t.Errorf("a copy for a copy source: %d, want 1", got)
	}
	if got := destinationOperation(transfer.None, 4); got != 0 {
		t.Errorf("nothing taken: %d, want 0", got)
	}
}
