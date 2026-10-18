package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDraftWriteFailureCanRetryWithoutLosingObservedCopy(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "drafts")
	if err := os.WriteFile(parent, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	draft := &LocalDraft{path: filepath.Join(parent, "draft.json")}
	if err := draft.Replace([]byte("unfinished text")); err == nil {
		t.Fatal("write through a regular file succeeded")
	}
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := draft.Replace([]byte("unfinished text")); err != nil {
		t.Fatal(err)
	}
	// Emulate a replacement whose rename completed before its sync reported
	// failure: the observed copy is old, but retrying identical bytes is safe.
	draft.observed = nil
	if err := draft.Replace([]byte("unfinished text")); err != nil {
		t.Fatalf("identical replacement could not be retried: %v", err)
	}
	info, err := os.Stat(draft.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("draft is not private: %v %v", info, err)
	}
	// A failed delete must leave the original available and retryable.
	if err := os.Rename(draft.path, draft.path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(draft.path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := draft.Replace(nil); err == nil {
		t.Fatal("unexpected object at the draft path was removed")
	}
	if err := os.Remove(draft.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(draft.path+".saved", draft.path); err != nil {
		t.Fatal(err)
	}
	if err := draft.Replace(nil); err != nil {
		t.Fatal(err)
	}
	if data, err := draft.Read(); err != nil || len(data) != 0 {
		t.Fatalf("discard left data: %s %v", data, err)
	}
}
