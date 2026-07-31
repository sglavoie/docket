package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteMigratedConfig_RewritesLegacyPaths(t *testing.T) {
	legacy := "/home/u/.tt"
	newDir := "/home/u/Library/Mobile Documents/com~apple~CloudDocs/tt"

	data := []byte(`{
		"version": 1,
		"data_dir": "/home/u/.tt",
		"custom_top_level": {"keep": true},
		"projects": [
			{"id": "p1", "slug": "alpha", "name": "Alpha", "path": "/home/u/.tt/projects/alpha"},
			{"id": "p2", "slug": "beta", "name": "Beta", "path": "/home/u/.tt/projects/beta"}
		]
	}`)

	out, err := RewriteMigratedConfig(data, legacy, newDir)
	if err != nil {
		t.Fatalf("RewriteMigratedConfig: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	var dataDir string
	_ = json.Unmarshal(raw["data_dir"], &dataDir)
	if dataDir != newDir {
		t.Errorf("data_dir = %q, want %q", dataDir, newDir)
	}

	// Unknown top-level field preserved.
	if _, ok := raw["custom_top_level"]; !ok {
		t.Error("custom_top_level was dropped")
	}

	var projects []map[string]string
	if err := json.Unmarshal(raw["projects"], &projects); err != nil {
		t.Fatalf("unmarshal projects: %v", err)
	}
	wantPaths := map[string]string{
		"p1": filepath.Join(newDir, "projects", "alpha"),
		"p2": filepath.Join(newDir, "projects", "beta"),
	}
	for _, p := range projects {
		if want := wantPaths[p["id"]]; p["path"] != want {
			t.Errorf("project %s path = %q, want %q", p["id"], p["path"], want)
		}
	}
}

func TestRewriteMigratedConfig_PreservesCustomPaths(t *testing.T) {
	legacy := "/home/u/.tt"
	newDir := "/home/u/icloud/tt"

	data := []byte(`{
		"data_dir": "/opt/custom/tt",
		"projects": [
			{"id": "p1", "slug": "alpha", "path": "/home/u/.tt/projects/alpha"},
			{"id": "p2", "slug": "beta", "path": "/opt/custom/tt/projects/beta"},
			{"id": "p3", "slug": "gamma", "path": "/somewhere/else/gamma"}
		]
	}`)

	out, err := RewriteMigratedConfig(data, legacy, newDir)
	if err != nil {
		t.Fatalf("RewriteMigratedConfig: %v", err)
	}

	var parsed struct {
		DataDir  string `json:"data_dir"`
		Projects []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Custom data_dir must be preserved (not legacy).
	if parsed.DataDir != "/opt/custom/tt" {
		t.Errorf("custom data_dir = %q, want unchanged", parsed.DataDir)
	}

	want := map[string]string{
		"p1": filepath.Join(newDir, "projects", "alpha"), // legacy -> rewritten
		"p2": "/opt/custom/tt/projects/beta",             // custom -> preserved
		"p3": "/somewhere/else/gamma",                    // custom -> preserved
	}
	for _, p := range parsed.Projects {
		if p.Path != want[p.ID] {
			t.Errorf("project %s path = %q, want %q", p.ID, p.Path, want[p.ID])
		}
	}
}

func TestRewriteMigratedConfig_RewritesEmptyAndMissingDataDir(t *testing.T) {
	legacy := "/home/u/.tt"
	newDir := "/home/u/icloud/tt"

	cases := map[string]string{
		"empty":   `{"data_dir": "", "projects": []}`,
		"missing": `{"projects": []}`,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := RewriteMigratedConfig([]byte(in), legacy, newDir)
			if err != nil {
				t.Fatalf("RewriteMigratedConfig: %v", err)
			}
			var parsed struct {
				DataDir string `json:"data_dir"`
			}
			if err := json.Unmarshal(out, &parsed); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if parsed.DataDir != newDir {
				t.Errorf("data_dir = %q, want %q", parsed.DataDir, newDir)
			}
		})
	}
}

func TestWriteMigratedConfig_AtomicRewriteInPlace(t *testing.T) {
	legacy := "/home/u/.tt"
	newDir := "/home/u/icloud/tt"

	dir := t.TempDir()
	path := filepath.Join(dir, ".tt.json")
	original := `{"data_dir":"/home/u/.tt","projects":[{"id":"p1","path":"/home/u/.tt/projects/alpha"}]}`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := WriteMigratedConfig(path, legacy, newDir); err != nil {
		t.Fatalf("WriteMigratedConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed struct {
		DataDir  string `json:"data_dir"`
		Projects []struct {
			Path string `json:"path"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.DataDir != newDir {
		t.Errorf("data_dir = %q, want %q", parsed.DataDir, newDir)
	}
	if len(parsed.Projects) != 1 || parsed.Projects[0].Path != filepath.Join(newDir, "projects", "alpha") {
		t.Errorf("project path not rewritten: %+v", parsed.Projects)
	}
}
