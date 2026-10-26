package store

import (
	"os"
	"path/filepath"
	"testing"
)

// State is saved privately and atomically with a backup copy, a half-written file
// is recovered from the backup, and no temporary files are left behind.
func TestAtomicSaveAndBackupRecovery(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	var v map[string]int
	if _, err := LoadJSON(p, &v); err != ErrNotExist {
		t.Fatalf("missing file: %v", err)
	}
	if err := SaveJSON(p, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := SaveJSON(p, map[string]int{"a": 2}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("state must be private, mode %v", fi.Mode().Perm())
	}
	// simulate a torn write of the main file
	if err := os.WriteFile(p, []byte(`{"a": `), 0o600); err != nil {
		t.Fatal(err)
	}
	v = nil
	used, err := LoadJSON(p, &v)
	if err != nil || !used || v["a"] != 1 {
		t.Fatalf("expected recovery from backup, got used=%v v=%v err=%v", used, v, err)
	}
	if m, _ := filepath.Glob(filepath.Join(dir, ".state.json.tmp-*")); len(m) != 0 {
		t.Fatal("temp files left behind")
	}
}
