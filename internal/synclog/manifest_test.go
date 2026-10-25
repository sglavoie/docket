package synclog

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func sampleManifest(t *testing.T) Manifest {
	t.Helper()
	base, blob := Digest([]byte("before <&>")), Digest([]byte("after"))
	m := Manifest{
		Format:        ManifestFormat,
		FormatVersion: ManifestFormatVersion,
		WorkspaceID:   "ws",
		DeviceID:      "device-a",
		DeviceSeq:     3,
		Parents:       []string{Digest([]byte("p1")), Digest([]byte("p2"))},
		Kind:          KindUser,
		LocalTxnID:    "journal<&>1",
		CreatedAt:     "2026-10-06T12:00:00Z",
		Schemas:       map[string]int{SchemaCatalog: 21, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 19},
		Requires:      []string{"project-document.v19"},
		Effects: []Effect{{
			Kind: EffectProjectDocument, ProjectID: "p", Path: "projects/a/tasks.md", Op: OpPut,
			BaseBlob: &base, Blob: &blob,
		}},
	}
	if m.Parents[0] > m.Parents[1] {
		m.Parents[0], m.Parents[1] = m.Parents[1], m.Parents[0]
	}
	if err := m.seal(); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestSyncManifestContentAddressIsCanonical shows the txn_id is a function of
// content alone: formatting does not move it, any edit invalidates it.
func TestSyncManifestContentAddressIsCanonical(t *testing.T) {
	m := sampleManifest(t)
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	canonical, err := m.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(canonical, []byte(`"txn_id"`)) || bytes.Contains(canonical, []byte(" ")) || !bytes.Contains(canonical, []byte("journal<&>1")) {
		t.Fatalf("canonical form is not compact, txn_id-free, unescaped UTF-8: %s", canonical)
	}
	if !bytes.HasPrefix(canonical, []byte(`{"created_at":`)) {
		t.Fatalf("canonical keys are not sorted: %s", canonical)
	}
	indented, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	compact := &bytes.Buffer{}
	if err := json.Compact(compact, indented); err != nil {
		t.Fatal(err)
	}
	for _, form := range [][]byte{indented, compact.Bytes()} {
		decoded, err := DecodeManifest(form)
		if err != nil {
			t.Fatalf("DecodeManifest: %v", err)
		}
		if decoded.TxnID != m.TxnID {
			t.Fatalf("decoded txn_id %s, want %s", decoded.TxnID, m.TxnID)
		}
	}
	tampered := strings.Replace(string(indented), `"device_seq": 3`, `"device_seq": 4`, 1)
	if _, err := DecodeManifest([]byte(tampered)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("tampered manifest error = %v, want ErrInvalidManifest", err)
	}
	newer := strings.Replace(string(indented), `"format_version": 1`, `"format_version": 2`, 1)
	if _, err := DecodeManifest([]byte(newer)); !errors.Is(err, ErrHeld) {
		t.Fatalf("newer manifest error = %v, want ErrHeld", err)
	}
}

// TestSyncManifestRefusesIncompleteEffects keeps every effect explicit.
func TestSyncManifestRefusesIncompleteEffects(t *testing.T) {
	digest := Digest([]byte("x"))
	other := Digest([]byte("y"))
	cases := map[string]Effect{
		"delete without base":   {Kind: EffectNote, ProjectID: "p", Path: "projects/a/notes/n.md", Op: OpDelete},
		"put without blob":      {Kind: EffectNote, ProjectID: "p", Path: "projects/a/notes/n.md", Op: OpPut},
		"escaping path":         {Kind: EffectNote, ProjectID: "p", Path: "../projects/a/notes/n.md", Op: OpPut, Blob: &digest},
		"absolute path":         {Kind: EffectCatalog, Path: "/.docket/workspace.json", Op: OpPut, Blob: &digest},
		"device-local path":     {Kind: EffectCatalog, Path: ".docket/undo/ledger.json", Op: OpPut, Blob: &digest},
		"kind mismatch":         {Kind: EffectNote, ProjectID: "p", Path: "projects/a/tasks.md", Op: OpPut, Blob: &digest},
		"project kind no id":    {Kind: EffectProjectDocument, Path: "projects/a/tasks.md", Op: OpPut, Blob: &digest},
		"move changing content": {Kind: EffectNote, ProjectID: "p", Path: "projects/a/notes/n.md", TargetPath: "projects/a/notes/m.md", Op: OpMove, BaseBlob: &digest, Blob: &other},
	}
	for name, effect := range cases {
		t.Run(name, func(t *testing.T) {
			m := sampleManifest(t)
			m.Effects = []Effect{effect}
			if err := m.seal(); err != nil {
				t.Fatal(err)
			}
			if err := m.Validate(); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("Validate = %v, want ErrInvalidManifest", err)
			}
		})
	}
}

// TestSyncClassifyPaths pins which paths are exchanged and which never are.
func TestSyncClassifyPaths(t *testing.T) {
	synced := map[string]EffectKind{
		".docket/workspace.json":                   EffectCatalog,
		".docket/planning.json":                    EffectPlanning,
		".docket/lenses.json":                      EffectLenses,
		".docket/global-notes/2026-weekly.md":      EffectGlobalNote,
		"projects/atlas/tasks.md":                  EffectProjectDocument,
		"projects/atlas/notes/runbook.md":          EffectNote,
		"projects/atlas/.attachments/abc/file.txt": EffectAttachment,
		"projects/atlas/blockers.md":               EffectProjectTree,
	}
	for rel, want := range synced {
		kind, _, local, err := Classify(rel)
		if err != nil || local || kind != want {
			t.Fatalf("Classify(%s) = %s local=%v err=%v, want %s", rel, kind, local, err, want)
		}
	}
	for _, rel := range []string{
		".docket/transactions/x.json", ".docket/undo/ledger.json", ".docket/sync/devices/d/state.json",
		".docket/backups/snap/workspace.json", ".docket/conflicts/c.json", ".docket/workspace.json.bak",
		"projects/atlas/.tasks.md.lock", ".docket/planning.json.lock", "projects/atlas/.tt-stage-x-001.stage",
	} {
		if _, _, local, err := Classify(rel); err != nil || !local {
			t.Fatalf("Classify(%s) local=%v err=%v, want device-local", rel, local, err)
		}
	}
	for _, rel := range []string{"stray.txt", ".docket/index.json", "projects/tasks.md"} {
		if _, _, _, err := Classify(rel); !errors.Is(err, ErrUnclassifiedPath) {
			t.Fatalf("Classify(%s) err=%v, want ErrUnclassifiedPath", rel, err)
		}
	}
}

// TestClassifyReadsPreV24RecordsThroughTheirCanonicalPath keeps a record sealed
// before schema 24 valid while the tombstone now at .tt/ is never synced.
func TestClassifyReadsPreV24RecordsThroughTheirCanonicalPath(t *testing.T) {
	if got := CanonicalPath(".tt/workspace.json"); got != ".docket/workspace.json" {
		t.Fatalf("CanonicalPath = %q", got)
	}
	for _, rel := range []string{"projects/atlas/tasks.md", ".tt-stage-x", ".docket/planning.json"} {
		if got := CanonicalPath(rel); got != rel {
			t.Fatalf("CanonicalPath(%q) = %q, want it unchanged", rel, got)
		}
	}
	if _, _, local, err := Classify(".tt/workspace.json"); err != nil || !local {
		t.Fatalf("the tombstone catalog classifies as local=%v err=%v, want device-local", local, err)
	}
	digest := Digest([]byte("catalog"))
	legacy := Effect{Kind: EffectCatalog, Path: ".tt/workspace.json", Op: OpPut, Blob: &digest}
	if err := legacy.validate(); err != nil {
		t.Fatalf("a pre-v24 catalog effect no longer validates: %v", err)
	}
}
