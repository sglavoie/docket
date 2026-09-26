package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tterrors"
)

// resetProjectExportFlags restores the export flag globals after a test.
func resetProjectExportFlags(t *testing.T) {
	t.Helper()
	file, all, dir := projectExportFile, projectExportAll, projectExportDir
	t.Cleanup(func() { projectExportFile, projectExportAll, projectExportDir = file, all, dir })
	projectExportFile, projectExportAll, projectExportDir = "", false, ""
}

// newExportFixtureWorkspace bootstraps a workspace holding an active project
// "alpha" and an archived project "old", each with one task on disk.
func newExportFixtureWorkspace(t *testing.T) *CLIWorkspace {
	t.Helper()
	root := t.TempDir()
	setCommandDataDir(t, root)
	t.Setenv("HOME", t.TempDir())
	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("LoadCLIWorkspace: %v", err)
	}
	catalog := *workspace.Runtime.Catalog()
	catalog.Projects = append([]config.CatalogProject(nil), catalog.Projects...)
	now := time.Now().UTC()
	for _, spec := range []struct {
		id, slug, name, task string
		archived             bool
	}{
		{"p-alpha", "alpha", "Alpha", "Alpha task", false},
		{"p-old", "old", "Old Work", "Old task", true},
	} {
		entry := config.CatalogProject{ID: spec.id, Slug: spec.slug, Name: spec.name, CreatedAt: now, UpdatedAt: now, Archived: spec.archived}
		if spec.archived {
			entry.ArchivedAt = &now
		}
		catalog.Projects = append(catalog.Projects, entry)
		project := entry.ToModel(root)
		task := testutil.NewTestTask(spec.task)
		task.ShortID = 1
		project.Tasks = []*models.Task{task}
		project.NextShortID = 2
		if saveErr := storage.SaveProject(project); saveErr != nil {
			t.Fatalf("seed %s: %v", spec.slug, saveErr)
		}
	}
	if err = workspace.Runtime.CommitCatalog(&catalog); err != nil {
		t.Fatalf("commit catalog: %v", err)
	}
	workspace, err = LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	return workspace
}

func TestRunProjectExportAllWritesEveryProjectIncludingArchived(t *testing.T) {
	workspace := newExportFixtureWorkspace(t)
	resetProjectExportFlags(t)
	dir := filepath.Join(t.TempDir(), "out")
	projectExportAll = true
	projectExportDir = dir

	stdout, _, err := captureDoctorOutput(t, func() error { return runProjectExport(nil, nil) })
	if err != nil {
		t.Fatalf("export --all: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat export dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("export dir mode = %o, want 700", perm)
	}
	for _, want := range []struct{ slug, text string }{
		{"alpha", "Alpha task"},
		{"old", "Old task"},
		{config.SystemUnfiledSlug, "# Project: "},
	} {
		data, readErr := os.ReadFile(filepath.Join(dir, want.slug+".md"))
		if readErr != nil {
			t.Fatalf("read %s export: %v", want.slug, readErr)
		}
		if !strings.Contains(string(data), want.text) {
			t.Errorf("%s export lacks %q:\n%s", want.slug, want.text, data)
		}
	}
	if got := len(workspace.Runtime.AllProjects()); strings.Count(stdout, "Exported ") != got {
		t.Errorf("stdout names %d exports, want %d:\n%s", strings.Count(stdout, "Exported "), got, stdout)
	}
}

func TestRunProjectExportAllDefaultsToExportsDir(t *testing.T) {
	workspace := newExportFixtureWorkspace(t)
	resetProjectExportFlags(t)
	projectExportAll = true

	if _, _, err := captureDoctorOutput(t, func() error { return runProjectExport(nil, nil) }); err != nil {
		t.Fatalf("export --all: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace.Manager.ExportsDir(), "old.md")); err != nil {
		t.Fatalf("archived project missing from default exports dir: %v", err)
	}
}

func TestRunProjectExportFileDashWritesToStdout(t *testing.T) {
	workspace := newExportFixtureWorkspace(t)
	resetProjectExportFlags(t)
	projectExportFile = "-"

	stdout, _, err := captureDoctorOutput(t, func() error { return runProjectExport(nil, []string{"alpha"}) })
	if err != nil {
		t.Fatalf("export --file -: %v", err)
	}
	if !strings.HasPrefix(stdout, "# Project: Alpha\n") || !strings.Contains(stdout, "Alpha task") {
		t.Errorf("stdout is not the Alpha export:\n%s", stdout)
	}
	if strings.Contains(stdout, "Exported ") {
		t.Errorf("stdout export carries a status line:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(workspace.Manager.ExportsDir(), "-")); !os.IsNotExist(err) {
		t.Errorf("--file - wrote a file named -: %v", err)
	}
}

func TestExportProjectMarkdownToMatchesTheFileExport(t *testing.T) {
	workspace := newExportFixtureWorkspace(t)
	project := workspace.Runtime.GetProject("alpha")
	var buf bytes.Buffer
	if err := storage.ExportProjectMarkdownTo(&buf, project); err != nil {
		t.Fatalf("ExportProjectMarkdownTo: %v", err)
	}
	path := filepath.Join(t.TempDir(), "alpha.md")
	if _, err := storage.ExportProjectMarkdown(workspace.Runtime.GetProject("alpha"), path); err != nil {
		t.Fatalf("ExportProjectMarkdown: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if buf.String() != string(data) {
		t.Errorf("writer export differs from file export:\n%s\n---\n%s", buf.String(), data)
	}
}

func TestRunProjectExportRefusesConflictingFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		all  bool
		file string
		dir  string
		args []string
		want string
	}{
		{"all with slug", true, "", "", []string{"alpha"}, "do not also name one"},
		{"all with file", true, "/tmp/x.md", "", nil, "use --dir, not --file"},
		{"all with stdout", true, "-", "", nil, "use --dir, not --file"},
		{"dir without all", false, "", "/tmp/out", []string{"alpha"}, "--dir only applies with --all"},
		{"no slug", false, "", "", nil, "project slug required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetProjectExportFlags(t)
			projectExportAll, projectExportFile, projectExportDir = tc.all, tc.file, tc.dir
			err := runProjectExport(nil, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestRunProjectExportUnknownProjectNamesTheNextStep(t *testing.T) {
	newExportFixtureWorkspace(t)
	resetProjectExportFlags(t)

	err := runProjectExport(nil, []string{"nope"})
	if !errors.Is(err, tterrors.ErrProjectNotFound) {
		t.Fatalf("error = %v, want ErrProjectNotFound", err)
	}
	for _, want := range []string{`project not found: "nope"`, "slug or ID", "--all"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestRunProjectExportPrintsTheDestinationAsADisplayPath(t *testing.T) {
	newExportFixtureWorkspace(t)
	resetProjectExportFlags(t)
	projectExportFile = filepath.Join(os.Getenv("HOME"), "alpha.md")

	stdout, _, err := captureDoctorOutput(t, func() error { return runProjectExport(nil, []string{"alpha"}) })
	if err != nil {
		t.Fatalf("export alpha: %v", err)
	}
	want := `Exported "Alpha" to ~` + string(filepath.Separator) + "alpha.md"
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, want)
	}
}
