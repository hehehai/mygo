package transfer

import (
	"context"
	"strings"

	"github.com/egoist/mygo/internal/promised"
)

// PromisedFiles are files another app promises to write once a drop ends
// rather than giving them on the pasteboard (macOS file promises: a
// picture dragged out of a browser, Photos or Mail). A DropData target
// that lists it takes drags offering promises; the drop's data then holds
// a token, and ReceivePromised waits for the files. Only macOS offers
// it; it is never read from a clipboard.
const PromisedFiles Format = "application/x-mygo-promised-files"

// ReceivePromised waits for the files promised in a drop (PromisedFiles)
// until they are all written or ctx ends, and returns the paths written
// by then with the folder they are in: a new temporary folder the caller
// removes once done with them (a file may still come after ctx ended).
// Call it once, off the main thread. ErrFormat: no promise in d.
func (d Data) ReceivePromised(ctx context.Context) (files []string, dir string, err error) {
	b, err := d.Read(PromisedFiles)
	if err != nil {
		return nil, "", err
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return nil, "", ErrFormat
	}
	return promised.Wait(ctx, token)
}
