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

// writeLegacyConfig writes a legacy ~/.tt/.tt.json fixture with one project so
// first-run migration tests exercise both data_dir and project-path rewriting.
func writeLegacyConfig(t *testing.T, dataDir string, extraFiles map[string]string) {
	t.Helper()
	cfg := map[string]any{
		"version":  CurrentConfigVersion,
		"data_dir": dataDir,
		"projects": []map[string]any{
			{
				"id":   "p1",
				"slug": "alpha",
				"name": "Alpha",
				"path": filepath.Join(dataDir, "projects", "alpha"),
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy config: %v", err)
	}
	files := map[string]string{
		configFileName:            string(data),
		"projects/alpha/tasks.md": "# Alpha",
	}
	for k, v := range extraFiles {
		files[k] = v
	}
	writeFileTree(t, dataDir, files)
}

func TestLoad_FirstRunLegacyStaysLocalNoMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	local := LocalDataRoot()
	writeLegacyConfig(t, local, nil)

	// iCloud Drive is available but holds no tt workspace: local-first keeps the
	// existing local install in place — automatic migration has been removed.
	if got := ActiveDefaultDataDir(); got != local {
		t.Fatalf("ActiveDefaultDataDir() = %q, want local %q (no iCloud workspace exists)", got, local)
	}

	cfg, err := NewManager("").Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataDir != local {
		t.Errorf("data_dir = %q, want local %q", cfg.DataDir, local)
	}

	icloudRoot := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
	if _, err := os.Stat(filepath.Join(icloudRoot, configFileName)); !os.IsNotExist(err) {
		t.Errorf("no iCloud config should be created (migration removed), stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(icloudRoot, "projects", "alpha")); !os.IsNotExist(err) {
		t.Errorf("legacy data must not be copied into iCloud, stat err=%v", err)
	}
	// Legacy install remains canonical, in place.
	if len(cfg.Projects) != 1 {
		t.Fatalf("want 1 project, got %d", len(cfg.Projects))
	}
	if want := filepath.Join(local, "projects", "alpha"); cfg.Projects[0].Path != want {
		t.Errorf("project path = %q, want %q", cfg.Projects[0].Path, want)
	}
}

func TestLoad_ExistingICloudWorkspaceIsPreserved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	local := LocalDataRoot()
	writeLegacyConfig(t, local, nil) // legacy has project "alpha"

	icloudRoot := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
	// Pre-existing iCloud workspace with a different project.
	icloud := map[string]any{
		"version":  CurrentConfigVersion,
		"data_dir": icloudRoot,
		"projects": []map[string]any{
			{"id": "p2", "slug": "beta", "name": "Beta", "path": filepath.Join(icloudRoot, "projects", "beta")},
		},
	}
	icloudData, err := json.MarshalIndent(icloud, "", "  ")
	if err != nil {
		t.Fatalf("marshal iCloud config: %v", err)
	}
	writeFileTree(t, icloudRoot, map[string]string{configFileName: string(icloudData)})

	// The existing iCloud workspace stays selected even though a local install
	// also exists.
	if got := ActiveDefaultDataDir(); got != icloudRoot {
		t.Fatalf("ActiveDefaultDataDir() = %q, want existing iCloud %q", got, icloudRoot)
	}

	cfg, err := NewManager("").Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Projects) != 1 || cfg.Projects[0].ID != "p2" {
		t.Fatalf("expected only iCloud project p2, got %+v", cfg.Projects)
	}
	// The local install was not merged into iCloud.
	if _, err := os.Stat(filepath.Join(icloudRoot, "projects", "alpha")); !os.IsNotExist(err) {
		t.Errorf("local project must not be merged into iCloud, stat err=%v", err)
	}
}

func TestLoad_CustomConfigPathIsIsolated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	makeICloudDrive(t, home)

	writeLegacyConfig(t, LocalDataRoot(), nil)

	custom := filepath.Join(t.TempDir(), "custom", "config.json")
	if _, err := NewManager(custom).Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// A custom-path workspace is isolated: no iCloud workspace is created or
	// touched.
	icloudRoot := filepath.Join(home, iCloudDriveRelPath, iCloudDataDirName)
	if _, err := os.Stat(filepath.Join(icloudRoot, configFileName)); !os.IsNotExist(err) {
		t.Errorf("custom-path manager must not touch iCloud, stat err=%v", err)
	}
}

func TestLoad_ICloudUnavailableUsesLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No iCloud drive dir created → default is the local root.

	local := LocalDataRoot()
	writeLegacyConfig(t, local, nil)

	if got := ActiveDefaultDataDir(); got != local {
		t.Fatalf("test precondition: default = %q, want local %q", got, local)
	}

	cfg, err := NewManager("").Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DataDir != local {
		t.Errorf("data_dir = %q, want local %q", cfg.DataDir, local)
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
