package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// TestLoadOrCreateDomainsNamesACorruptCatalogBeforeMigrating pins the startup
// refusal of a catalog that is not JSON: it names the file and a way back, and
// never blames whichever schema boundary happened to read the bytes first.
func TestLoadOrCreateDomainsNamesACorruptCatalogBeforeMigrating(t *testing.T) {
	for _, tc := range []struct {
		name       string
		withBackup bool
	}{
		{"without a sidecar", false},
		{"with a sidecar", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := newDomainManager(t)
			if _, err := manager.LoadOrCreateDomains(); err != nil {
				t.Fatal(err)
			}
			dataDir := manager.workspaceDataDir()
			catalogPath := WorkspaceCatalogPath(dataDir)
			good, err := os.ReadFile(catalogPath)
			if err != nil {
				t.Fatal(err)
			}
			_ = os.Remove(catalogPath + ".bak")
			if tc.withBackup {
				if writeErr := os.WriteFile(catalogPath+".bak", good, FilePermissions); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			corrupt := []byte("{\"schema_version\": 3, oops")
			if writeErr := os.WriteFile(catalogPath, corrupt, FilePermissions); writeErr != nil {
				t.Fatal(writeErr)
			}

			_, err = manager.LoadOrCreateDomains()
			if !errors.Is(err, ErrCorruptWorkspaceCatalog) {
				t.Fatalf("load error = %v, want ErrCorruptWorkspaceCatalog", err)
			}
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Errorf("load error %v does not wrap the JSON syntax error", err)
			}
			message := err.Error()
			wants := []string{DisplayPath(catalogPath), DisplayPath(PreScopeV7BackupsDir(dataDir)), "tt doctor"}
			if tc.withBackup {
				wants = append(wants, DisplayPath(catalogPath+".bak"))
			}
			for _, want := range wants {
				if !strings.Contains(message, want) {
					t.Errorf("load error %q does not name %q", message, want)
				}
			}
			if !tc.withBackup && strings.Contains(message, ".bak") {
				t.Errorf("load error %q offers a sidecar that does not exist", message)
			}
			if strings.Contains(message, "schema v") {
				t.Errorf("load error %q blames a schema boundary", message)
			}
			// The refusal is read-only: neither copy was rewritten.
			if after, _ := os.ReadFile(catalogPath); string(after) != string(corrupt) {
				t.Errorf("corrupt catalog was rewritten to %q", after)
			}
		})
	}
}
