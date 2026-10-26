//go:build unix

// Package lock prevents two Vahti processes from using one data directory.
package lock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// ErrBusy means another process holds the lock.
var ErrBusy = errors.New("another Vahti process is using this data directory")

// Acquire takes an exclusive lock on dir/.lock. Keep the file open to hold it.
func Acquire(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return f, nil
}

// MatchOwner gives paths the same owner as dir when running as root, so files
// written by "sudo vahti ..." stay writable for the service user.
func MatchOwner(dir string, paths ...string) {
	if os.Geteuid() != 0 {
		return
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	for _, p := range paths {
		_ = os.Lchown(p, int(st.Uid), int(st.Gid))
	}
}
