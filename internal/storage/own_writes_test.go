package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCommitRecordsOwnWrites pins the registry a polling frontend reads: a
// commit leaves the stamp of each path it wrote or deleted, a later write by
// anything else no longer matches it, and a service sharing no registry
// records nothing there.
func TestCommitRecordsOwnWrites(t *testing.T) {
	root := t.TempDir()
	own := NewOwnWrites()
	svc := NewCommitService(filepath.Join(root, ".docket", "transactions"), []string{root}).WithOwnWrites(own)
	other := NewCommitService(filepath.Join(root, ".docket", "transactions"), []string{root})
	written := filepath.Join(root, ".docket", "workspace.json")
	removed := filepath.Join(root, ".docket", "planning.json")
	if err := os.MkdirAll(filepath.Dir(removed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(removed, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := own.Stamp(written); ok {
		t.Fatal("a path no commit touched has an own write")
	}

	if _, err := svc.CommitArtifacts([]ArtifactChange{
		{Path: written, Data: []byte(`{"v":1}`)},
		{Path: removed, Delete: true},
	}); err != nil {
		t.Fatal(err)
	}
	if recorded, ok := own.Stamp(written); !ok || recorded != StampFile(written) {
		t.Fatalf("written path own write = %+v, %t; want its current stamp", recorded, ok)
	}
	if recorded, ok := own.Stamp(removed); !ok || !recorded.Missing {
		t.Fatalf("deleted path own write = %+v, %t; want a recorded absence", recorded, ok)
	}

	if err := os.WriteFile(written, []byte(`{"v":22}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if recorded, _ := own.Stamp(written); recorded == StampFile(written) {
		t.Fatal("a write outside the commit still matches the recorded stamp")
	}

	elsewhere := filepath.Join(root, ".docket", "lenses.json")
	if _, err := other.CommitArtifacts([]ArtifactChange{{Path: elsewhere, Data: []byte("[]")}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := own.Stamp(elsewhere); ok {
		t.Fatal("a service sharing no registry recorded its commit there")
	}
	var none *OwnWrites
	if _, ok := none.Stamp(written); ok {
		t.Fatal("a nil registry reported an own write")
	}
}
