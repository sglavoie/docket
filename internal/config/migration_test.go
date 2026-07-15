package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeFileTree writes files described by a rel-path -> contents map, creating
// parent directories as needed, so copy tests can build a legacy data dir fast.
func writeFileTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, contents := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(contents), 0644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

func TestMigrateCopyTree_PreservesDurableDataExcludesRuntime(t *testing.T) {
	src := filepath.Join(t.TempDir(), "legacy")
	dst := filepath.Join(t.TempDir(), "icloud")

	writeFileTree(t, src, map[string]string{
		".tt.json":                                         `{"version":1}`,
		"projects/alpha/tasks.md":                          "# Alpha",
		"projects/beta/tasks.md":                           "# Beta",
		"themes/custom.json":                               `{"name":"custom"}`,
		"exports/report.md":                                "# Export",
		"internal/.tt/state":                               "internal-data",
		"logs/tt.log":                                      "log line",
		"pid.lock":                                         "12345",
		"machine.json":                                     `{"kind":"machine"}`,
		"runtime/abc123/pid.lock":                          "999",
		".runtime/scratch":                                 "scratch",
		"projects/alpha/attachments/pic.txt":               "attachment",
		"projects/logs/tasks.md":                           "# Logs Project",
		"projects/runtime/tasks.md":                        "# Runtime Project",
		"projects/.runtime/tasks.md":                       "# Dot Runtime Project",
		"projects/pid.lock/tasks.md":                       "# PID Lock Project",
		"projects/alpha/attachments/logs/evidence.txt":     "durable log attachment",
		"projects/alpha/attachments/runtime/evidence.txt":  "durable runtime attachment",
		"projects/alpha/attachments/.runtime/evidence.txt": "durable dot-runtime attachment",
		"projects/alpha/attachments/pid.lock":              "durable pid attachment",
	})

	if err := MigrateCopyTree(src, dst); err != nil {
		t.Fatalf("MigrateCopyTree: %v", err)
	}

	// Durable data must be copied.
	mustExist := []string{
		".tt.json",
		"projects/alpha/tasks.md",
		"projects/beta/tasks.md",
		"themes/custom.json",
		"exports/report.md",
		"internal/.tt/state",
		"projects/alpha/attachments/pic.txt",
		"projects/logs/tasks.md",
		"projects/runtime/tasks.md",
		"projects/.runtime/tasks.md",
		"projects/pid.lock/tasks.md",
		"projects/alpha/attachments/logs/evidence.txt",
		"projects/alpha/attachments/runtime/evidence.txt",
		"projects/alpha/attachments/.runtime/evidence.txt",
		"projects/alpha/attachments/pid.lock",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("expected copied %s: %v", rel, err)
		}
	}

	// Runtime noise must be excluded.
	mustNotExist := []string{
		"logs",
		"logs/tt.log",
		"pid.lock",
		"machine.json",
		"runtime",
		"runtime/abc123/pid.lock",
		".runtime",
		".runtime/scratch",
	}
	for _, rel := range mustNotExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be excluded, stat err=%v", rel, err)
		}
	}

	// Copied contents must match, including durable project data whose basename
	// matches a top-level runtime exclusion.
	wantContents := map[string]string{
		"projects/alpha/tasks.md":                         "# Alpha",
		"projects/logs/tasks.md":                          "# Logs Project",
		"projects/alpha/attachments/logs/evidence.txt":    "durable log attachment",
		"projects/alpha/attachments/runtime/evidence.txt": "durable runtime attachment",
		"projects/alpha/attachments/pid.lock":             "durable pid attachment",
	}
	for rel, want := range wantContents {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatalf("read copied file %s: %v", rel, err)
		}
		if string(got) != want {
			t.Errorf("copied contents for %s = %q, want %q", rel, got, want)
		}
	}
}

func TestMigrateCopyTree_RefusesConflicts(t *testing.T) {
	src := filepath.Join(t.TempDir(), "legacy")
	dst := filepath.Join(t.TempDir(), "icloud")

	writeFileTree(t, src, map[string]string{
		"projects/alpha/tasks.md": "# Alpha src",
		"themes/custom.json":      `{"name":"custom"}`,
	})
	// Pre-existing destination file that would be overwritten.
	writeFileTree(t, dst, map[string]string{
		"projects/alpha/tasks.md": "# Alpha existing",
	})

	err := MigrateCopyTree(src, dst)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}

	// Conflict must abort before copying anything: the other file must not
	// have been written, and the existing file must be untouched.
	if _, statErr := os.Stat(filepath.Join(dst, "themes/custom.json")); !os.IsNotExist(statErr) {
		t.Errorf("expected no partial copy of themes/custom.json, stat err=%v", statErr)
	}
	got, readErr := os.ReadFile(filepath.Join(dst, "projects/alpha/tasks.md"))
	if readErr != nil {
		t.Fatalf("read existing file: %v", readErr)
	}
	if string(got) != "# Alpha existing" {
		t.Errorf("existing file was overwritten: got %q", got)
	}
}

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
