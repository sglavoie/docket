package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// corruptDoctorFixture rewrites the seeded tasks.md bytes so Beta repeats
// Alpha's short ID and names a parent that does not exist. The strict reader
// refuses both, so this is the file doctor --fix has to reach from disk.
func corruptDoctorFixture(document string, alpha *models.Task) string {
	document = strings.Replace(document, "short_id: 2,", "short_id: 1,", 1)
	return strings.Replace(document, "parent: "+alpha.ID, "parent: nonexistent-parent", 1)
}

// repairLoadedUnfiled returns Unfiled loaded the way doctor loads it, with the
// defects on disk kept in the model.
func repairLoadedUnfiled(t *testing.T, workspace *CLIWorkspace) *models.Project {
	t.Helper()
	unfiled := workspace.Runtime.SystemUnfiledProject()
	if _, err := storage.LoadProjectForRepair(unfiled); err != nil {
		t.Fatalf("repair-load Unfiled: %v", err)
	}
	return unfiled
}

func TestFixIssues(t *testing.T) {
	root, workspace := newDoctorFixtureWorkspace(t, corruptDoctorFixture)
	unfiled := workspace.Runtime.SystemUnfiledProject()
	if err := storage.LoadProject(unfiled); err == nil {
		t.Fatal("the strict reader accepted the corrupted fixture")
	}
	unfiled = repairLoadedUnfiled(t, workspace)

	fixed, err := fixIssues(root, []*models.Project{unfiled})
	if err != nil {
		t.Fatalf("fixIssues: %v", err)
	}
	if len(fixed) != 2 ||
		!strings.Contains(fixed[0], "reassigned duplicate short ID #1") ||
		!strings.Contains(fixed[1], "cleared broken parent ref nonexistent-parent") {
		t.Fatalf("fixed = %q", fixed)
	}

	// The repair is on disk, not only in memory, and the strict reader accepts it.
	reloaded := &models.Project{ID: unfiled.ID, Name: unfiled.Name, Slug: unfiled.Slug, Path: unfiled.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload repaired project: %v", err)
	}
	if len(reloaded.Tasks) != 2 || reloaded.Tasks[0].ShortID == reloaded.Tasks[1].ShortID {
		t.Fatalf("reloaded tasks = %+v, want two distinct short IDs", reloaded.Tasks)
	}
	for _, task := range reloaded.Tasks {
		if task.ParentID != nil {
			t.Fatalf("task %q still has parent %q", task.Title, *task.ParentID)
		}
	}
}

func TestFixIssuesSkipsAProjectWithNothingToRepair(t *testing.T) {
	root, workspace := newDoctorFixtureWorkspace(t)
	unfiled := loadedUnfiled(t, workspace)
	oldLoad := loadProjectForRepair
	t.Cleanup(func() { loadProjectForRepair = oldLoad })
	loadProjectForRepair = func(*models.Project) (*string, error) {
		t.Fatal("a healthy project was reloaded for repair")
		return nil, nil
	}

	fixed, err := fixIssues(root, []*models.Project{unfiled})
	if err != nil || len(fixed) != 0 {
		t.Fatalf("fixIssues = %q, %v; want nothing", fixed, err)
	}
}

func TestFixIssuesRefusesAStaleRevision(t *testing.T) {
	root, workspace := newDoctorFixtureWorkspace(t, corruptDoctorFixture)
	unfiled := repairLoadedUnfiled(t, workspace)
	path := storage.TasksFilePath(unfiled.Path)
	oldLoad := loadProjectForRepair
	t.Cleanup(func() { loadProjectForRepair = oldLoad })
	loadProjectForRepair = func(project *models.Project) (*string, error) {
		rev, err := oldLoad(project)
		if err != nil {
			return nil, err
		}
		// Another process commits between the repair's read and its commit.
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read tasks.md: %v", err)
		}
		concurrent := strings.Replace(string(content), "## Task: Alpha", "## Task: Alpha elsewhere", 1)
		if err := os.WriteFile(path, []byte(concurrent), 0o600); err != nil {
			t.Fatalf("write concurrent change: %v", err)
		}
		return rev, nil
	}

	fixed, err := fixIssues(root, []*models.Project{unfiled})
	if !errors.Is(err, storage.ErrStaleArtifact) || !strings.Contains(err.Error(), "project Unfiled") {
		t.Fatalf("fixIssues error = %v, want a stale revision naming Unfiled", err)
	}
	if len(fixed) != 0 {
		t.Fatalf("fixed = %q, want nothing reported for a refused commit", fixed)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read tasks.md: %v", readErr)
	}
	if !strings.Contains(string(content), "Alpha elsewhere") {
		t.Fatal("the stale repair overwrote the concurrent change")
	}
}

// TestRunDoctorRepairsCorruptTaskBytesFromDisk drives the real diagnose, fix,
// and re-check path over a tasks.md whose bytes carry both repairable defects.
func TestRunDoctorRepairsCorruptTaskBytesFromDisk(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t, corruptDoctorFixture)
	path := storage.TasksFilePath(workspace.Runtime.SystemUnfiledProject().Path)
	corrupted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}

	stdout, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t), nil)
	})
	var issues *doctorIssuesError
	if !errors.As(err, &issues) || issues.ExitCode() != 2 {
		t.Fatalf("doctor error = %v, want an exit-2 issues error\n%s", err, stdout)
	}
	// An empty section is skipped in text mode, so a clean load prints no
	// "Project load failures" header at all.
	if strings.Contains(stdout, "Project load failures") ||
		!strings.Contains(stdout, "Duplicate short IDs: 1") ||
		!strings.Contains(stdout, "Broken parent refs: 1") ||
		!strings.Contains(stdout, "Run 'tt doctor --fix'") {
		t.Fatalf("doctor did not report the two repairable defects:\n%s", stdout)
	}
	if after, _ := os.ReadFile(path); string(after) != string(corrupted) {
		t.Fatal("a diagnostic run rewrote tasks.md")
	}

	stdout, _, err = captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	if err != nil {
		t.Fatalf("doctor --fix: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "Fixed: reassigned duplicate short ID #1") ||
		!strings.Contains(stdout, "Fixed: cleared broken parent ref nonexistent-parent") ||
		!strings.Contains(stdout, "All checks passed") {
		t.Fatalf("doctor --fix did not report the repairs and a clean state:\n%s", stdout)
	}
	repaired, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired tasks.md: %v", err)
	}
	if !strings.Contains(string(repaired), "Task: Alpha") || !strings.Contains(string(repaired), "Task: Beta") {
		t.Fatalf("the repair lost a task:\n%s", repaired)
	}
	entries, err := os.ReadDir(config.TransactionsDir(workspace.Runtime.DataDir()))
	if err != nil {
		t.Fatalf("read transactions dir: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			t.Fatalf("repair left journal residue %s", entry.Name())
		}
	}

	stdout, _, err = captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t), nil)
	})
	if err != nil {
		t.Fatalf("second doctor run: %v\n%s", err, stdout)
	}
	if after, _ := os.ReadFile(path); string(after) != string(repaired) {
		t.Fatal("the second diagnostic run rewrote tasks.md")
	}
}

// newDoctorFixtureWorkspace mints a filesystem workspace whose Unfiled project
// holds Alpha (#1) and its child Beta (#2), letting each edit rewrite the
// canonical tasks.md. It returns the data directory and a workspace loaded
// after those edits.
func newDoctorFixtureWorkspace(t *testing.T, edits ...func(document string, alpha *models.Task) string) (string, *CLIWorkspace) {
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
	unfiled := workspace.Runtime.SystemUnfiledProject()
	if loadErr := storage.LoadProject(unfiled); loadErr != nil {
		t.Fatalf("load Unfiled: %v", loadErr)
	}
	alpha := testutil.NewTestTask("Alpha")
	alpha.ShortID = 1
	beta := testutil.NewTestTask("Beta")
	beta.ShortID = 2
	parentID := alpha.ID
	beta.ParentID = &parentID
	unfiled.Tasks = []*models.Task{alpha, beta}
	if saveErr := storage.SaveProject(unfiled); saveErr != nil {
		t.Fatalf("seed Unfiled: %v", saveErr)
	}

	path := storage.TasksFilePath(unfiled.Path)
	for _, edit := range edits {
		document, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read seeded tasks.md: %v", readErr)
		}
		edited := edit(string(document), alpha)
		if edited == string(document) {
			t.Fatal("fixture edit changed nothing")
		}
		if writeErr := os.WriteFile(path, []byte(edited), 0o600); writeErr != nil {
			t.Fatalf("write edited tasks.md: %v", writeErr)
		}
	}

	workspace, err = LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("reload workspace: %v", err)
	}
	return root, workspace
}

// loadedUnfiled returns the workspace's Unfiled project with its tasks loaded.
func loadedUnfiled(t *testing.T, workspace *CLIWorkspace) *models.Project {
	t.Helper()
	unfiled := workspace.Runtime.SystemUnfiledProject()
	if err := storage.LoadProject(unfiled); err != nil {
		t.Fatalf("load Unfiled: %v", err)
	}
	return unfiled
}
