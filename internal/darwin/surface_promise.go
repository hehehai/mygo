//go:build darwin

package darwin

// File promises (transfer.PromisedFiles): a drag whose source writes its
// files only once the drop ends — a picture dragged out of a browser,
// Photos or Mail — offers promise types, not files. A view whose content
// takes PromisedFiles registers the types NSFilePromiseReceiver reads;
// a drop taken asks the sources to write into a new temporary folder and
// gives the content a token for the files (internal/promised).

import (
	"errors"
	"os"
	"slices"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/promised"
	"github.com/egoist/mygo/transfer"
)

// promiseWait is how long a drop's promise waits for its files at most.
const promiseWait = 2 * time.Minute

// macPromiseType is PromisedFiles' type: never on a pasteboard, it only
// names the format.
const macPromiseType = "dev.mygo.promised-files"

var macPromiseTypes []string // NSFilePromiseReceiver.readableDraggedTypes, once read

// filePromiseTypes are the pasteboard types of file promises.
func filePromiseTypes() []string {
	if macPromiseTypes == nil {
		macPromiseTypes = []string{}
		if cls := class("NSFilePromiseReceiver"); cls != 0 {
			for _, t := range arrayItems(send(cls, "readableDraggedTypes")) {
				macPromiseTypes = append(macPromiseTypes, goString(t))
			}
		}
	}
	return macPromiseTypes
}

// isPromiseType reports whether a pasteboard type is a file promise's.
func isPromiseType(t string) bool { return slices.Contains(filePromiseTypes(), t) }

// promiseTypeIDs are filePromiseTypes as NSStrings.
func promiseTypeIDs() []id {
	var out []id
	for _, t := range filePromiseTypes() {
		out = append(out, nsString(t))
	}
	return out
}

// receivePromises asks the sources of pb's promises to write their files
// into a new temporary folder and returns the token of the files ("" no
// promise there). Main thread, while the drop is performed.
func receivePromises(pb id) string {
	cls := class("NSFilePromiseReceiver")
	if cls == 0 {
		return ""
	}
	receivers := arrayItems(send(pb, "readObjectsForClasses:options:", uintptr(nsArray(cls)), 0))
	if len(receivers) == 0 {
		return ""
	}
	dir, err := os.MkdirTemp("", "mygo-promised-")
	if err != nil {
		return ""
	}
	want := 0
	for _, r := range receivers {
		want += max(1, sendInt(send(r, "fileTypes"), "count"))
	}
	token, p := promised.New(want, dir)
	dest := send(class("NSURL"), "fileURLWithPath:isDirectory:", uintptr(nsString(dir)), 1)
	queue := send(class("NSOperationQueue"), "mainQueue") // the reader runs on the main thread
	opts := send(class("NSDictionary"), "dictionary")
	var kept []id
	for _, r := range receivers {
		kept = append(kept, retain(r))
		reader := newBlock(func(_ objc.Block, u id, nserr id) {
			switch {
			case nserr != 0:
				p.Add("", nsError(nserr))
			case u == 0:
				p.Add("", errors.New("mygo: a promised file was not written"))
			default:
				p.Add(goString(send(u, "path")), nil)
			}
		})
		send(r, "receivePromisedFilesAtDestination:options:operationQueue:reader:", uintptr(dest), uintptr(opts), uintptr(queue), uintptr(reader))
		reader.Release()
	}
	time.AfterFunc(promiseWait, func() {
		p.End()
		if b := theBackend; b != nil {
			b.runOnMain(func() {
				for _, r := range kept {
					release(r)
				}
			})
		}
	})
	return token
}

// promiseItem is the drop's item for PromisedFiles: the token.
func promiseItem(token string) transfer.Item {
	return transfer.NewItem(transfer.Bytes(transfer.PromisedFiles, []byte(token)))
}
