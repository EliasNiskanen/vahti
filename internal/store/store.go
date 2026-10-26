// Package store writes JSON files atomically so a crash or power cut never
// leaves a half-written state file behind.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path via a temp file, fsync and rename.
// The previous version is kept as path+".bak".
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path + ".bak")
		if err := os.Link(path, path+".bak"); err != nil {
			// hard links may be unsupported; fall back to a copy
			if b, rerr := os.ReadFile(path); rerr == nil {
				_ = os.WriteFile(path+".bak", b, perm)
			}
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// SaveJSON marshals v and writes it atomically with 0600 permissions.
func SaveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, b, 0o600)
}

// ErrNotExist is returned by LoadJSON when neither file nor backup exists.
var ErrNotExist = errors.New("state file does not exist")

// LoadJSON reads path into v, falling back to path+".bak" if the main file
// is missing or corrupt. It reports whether the backup was used.
func LoadJSON(path string, v any) (usedBackup bool, err error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if jerr := json.Unmarshal(b, v); jerr == nil {
			return false, nil
		} else {
			err = fmt.Errorf("%s is corrupt: %w", path, jerr)
		}
	}
	mainErr := err
	b, berr := os.ReadFile(path + ".bak")
	if berr != nil {
		if errors.Is(mainErr, os.ErrNotExist) && errors.Is(berr, os.ErrNotExist) {
			return false, ErrNotExist
		}
		return false, mainErr
	}
	if jerr := json.Unmarshal(b, v); jerr != nil {
		return false, fmt.Errorf("%v; backup also corrupt: %w", mainErr, jerr)
	}
	return true, nil
}
