//go:build !unix

package lock

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrBusy means another process holds the lock.
var ErrBusy = errors.New("another Vahti process is using this data directory")

// Acquire opens dir/.lock. File locking is only enforced on Unix systems.
func Acquire(dir string) (*os.File, error) {
	return os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
}

// MatchOwner is a no-op outside Unix.
func MatchOwner(dir string, paths ...string) {}
