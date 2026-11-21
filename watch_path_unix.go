//go:build !windows

package mygo

import (
	"errors"
	"io/fs"
	"os"
)

func fileWatchOpenRoot(path string) (*os.Root, error) { return os.OpenRoot(path) }

// fileWatchHoldsRoot says a watch keeps its directory open between two
// looks. Here it opens it for each look and closes it after: a directory
// held open keeps its volume from being unmounted, so a disk with a
// watched folder on it could not be ejected ("in use"). What the watch
// knows of the directory's identity tells it when it was replaced
// meanwhile.
const fileWatchHoldsRoot = false

// fileWatchInaccessible reports whether err says a file can't be looked
// at.
func fileWatchInaccessible(err error) bool { return errors.Is(err, fs.ErrPermission) }

// fileWatchEntryInfo returns what entry is, read from a directory opened in
// root: reading a directory opened in an os.Root stats every entry as it
// reads it, relative to the directory (fstatat, without following a
// symlink), so that what it says can't come from outside the root
// (TestFileWatchEntryInfo).
func fileWatchEntryInfo(root *os.Root, entry fs.DirEntry) (os.FileInfo, error) {
	return entry.Info()
}
