package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
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
	if !strings.Contains(stdout, "Project load failures: 0") ||
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

func newDoctorCommand(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	command := &cobra.Command{}
	command.Flags().Bool("fix", false, "")
	command.Flags().Bool("json", false, "")
	if err := command.Flags().Parse(args); err != nil {
		t.Fatalf("parse doctor flags %v: %v", args, err)
	}
	return command
}

// plantStaleTodayEntry commits a Today entry naming a task Unfiled does not
// own, the fixable skew two machines most often leave behind.
func plantStaleTodayEntry(t *testing.T, workspace *CLIWorkspace) {
	t.Helper()
	unfiled := workspace.Runtime.SystemUnfiledProject()
	planning := &config.PlanningState{Today: []config.ProjectTaskLocator{{ProjectID: unfiled.ID, TaskID: "task-gone"}}}
	if err := workspace.Runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("plant stale Today entry: %v", err)
	}
}

func TestRunDoctorFixRepairsThenReportsTheCleanState(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)

	stdout, stderr, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	if err != nil {
		t.Fatalf("doctor --fix: %v\n%s", err, stdout)
	}
	if stderr != "" {
		t.Fatalf("doctor --fix stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Fixed: pruned 1 planning entry from Today and Next") ||
		!strings.Contains(stdout, "Planning membership issues: 0") ||
		!strings.Contains(stdout, "All checks passed") {
		t.Fatalf("doctor --fix did not report the post-fix state:\n%s", stdout)
	}
	if strings.Contains(stdout, "Re-run") {
		t.Fatalf("doctor --fix still asks for a re-run:\n%s", stdout)
	}

	stdout, _, err = captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t), nil)
	})
	if err != nil {
		t.Fatalf("second doctor run: %v\n%s", err, stdout)
	}
}

func TestRunDoctorWithoutFixExitsTwoForAFixableIssue(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)

	stdout, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t), nil)
	})
	var issues *doctorIssuesError
	if !errors.As(err, &issues) || issues.ExitCode() != 2 {
		t.Fatalf("doctor error = %v, want an exit-2 issues error", err)
	}
	if !strings.Contains(stdout, "Run 'tt doctor --fix'") {
		t.Fatalf("doctor did not point at --fix:\n%s", stdout)
	}
}

func TestRunDoctorUnfixableIssueExitsTwo(t *testing.T) {
	newDoctorFixtureWorkspace(t, func(document string, alpha *models.Task) string {
		stamp := alpha.CreatedAt.Format(time.RFC3339Nano)
		return strings.Replace(document, "state: backlog,", "state: backlog, completed: "+stamp+",", 1)
	})

	stdout, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	var issues *doctorIssuesError
	if !errors.As(err, &issues) {
		t.Fatalf("doctor --fix error = %v, want doctorIssuesError\n%s", err, stdout)
	}
	if issues.ExitCode() != 2 || issues.count != 1 || err.Error() != "found 1 issue(s)" {
		t.Fatalf("issues error = %+v (%q), want exit 2 for one issue", issues, err.Error())
	}
	if !strings.Contains(stdout, "Invalid completion timestamps: 1") ||
		!strings.Contains(stdout, "Resolve the reported integrity issues") {
		t.Fatalf("doctor did not report the unfixable issue:\n%s", stdout)
	}
}

func TestRunDoctorFixFailureIsNotAnIssuesVerdict(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)
	unfiledID := workspace.Runtime.SystemUnfiledProject().ID
	apply := doctorApplyFixes
	t.Cleanup(func() { doctorApplyFixes = apply })
	doctorApplyFixes = func(state *doctorState) ([]string, error) {
		// Another process rewrites planning after doctor read it.
		other, err := LoadCLIWorkspace()
		if err != nil {
			t.Fatalf("load concurrent workspace: %v", err)
		}
		concurrent := &config.PlanningState{Next: []config.ProjectTaskLocator{{ProjectID: unfiledID, TaskID: "task-elsewhere"}}}
		if err := other.Runtime.CommitPlanning(concurrent); err != nil {
			t.Fatalf("concurrent planning commit: %v", err)
		}
		return apply(state)
	}

	_, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	var issues *doctorIssuesError
	if err == nil || errors.As(err, &issues) {
		t.Fatalf("doctor --fix error = %v, want a fix failure that exits 1", err)
	}
	if !strings.Contains(err.Error(), "could not apply every repair") ||
		!strings.Contains(err.Error(), "repair planning membership") {
		t.Fatalf("doctor --fix error = %v, want it to name the failed planning repair", err)
	}
}

func TestDoctorJSONPrintsOnlyAParseableReport(t *testing.T) {
	root, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)
	home := os.Getenv("HOME")

	// Anything logged while doctor runs, as BuildTaskTree's warnings are, must
	// reach the log file the root pre-run opened and never the standard logger.
	var leaked bytes.Buffer
	log.SetOutput(&leaked)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	runE := doctorCmd.RunE
	doctorCmd.RunE = func(cmd *cobra.Command, args []string) error {
		logging.Warn("doctor json probe")
		return runE(cmd, args)
	}

	rootCmd.SetArgs([]string{"doctor", "--json", "--data-dir", root})
	t.Cleanup(func() {
		doctorCmd.RunE = runE
		rootCmd.SetArgs(nil)
		_ = doctorCmd.Flags().Set("json", "false")
	})
	stdout, stderr, err := captureDoctorOutput(t, func() error {
		return executeCommand(rootCmd)
	})

	var issues *doctorIssuesError
	if !errors.As(err, &issues) || issues.ExitCode() != 2 || err.Error() != "" {
		t.Fatalf("doctor --json error = %v, want a silent exit-2 issues error", err)
	}
	if stderr != "" || leaked.Len() != 0 {
		t.Fatalf("stderr = %q, standard logger = %q; want both empty", stderr, leaked.String())
	}
	var report doctorReport
	if decodeErr := json.Unmarshal([]byte(stdout), &report); decodeErr != nil {
		t.Fatalf("stdout is not one JSON report: %v\n%s", decodeErr, stdout)
	}
	if report.Schema != doctorReportSchema || report.TotalIssues != 1 ||
		len(report.Buckets["planning_membership"]) != 1 ||
		report.Buckets["duplicate_short_ids"] == nil ||
		report.Notes["workspace_status"] == nil ||
		report.Fixed == nil {
		t.Fatalf("report = %+v", report)
	}

	logs, globErr := filepath.Glob(filepath.Join(home, ".tt", "logs", "tt-*.log"))
	if globErr != nil || len(logs) != 1 {
		t.Fatalf("log files = %v (%v), want exactly one", logs, globErr)
	}
	content, readErr := os.ReadFile(logs[0])
	if readErr != nil || !strings.Contains(string(content), "doctor json probe") {
		t.Fatalf("log file = %q (%v), want the probe warning", content, readErr)
	}
}

func TestDoctorJSONFixCarriesFixedLinesInTheReport(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)

	stdout, stderr, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix", "--json"), nil)
	})
	if err != nil || stderr != "" {
		t.Fatalf("doctor --fix --json = %v, stderr %q", err, stderr)
	}
	var report doctorReport
	if decodeErr := json.Unmarshal([]byte(stdout), &report); decodeErr != nil {
		t.Fatalf("stdout is not one JSON report: %v\n%s", decodeErr, stdout)
	}
	if report.TotalIssues != 0 || len(report.Fixed) != 1 ||
		report.Fixed[0] != "pruned 1 planning entry from Today and Next" {
		t.Fatalf("report = %+v", report)
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
	command.Flags().Bool("json", false, "")
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
		!strings.Contains(stdout, fmt.Sprintf("[Migration] workspace schema %d is current", config.CurrentConfigVersion)) ||
		!strings.Contains(stdout, "[Undo] workspace ledger is valid (0 undo entries, 0 redo entries)") ||
		!strings.Contains(stdout, "All checks passed") {
		t.Fatalf("doctor did not report a healthy fresh workspace:\n%s", stdout)
	}
	// A workspace created at the current schema crossed no boundary, so it has no
	// missing early snapshot to explain.
	if strings.Contains(stdout, "clean schema-") {
		t.Fatalf("doctor narrated migration history a fresh workspace never had:\n%s", stdout)
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

// TestCheckMigrationSnapshotsExplainsMissingEarlySnapshotsOnlyAfterMigrating
// keeps the "clean schema-N" notes for the workspace they describe: one whose
// history shows a migration, so a missing early snapshot needs explaining.
func TestCheckMigrationSnapshotsExplainsMissingEarlySnapshotsOnlyAfterMigrating(t *testing.T) {
	fresh := &doctorResults{}
	checkMigrationSnapshots(t.TempDir(), config.CurrentConfigVersion, fresh)
	for _, note := range fresh.workspaceNotes {
		if strings.Contains(note.detail, "clean schema-") {
			t.Fatalf("never-migrated workspace notes = %+v", fresh.workspaceNotes)
		}
	}

	migratedDir := t.TempDir()
	backupsDir := config.PreSimplificationV5BackupsDir(migratedDir)
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.tar.gz"), []byte("archive"), 0o400); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	migrated := &doctorResults{}
	checkMigrationSnapshots(migratedDir, config.CurrentConfigVersion, migrated)
	var details []string
	for _, note := range migrated.workspaceNotes {
		details = append(details, note.detail)
	}
	joined := strings.Join(details, "\n")
	for _, want := range []string{
		"clean schema-6 workspace; no pre-v6 snapshot recorded",
		"clean schema-7 workspace; no pre-v7 snapshot recorded",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("migrated workspace notes lack %q:\n%s", want, joined)
		}
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
