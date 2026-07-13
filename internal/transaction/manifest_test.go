package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion,
		ID:            "txn-1",
		State:         StatePrepared,
		CreatedAt:     "2026-07-10T00:00:00Z",
		Ops: []FileOp{
			{Kind: OpWrite, Target: "/data/projects/a/tasks.md", Staged: "/data/projects/a/.tt-stage-txn-1-000.stage", Mode: 0o600},
		},
	}
}

func TestManifestValidate_Valid(t *testing.T) {
	if err := validManifest().Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

func TestManifestValidate_Rejections(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Manifest)
		wantErr error
	}{
		{"missing schema", func(m *Manifest) { m.SchemaVersion = 0 }, ErrInvalidManifest},
		{"future schema", func(m *Manifest) { m.SchemaVersion = SchemaVersion + 1 }, ErrUnsupportedSchema},
		{"missing id", func(m *Manifest) { m.ID = "" }, ErrInvalidManifest},
		{"unknown state", func(m *Manifest) { m.State = "bogus" }, ErrInvalidManifest},
		{"no ops", func(m *Manifest) { m.Ops = nil }, ErrInvalidManifest},
		{"op missing target", func(m *Manifest) { m.Ops[0].Target = "" }, ErrInvalidManifest},
		{"write missing staged", func(m *Manifest) { m.Ops[0].Staged = "" }, ErrInvalidManifest},
		{"unknown op kind", func(m *Manifest) { m.Ops[0].Kind = "frob" }, ErrUnknownOpKind},
		{"move missing source", func(m *Manifest) {
			m.Ops[0] = FileOp{Kind: OpMove, Target: "/data/projects/a/tasks.md"}
		}, ErrInvalidManifest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.mutate(m)
			err := m.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() = %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

func TestManifestValidate_MoveAndDeleteOps(t *testing.T) {
	m := &Manifest{
		SchemaVersion: SchemaVersion,
		ID:            "txn-2",
		State:         StatePrepared,
		CreatedAt:     "2026-07-10T00:00:00Z",
		Ops: []FileOp{
			{Kind: OpMove, Target: "/data/projects/b/tasks.md", Source: "/data/projects/a/tasks.md"},
			{Kind: OpDelete, Target: "/data/projects/c/tasks.md"},
		},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("move+delete manifest rejected: %v", err)
	}
}

// TestLoadManifest_RoundTrip prepares a real transaction, reloads its manifest
// from disk, and confirms every field survives the JSON round-trip.
func TestLoadManifest_RoundTrip(t *testing.T) {
	root := t.TempDir()
	j := newTestJournal(t, root)

	target := filepath.Join(root, "projects", "a", "tasks.md")
	moveSrc := filepath.Join(root, "projects", "a", "old.md")
	deleteTarget := filepath.Join(root, "projects", "a", "gone.md")
	prepared, err := j.Prepare([]Op{
		{Kind: OpWrite, Target: target, Data: []byte("hello"), Mode: 0o600},
		{Kind: OpMove, Target: filepath.Join(root, "projects", "a", "new.md"), Source: moveSrc},
		{Kind: OpDelete, Target: deleteTarget},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	loaded, err := LoadManifest(j.ManifestPath(prepared.ID))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.SchemaVersion != prepared.SchemaVersion || loaded.ID != prepared.ID ||
		loaded.State != prepared.State || loaded.CreatedAt != prepared.CreatedAt {
		t.Fatalf("header mismatch: loaded %+v, prepared %+v", loaded, prepared)
	}
	if len(loaded.Ops) != len(prepared.Ops) {
		t.Fatalf("op count = %d, want %d", len(loaded.Ops), len(prepared.Ops))
	}
	for i := range prepared.Ops {
		if loaded.Ops[i] != prepared.Ops[i] {
			t.Fatalf("op %d mismatch: loaded %+v, prepared %+v", i, loaded.Ops[i], prepared.Ops[i])
		}
	}
}

func TestLoadManifest_RejectsCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected error decoding corrupt manifest")
	}
}
