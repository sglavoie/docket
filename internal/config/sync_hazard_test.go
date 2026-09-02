package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// These tests cover the cross-machine hazards a synced workspace exposes at the
// domain layer: a machine still running an older binary against a catalog a
// newer one already advanced, and a lens set arriving from an arbitrary number
// of machines in an arbitrary order. Neither can be expressed as a scenario —
// the TUI driver has one app, one binary, and one machine home — so they live
// here, where a second home and a hand-written schema marker are both reachable.
//
// This file holds the older-binary hazard and the renderings the lens hazards
// share. The lens halves are beside it: the convergence property and the join
// laws in sync_hazard_lens_adoption_test.go, the identity bound in
// sync_hazard_lens_identity_test.go, and the stale-write path in
// sync_hazard_lens_reconcile_test.go.

// TestOlderBinaryRefusesNewerCatalog is the first machine to sync a catalog a
// newer binary published. Refusing is the whole contract: there is no
// down-migration, so an older build that opened the workspace anyway would write
// a schema it cannot represent back over the newer one.
func TestOlderBinaryRefusesNewerCatalog(t *testing.T) {
	home := t.TempDir()
	dataDir, _ := stageSharedWorkspaceOn(t, home)

	bumpCatalogSchemaVersion(t, WorkspaceCatalogPath(dataDir), CurrentConfigVersion+1)
	before := snapshotWorkspaceFiles(t, dataDir)

	manager := NewManagerOnMachine(ConfigPathForDataDir(dataDir), home)
	_, err := manager.LoadOrCreateDomains()
	if err == nil {
		t.Fatal("LoadOrCreateDomains accepted a catalog newer than this build supports")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("refusal = %v, want it to name the unsupported schema version", err)
	}

	// A refusal that rewrote anything would be worse than no refusal: the older
	// binary would have touched a workspace it cannot represent.
	after := snapshotWorkspaceFiles(t, dataDir)
	if len(before) != len(after) {
		t.Fatalf("workspace file count changed from %d to %d during the refusal", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("refusal rewrote %q", path)
		}
	}
}

// bumpCatalogSchemaVersion rewrites only the schema marker in workspace.json,
// leaving every other key exactly as written. Decoding into WorkspaceCatalog and
// re-encoding would drop unknown keys, which is precisely what a newer binary's
// catalog has.
func bumpCatalogSchemaVersion(t *testing.T, catalogPath string, version int) {
	t.Helper()
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var decoded map[string]any
	if decodeErr := json.Unmarshal(raw, &decoded); decodeErr != nil {
		t.Fatalf("decode catalog: %v", decodeErr)
	}
	decoded["schema_version"] = version
	encoded, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		t.Fatalf("encode catalog: %v", err)
	}
	if err := os.WriteFile(catalogPath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
}

// snapshotWorkspaceFiles records every regular file under the data directory by
// content, keyed on the path relative to it.
func snapshotWorkspaceFiles(t *testing.T, dataDir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dataDir, path)
		if relErr != nil {
			return relErr
		}
		files[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot workspace files: %v", err)
	}
	return files
}

// canonicalLensRecords renders a collection as its records alone, sorted so two
// collections holding the same lenses in different slots compare equal. Nothing
// is blanked: id, name, mode, scope, query and sort all take part, and the
// length of the rendering carries the cardinality.
func canonicalLensRecords(t *testing.T, list NamedLensList) string {
	t.Helper()
	rendered := make([]string, 0, len(list))
	for _, lens := range list {
		encoded, err := json.Marshal(lens)
		if err != nil {
			t.Fatalf("render lens %q: %v", lens.ID, err)
		}
		rendered = append(rendered, string(encoded))
	}
	sort.Strings(rendered)
	return "[" + strings.Join(rendered, ",") + "]"
}

// definitionKeySet reduces collections to the set of definitions they hold, with
// identity and name erased so a derived id or a disambiguated name does not read
// as a different lens.
func definitionKeySet(lists ...NamedLensList) map[string]bool {
	keys := make(map[string]bool)
	for _, list := range lists {
		for _, lens := range list {
			keys[definitionKey(lens)] = true
		}
	}
	return keys
}

func definitionKey(lens NamedLens) string {
	anonymous := lens.Clone()
	anonymous.ID = ""
	anonymous.Name = ""
	encoded, err := json.Marshal(anonymous)
	if err != nil {
		// Marshaling a lens cannot fail, and a key that hid the failure would
		// silently collapse two definitions into one.
		return "unmarshalable:" + lens.ID
	}
	return string(encoded)
}
