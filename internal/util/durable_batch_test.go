package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDurableBatchIncludesNewDirectoryAncestorsAndPreservesModes(t *testing.T) {
	root := t.TempDir()
	b := NewDurableBatch()
	dir := filepath.Join(root, "new", "nested")
	if err := b.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "two"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("old"), 0400); err != nil {
			t.Fatal(err)
		}
		if err := b.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0400 {
			t.Fatalf("stricter permissions lost: %v, %v", info, err)
		}
	}
	for _, path := range []string{root, filepath.Join(root, "new"), dir} {
		if !b.dirs[path] {
			t.Fatalf("directory entry would not be made durable: %s", path)
		}
	}
	if len(b.dirs) != 3 {
		t.Fatalf("directories not deduplicated: %v", b.dirs)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(b.dirs) != 0 {
		t.Fatal("successful barrier did not clear pending directories")
	}
}

func TestDurableBatchFailedFlushRemainsPending(t *testing.T) {
	t.Setenv(SkipFsyncEnv, "")
	b := NewDurableBatch()
	missing := filepath.Join(t.TempDir(), "missing")
	if err := b.SyncDir(missing); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err == nil || !b.dirs[missing] {
		t.Fatalf("failed barrier was acknowledged: %v", err)
	}
	if err := os.Mkdir(missing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil || len(b.dirs) != 0 {
		t.Fatalf("barrier retry failed: %v", err)
	}
}

func TestAtomicWriteIncludesPermissionsBeforeFileSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	fileSynced := false
	err := atomicWriteFile(path, []byte("content"), 0400, func(f *os.File) error {
		info, err := f.Stat()
		if err != nil || info.Mode().Perm() != 0400 {
			t.Fatalf("permissions missing at durability barrier: %v, %v", info, err)
		}
		fileSynced = true
		return nil
	}, func(string) error {
		if !fileSynced {
			t.Fatal("directory published before file sync")
		}
		return nil
	})
	if err != nil || !fileSynced {
		t.Fatalf("write: %v", err)
	}
}
