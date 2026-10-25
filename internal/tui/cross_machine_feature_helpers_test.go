package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// crossMachineWorkspace mints, seeds and opens one workspace for one race. Every
// race gets its own: a shared one would let an earlier race's commits decide a
// later one's merge, and the ledger and conflict assertions here are counts over
// the whole workspace.
func crossMachineWorkspace(t *testing.T) (*uitestWorkspace, *models.Project) {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	project, err := seedCrossMachineWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	openCrossMachineWorkspaceOnce(t, workspace.DataDir)
	return workspace, project
}

// crossMachineInterruptedResult is what a held-versus-writer race leaves to
// judge: the document both machines were about to rewrite, the workspace whose
// durable state the refusal is a claim about, and each machine's own outcome.
type crossMachineInterruptedResult struct {
	merged  *models.Project
	dataDir string
	held    string
	writer  string
	// undoBefore is the ledger depth before either machine ran, so a caller can
	// state how many entries the pair was allowed to add rather than asserting an
	// absolute count a seeded workspace might change.
	undoBefore int
	// planningBefore is the synced planning artifact as it stood before either
	// machine ran, for the caller that has to prove a refused batch did not
	// rewrite it.
	planningBefore []byte
}

// runCrossMachineInterruptedMutation holds one machine at its commit seam, lets
// a second machine commit and exit, and only then releases the first.
//
// It is the deterministic counterpart to runCrossMachineRace. Releasing both
// together proves the overlap but leaves the winner undecided, so neither
// machine's outcome can be asserted by name; here the writer is guaranteed to
// win and the held machine is guaranteed to be looking at a document that moved
// under it. That is what makes "the loser staged nothing" a claim a test can
// make, and what lets the two refusal shapes — a strict precondition and an
// ordinary recorded merge conflict — be told apart by which role was held.
func runCrossMachineInterruptedMutation(t *testing.T, held, writer crossMachineRole, prepare ...func(*models.Project)) crossMachineInterruptedResult {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("locate this test binary: %v", err)
	}
	workspace, project := crossMachineWorkspace(t)
	for _, seed := range prepare {
		seed(project)
		if err = storage.SaveProject(project); err != nil {
			t.Fatal(err)
		}
	}
	baseRevision := crossMachineProjectRevision(t, project)
	undoBefore := crossMachineUndoCount(t, workspace.DataDir)
	planningBefore := crossMachinePlanningBytes(t, workspace.DataDir)

	barrierDir := t.TempDir()
	proceed := filepath.Join(barrierDir, "go")
	heldHelper := startCrossMachineHelper(t, binary, workspace.DataDir, barrierDir, proceed, held)
	heldHelper.awaitBarrier(t)
	if heldHelper.revision != baseRevision {
		t.Fatalf("the held machine reached its commit boundary at revision %q, want the pre-race %q",
			heldHelper.revision, baseRevision)
	}
	if current := crossMachineProjectRevision(t, project); current != baseRevision {
		t.Fatalf("the document moved to %q while the machine was held; the barrier does not precede the commit", current)
	}

	// The writer runs unheld. Its own go file already exists, so a barrier it
	// installs passes straight through, and it publishes no ready file, so
	// nothing waits on it. The two overrides ride spec.env because exec dedupes
	// Cmd.Env keeping the last assignment.
	writerProceed := filepath.Join(barrierDir, "writer-go")
	if err = os.WriteFile(writerProceed, []byte("go\n"), storage.FilePermissions); err != nil {
		t.Fatalf("open the writer's barrier: %v", err)
	}
	writer.env = append(append([]string{}, writer.env...), helperReadyEnv+"=", helperGoEnv+"="+writerProceed)
	writerHelper := startCrossMachineHelper(t, binary, workspace.DataDir, barrierDir, writerProceed, writer)
	writerHelper.awaitExit(t)
	if current := crossMachineProjectRevision(t, project); current == baseRevision {
		t.Fatalf("the interrupting machine committed nothing; the held machine would have nothing to lose to")
	}

	if err = os.WriteFile(proceed, []byte("go\n"), storage.FilePermissions); err != nil {
		t.Fatalf("release the held machine: %v", err)
	}
	heldHelper.awaitExit(t)

	merged := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err = storage.LoadProject(merged); err != nil {
		t.Fatalf("reload %s after the race: %v", project.Name, err)
	}
	result := crossMachineInterruptedResult{
		merged:         merged,
		dataDir:        workspace.DataDir,
		held:           heldHelper.outcome(t),
		writer:         writerHelper.outcome(t),
		undoBefore:     undoBefore,
		planningBefore: planningBefore,
	}
	if strings.HasPrefix(result.held, "lost") || strings.HasPrefix(result.writer, "lost") {
		t.Fatalf("a commit was silently replaced: held %q / writer %q", result.held, result.writer)
	}
	if strings.HasPrefix(result.held, "failed:") || strings.HasPrefix(result.writer, "failed:") {
		t.Fatalf("a machine failed: held %q / writer %q", result.held, result.writer)
	}
	if result.writer != "committed" {
		t.Fatalf("the interrupting machine reported %q, want a commit", result.writer)
	}
	return result
}

// requireCrossMachineStagedNothing is the precondition refusal's whole contract,
// read off durable state rather than off the refused machine's word: no conflict
// bundle exists, and the ledger gained exactly the one entry the interrupting
// write was entitled to.
func requireCrossMachineStagedNothing(t *testing.T, race crossMachineInterruptedResult, writerAction string) {
	t.Helper()
	records, err := storage.NewConflictStore(config.ConflictsDir(race.dataDir)).List()
	if err != nil {
		t.Fatalf("list recorded conflicts: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("a precondition refusal staged %d conflict bundle(s); it is validated ahead of the merge", len(records))
	}
	if got, want := crossMachineUndoCount(t, race.dataDir), race.undoBefore+1; got != want {
		t.Fatalf("the workspace ledger holds %d undo entries, want %d — the interrupting write's alone", got, want)
	}
	if action := crossMachineNewestUndoAction(t, race.dataDir); action != writerAction {
		t.Fatalf("the ledger's newest entry is %q, want the interrupting %q", action, writerAction)
	}
}

func requireCrossMachinePlanningUnchanged(t *testing.T, race crossMachineInterruptedResult) {
	t.Helper()
	if after := crossMachinePlanningBytes(t, race.dataDir); string(after) != string(race.planningBefore) {
		t.Fatalf("the synced planning artifact changed under a refused batch")
	}
}

func crossMachineLedger(dataDir string) *storage.WorkspaceLedger {
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir})
}

func crossMachineUndoCount(t *testing.T, dataDir string) int {
	t.Helper()
	count, err := crossMachineLedger(dataDir).UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("count undo entries: %v", err)
	}
	return count
}

func crossMachineNewestUndoAction(t *testing.T, dataDir string) string {
	t.Helper()
	entry, ok, err := crossMachineLedger(dataDir).SelectNewestUndo(storage.GlobalScope())
	if err != nil {
		t.Fatalf("read the workspace ledger: %v", err)
	}
	if !ok {
		return ""
	}
	return entry.Action
}

func crossMachinePlanningBytes(t *testing.T, dataDir string) []byte {
	t.Helper()
	raw, err := os.ReadFile(config.PlanningStatePath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read planning.json: %v", err)
	}
	return raw
}
