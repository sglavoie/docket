package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestCheckMissingTaskFiles(t *testing.T) {
	// Project with a nonexistent path → should report missing task file
	project := testutil.NewTestProject("Ghost")
	project.Path = t.TempDir() // exists but no tasks.md inside

	results := &doctorResults{}
	checkMissingTaskFiles([]*models.Project{project}, results)

	if len(results.missingTaskFiles) != 1 {
		t.Fatalf("expected 1 missing task file, got %d", len(results.missingTaskFiles))
	}
	if results.missingTaskFiles[0].project != "Ghost" {
		t.Errorf("expected project 'Ghost', got %q", results.missingTaskFiles[0].project)
	}
}

func TestCheckEmptyProjects(t *testing.T) {
	empty := testutil.NewTestProject("Empty")
	nonEmpty := testutil.NewTestProjectWithTasks("Has Tasks")

	results := &doctorResults{}
	checkEmptyProjects([]*models.Project{empty, nonEmpty}, results)

	if len(results.emptyProjects) != 1 {
		t.Fatalf("expected 1 empty project, got %d", len(results.emptyProjects))
	}
	if results.emptyProjects[0].project != "Empty" {
		t.Errorf("expected project 'Empty', got %q", results.emptyProjects[0].project)
	}
}

func TestCheckDuplicateIDs(t *testing.T) {
	t.Run("duplicate within project", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task2.ID = task1.ID // force duplicate
		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{project}, results)

		if len(results.duplicateIDs) == 0 {
			t.Fatal("expected at least 1 duplicate ID issue, got 0")
		}
	})

	t.Run("duplicate across projects", func(t *testing.T) {
		p1 := testutil.NewTestProject("P1")
		p2 := testutil.NewTestProject("P2")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task2.ID = task1.ID // force same ID across projects
		p1.Tasks = []*models.Task{task1}
		p2.Tasks = []*models.Task{task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{p1, p2}, results)

		if len(results.duplicateIDs) != 1 {
			t.Fatalf("expected 1 cross-project duplicate, got %d", len(results.duplicateIDs))
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateIDs([]*models.Project{project}, results)

		if len(results.duplicateIDs) != 0 {
			t.Fatalf("expected 0 duplicate ID issues, got %d", len(results.duplicateIDs))
		}
	})
}

func TestCheckDuplicateShortIDs(t *testing.T) {
	t.Run("duplicate within project", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task1.ShortID = 5
		task2.ShortID = 5 // force duplicate

		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateShortIDs([]*models.Project{project}, results)

		if len(results.duplicateShortIDs) != 1 {
			t.Fatalf("expected 1 duplicate short ID issue, got %d", len(results.duplicateShortIDs))
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		project := testutil.NewTestProject("P1")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task1.ShortID = 1
		task2.ShortID = 2

		project.Tasks = []*models.Task{task1, task2}

		results := &doctorResults{}
		checkDuplicateShortIDs([]*models.Project{project}, results)

		if len(results.duplicateShortIDs) != 0 {
			t.Fatalf("expected 0 duplicate short ID issues, got %d", len(results.duplicateShortIDs))
		}
	})

	t.Run("duplicates across projects not flagged", func(t *testing.T) {
		p1 := testutil.NewTestProject("P1")
		p2 := testutil.NewTestProject("P2")
		task1 := testutil.NewTestTask("Task A")
		task2 := testutil.NewTestTask("Task B")
		task1.ShortID = 1
		task2.ShortID = 1 // same short ID but different projects — OK

		p1.Tasks = []*models.Task{task1}
		p2.Tasks = []*models.Task{task2}

		results := &doctorResults{}
		checkDuplicateShortIDs([]*models.Project{p1, p2}, results)

		if len(results.duplicateShortIDs) != 0 {
			t.Fatalf("expected 0 issues (different projects), got %d", len(results.duplicateShortIDs))
		}
	})
}

func TestCheckBrokenParentRefs(t *testing.T) {
	project := testutil.NewTestProject("P1")
	parent := testutil.NewTestTask("Parent")
	child := testutil.NewTestTask("Child")
	orphan := testutil.NewTestTask("Orphan")

	child.ParentID = &parent.ID
	badID := "nonexistent-uuid"
	orphan.ParentID = &badID

	project.Tasks = []*models.Task{parent, child, orphan}

	results := &doctorResults{}
	checkBrokenParentRefs([]*models.Project{project}, results)

	if len(results.brokenParentRefs) != 1 {
		t.Fatalf("expected 1 broken parent ref, got %d", len(results.brokenParentRefs))
	}
	if !strings.Contains(results.brokenParentRefs[0].detail, orphan.Title) {
		t.Errorf("expected orphan detail, got %q", results.brokenParentRefs[0].detail)
	}
}

func TestCheckCompletionTimestamps(t *testing.T) {
	validCompleted := time.Now().AddDate(0, 0, -30)
	zero := time.Time{}
	project := testutil.NewTestProject("History")
	valid := testutil.NewTestTask("Valid terminal")
	valid.WorkflowState = models.StateDone
	valid.CompletedAt = &validCompleted
	missing := testutil.NewTestTask("Missing terminal timestamp")
	missing.WorkflowState = models.StateCanceled
	missing.CompletedAt = nil
	zeroTerminal := testutil.NewTestTask("Zero terminal timestamp")
	zeroTerminal.WorkflowState = models.StateDone
	zeroTerminal.CompletedAt = &zero
	stale := testutil.NewTestTask("Stale nonterminal timestamp")
	stale.WorkflowState = models.StateBacklog
	stale.CompletedAt = &validCompleted
	project.Tasks = []*models.Task{valid, missing, zeroTerminal, stale}

	results := &doctorResults{}
	checkCompletionTimestamps([]*models.Project{project}, results)

	if got := len(results.invalidCompletionTimestamps); got != 3 {
		t.Fatalf("invalid completion timestamps = %d, want 3: %+v", got, results.invalidCompletionTimestamps)
	}
}

func TestTotalIssues(t *testing.T) {
	results := &doctorResults{
		duplicateIDs:  []doctorIssue{{project: "p"}, {project: "p"}},
		emptyProjects: []doctorIssue{{project: "p"}, {project: "p"}, {project: "p"}},
	}

	// Empty projects are informational and not live schema-6 issues.
	if got := results.totalIssues(); got != 2 {
		t.Errorf("totalIssues() = %d, want 2", got)
	}
}

func TestFixIssues(t *testing.T) {
	deps, project := setupTestDepsWithProject(t, "FixTest")
	defer deps.Cleanup()

	task1 := testutil.NewTestTask("Good Task")
	task2 := testutil.NewTestTask("Broken Task")
	badParent := "nonexistent-parent"
	task2.ParentID = &badParent

	project.Tasks = []*models.Task{task1, task2}

	// Save initial state so fixIssues can persist
	deps.Repository.Projects[project.ID] = project

	fixIssues([]*models.Project{project})

	// Verify broken parent ref was cleared
	if task2.ParentID != nil {
		t.Errorf("expected ParentID to be cleared, got %v", task2.ParentID)
	}

}

func captureDoctorOutput(t *testing.T, fn func() error) (stdout string, stderr string, runErr error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	runErr = fn()
	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)

	return stdoutBuf.String(), stderrBuf.String(), runErr
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestRunDoctorFreshWorkspaceBootstrapsProtectedUnfiledIdempotently(t *testing.T) {
	dataDir := t.TempDir()
	setCommandDataDir(t, dataDir)

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })

	first, err := LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("first LoadCLIWorkspace: %v", err)
	}
	unfiled := first.Runtime.SystemUnfiledProject()
	if unfiled == nil {
		t.Fatal("fresh workspace has no protected system Unfiled")
	}
	if unfiled.ID != first.Runtime.Catalog().SystemUnfiledProjectID ||
		unfiled.Name != config.SystemUnfiledName ||
		unfiled.Slug != config.SystemUnfiledSlug {
		t.Fatalf("fresh protected Unfiled = %+v, catalog id = %q", unfiled, first.Runtime.Catalog().SystemUnfiledProjectID)
	}
	taskPath := storage.TasksFilePath(unfiled.Path)
	before, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read fresh Unfiled task document: %v", err)
	}
	document, err := storage.ParseProjectDocument(string(before))
	if err != nil {
		t.Fatalf("parse fresh Unfiled task document: %v", err)
	}
	if len(document.Tasks) != 0 {
		t.Fatalf("fresh Unfiled has %d tasks, want 0", len(document.Tasks))
	}

	second, err := LoadCLIWorkspace()
	if err != nil {
		t.Fatalf("second LoadCLIWorkspace: %v", err)
	}
	if got := second.Runtime.Catalog().SystemUnfiledProjectID; got != unfiled.ID {
		t.Fatalf("repeat bootstrap changed Unfiled id from %q to %q", unfiled.ID, got)
	}
	after, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read Unfiled task document after repeat load: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("repeat workspace load rewrote the canonical Unfiled task document")
	}

	command := &cobra.Command{}
	command.Flags().Bool("fix", false, "")
	stdout, stderr, runErr := captureDoctorOutput(t, func() error {
		return runDoctor(command, nil)
	})
	if runErr != nil {
		t.Fatalf("doctor on fresh workspace: %v\n%s", runErr, stdout)
	}
	if stderr != "" {
		t.Fatalf("doctor stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Workspace integrity issues: 0") ||
		!strings.Contains(stdout, "[Migration] workspace schema 13 is current") ||
		!strings.Contains(stdout, "[Undo] workspace ledger is valid (0 undo entries, 0 redo entries)") ||
		!strings.Contains(stdout, "[Migration] clean schema-6 workspace; no pre-v6 snapshot recorded") ||
		!strings.Contains(stdout, "[Migration] clean schema-7 workspace; no pre-v7 snapshot recorded") ||
		!strings.Contains(stdout, "All checks passed") {
		t.Fatalf("doctor did not report a healthy fresh workspace:\n%s", stdout)
	}
	if _, err := os.Lstat(config.PreSimplificationV5BackupsDir(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("doctor created or changed the migration backups directory: %v", err)
	}
}

func TestCheckProtectedUnfiledReportsMissingAndInvalidIdentity(t *testing.T) {
	results := &doctorResults{}
	checkProtectedUnfiled(config.NewWorkspaceCatalog(), results)
	if len(results.workspaceIssues) != 1 || !strings.Contains(results.workspaceIssues[0].detail, "ID is missing") {
		t.Fatalf("missing Unfiled issues = %+v", results.workspaceIssues)
	}

	catalog := config.NewWorkspaceCatalog()
	project := testutil.NewTestProject("Wrong Name")
	project.Slug = "wrong-name"
	if err := catalog.UpsertProject(project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	catalog.SystemUnfiledProjectID = project.ID
	results = &doctorResults{}
	checkProtectedUnfiled(catalog, results)
	if len(results.workspaceIssues) != 1 || !strings.Contains(results.workspaceIssues[0].detail, "want \"Unfiled\"/\"unfiled\"") {
		t.Fatalf("invalid Unfiled identity issues = %+v", results.workspaceIssues)
	}
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestCheckMigrationSnapshotsReportsIncompletePairWithoutMutation(t *testing.T) {
	dataDir := t.TempDir()
	backupsDir := config.PreSimplificationV5BackupsDir(dataDir)
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}
	archivePath := filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.tar.gz")
	want := []byte("immutable archive")
	if err := os.WriteFile(archivePath, want, 0o400); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	results := &doctorResults{}
	checkMigrationSnapshots(dataDir, config.V5ConfigSchemaVersion, results)
	if len(results.workspaceIssues) != 1 ||
		!strings.Contains(results.workspaceIssues[0].detail, "missing its audit report") {
		t.Fatalf("snapshot issues = %+v", results.workspaceIssues)
	}
	got, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive after doctor check: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("snapshot check changed the rollback archive")
	}
	auditPath := filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.audit.md")
	if _, err := os.Lstat(auditPath); !os.IsNotExist(err) {
		t.Fatalf("snapshot check recreated a missing audit report: %v", err)
	}
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestCheckWorkspaceUndoLedgerReportsCorruption(t *testing.T) {
	dataDir := t.TempDir()
	undoDir := config.UndoLedgerDir(dataDir)
	if err := os.MkdirAll(undoDir, 0o700); err != nil {
		t.Fatalf("create undo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(undoDir, "ledger.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt ledger: %v", err)
	}

	results := &doctorResults{}
	checkWorkspaceUndoLedger(dataDir, results)
	if len(results.workspaceIssues) != 1 ||
		!strings.Contains(results.workspaceIssues[0].detail, "failed to parse undo ledger") {
		t.Fatalf("undo issues = %+v", results.workspaceIssues)
	}
}

//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func TestCheckRetainedV5DomainsDoesNotCreateRetiredFiles(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(config.InternalDir(dataDir), 0o700); err != nil {
		t.Fatalf("create internal dir: %v", err)
	}
	for _, path := range []string{
		config.WorkspaceCatalogPath(dataDir),
		config.PlanningStatePath(dataDir),
	} {
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatalf("write retained domain: %v", err)
		}
	}

	results := &doctorResults{}
	checkRetainedDomainArtifacts(dataDir, config.V5ConfigSchemaVersion, results)
	if len(results.workspaceIssues) != 0 {
		t.Fatalf("retained v5 domain issues = %+v", results.workspaceIssues)
	}
	for _, path := range []string{config.SettingsPath(dataDir), config.AutomationPath(dataDir)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("domain check recreated retired file %s: %v", path, err)
		}
	}
}
