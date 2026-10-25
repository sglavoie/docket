package tui

import (
	"fmt"
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/valuepicker"
)

// bulkMoveSelection installs a frozen visual selection over the named fixture
// tasks and returns them with their owning projects, in the order asked for.
// The selection goes into the live Outliner rather than a hand-built context, so
// the test drives the same target resolution the visual-selection `%` does.
func bulkMoveSelection(t *testing.T, m *AppModel, titles ...string) []TaskProjectRef {
	t.Helper()
	refs := make([]TaskProjectRef, 0, len(titles))
	identities := make([]taskref.TaskIdentity, 0, len(titles))
	for _, title := range titles {
		task, project := taskContentFixtureTask(t, m, title)
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
		identities = append(identities, taskref.For(task, project))
	}
	if m.crossOutliner == nil {
		t.Fatal("the fixture app has no Outliner to select in")
	}
	m.crossOutliner.ReplaceSelection(identities)
	return refs
}

// executeBulkMoveToProject runs the picked destination through the real
// executor and unwraps the command the binding returned. Going through
// operationExecutor is the point: it proves the manifest's Bulk flag widened the
// descriptor's target source and that executeBulkTaskValueOperation now routes
// the move instead of refusing it.
func executeBulkMoveToProject(t *testing.T, m *AppModel, destinationID string) tea.Msg {
	t.Helper()
	result := m.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:       operation.OperationTaskMoveToProject,
		Context:  m.operationCommandContext(),
		Value:    destinationID,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute Move to Project: %v", result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("Move to Project produced %T, want an operation command", result.Message)
	}
	if command.Cmd == nil {
		t.Fatal("Move to Project produced no command")
	}
	return command.Cmd()
}

func taskTitlesInProject(t *testing.T, m *AppModel, projectID string) []string {
	t.Helper()
	project := m.projectByID(projectID)
	if project == nil {
		t.Fatalf("project %q is no longer loaded", projectID)
	}
	titles := make([]string, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil {
			titles = append(titles, task.Title)
		}
	}
	sort.Strings(titles)
	return titles
}

// TestBulkMoveToProjectCommitsASelectionSpanningSourceProjects is the session's
// central claim: a visual selection whose tasks live in three different projects
// relocates into one destination as a single transaction with a single undo
// entry, and the count in the status sentence is the number of subtrees moved,
// not the number of source projects crossed.
func TestBulkMoveToProjectCommitsASelectionSpanningSourceProjects(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	refs := bulkMoveSelection(t,
		m,
		"Alpha migration kickoff",
		"Renew passport",
		"Finish the distributed systems book",
	)
	destination := taskContentFixtureProject(t, m, "Client Beta")

	msg, ok := executeBulkMoveToProject(t, m, destination.ID).(coordinator.BatchTransactionResultMsg)
	if !ok {
		t.Fatalf("a multi-project selection did not commit as a batch")
	}
	if msg.Action != "Move to Project" {
		t.Errorf("batch action = %q, want %q", msg.Action, "Move to Project")
	}
	if want := "Moved 3 tasks to Client Beta"; msg.Summary != want {
		t.Errorf("batch summary = %q, want %q", msg.Summary, want)
	}
	if msg.TransactionID == "" {
		t.Error("the committed batch carries no transaction ID")
	}

	touched := make(map[string]bool, len(msg.TouchedProjectIDs))
	for _, projectID := range msg.TouchedProjectIDs {
		touched[projectID] = true
	}
	for _, ref := range refs {
		if !touched[ref.Project.ID] {
			t.Errorf("source project %q is missing from the touched set", ref.Project.Name)
		}
	}
	if !touched[destination.ID] {
		t.Error("the destination project is missing from the touched set")
	}

	if len(msg.ClearSelectionTaskLocators) != len(refs) {
		t.Fatalf(
			"cleared locators = %d, want the %d source addresses the move emptied",
			len(msg.ClearSelectionTaskLocators),
			len(refs),
		)
	}
	cleared := make(map[taskref.TaskIdentity]bool, len(msg.ClearSelectionTaskLocators))
	for _, locator := range msg.ClearSelectionTaskLocators {
		cleared[taskref.FromLocator(locator)] = true
	}
	for _, ref := range refs {
		if !cleared[ref.Identity()] {
			t.Errorf("the source address of %q survives in the selection", ref.Task.Title)
		}
	}

	m.handleBatchTransactionResult(msg)

	// The Outliner snapshot refreshed: every source is empty and the destination
	// holds its own task plus the three that arrived.
	for _, ref := range refs {
		if titles := taskTitlesInProject(t, m, ref.Project.ID); len(titles) != 0 {
			t.Errorf("source project %q still holds %v", ref.Project.Name, titles)
		}
	}
	landed := taskTitlesInProject(t, m, destination.ID)
	want := []string{
		"Alpha migration kickoff",
		"Beta rollout checklist",
		"Finish the distributed systems book",
		"Renew passport",
	}
	if fmt.Sprint(landed) != fmt.Sprint(want) {
		t.Errorf("destination tasks = %v, want %v", landed, want)
	}

	// A move invalidates the very identities the selection is made of, so the
	// committed batch leaves no selection behind to report or to chain from.
	if count := m.crossOutliner.SelectedCount(); count != 0 {
		t.Errorf("selection retained %d stale identity/identities after the move", count)
	}
	if m.crossOutliner.IsMultiSelectActive() {
		t.Error("visual selection stayed active with nothing selected")
	}

	// The status strip reads the ledger entry the batch wrote, so the undo label
	// names the move and its scale once for the whole selection.
	wantStatus := fmt.Sprintf("Move to Project: 3 tasks → %s · u undo", destination.Slug)
	if m.feedback.message != wantStatus {
		t.Errorf("status strip = %q, want %q", m.feedback.message, wantStatus)
	}
}

// TestBulkMoveToProjectUndoRestoresEveryTaskToItsOriginProject pins the single
// undo entry: one `u` returns all three tasks to the three projects they came
// from, so the batch is not three transactions wearing one status sentence.
func TestBulkMoveToProjectUndoRestoresEveryTaskToItsOriginProject(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	refs := bulkMoveSelection(t,
		m,
		"Alpha migration kickoff",
		"Renew passport",
		"Finish the distributed systems book",
	)
	destination := taskContentFixtureProject(t, m, "Client Beta")
	origins := make(map[string]string, len(refs))
	for _, ref := range refs {
		origins[ref.Task.Title] = ref.Project.ID
	}

	msg, ok := executeBulkMoveToProject(t, m, destination.ID).(coordinator.BatchTransactionResultMsg)
	if !ok {
		t.Fatal("a multi-project selection did not commit as a batch")
	}
	m.handleBatchTransactionResult(msg)

	m = sendAppKey(m, syntheticKey('u'))
	for _, projectID := range msg.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}

	for title, originID := range origins {
		origin := m.projectByID(originID)
		if origin == nil {
			t.Fatalf("origin project %q is no longer loaded", originID)
		}
		found := false
		for _, task := range origin.Tasks {
			if task != nil && task.Title == title {
				found = true
			}
		}
		if !found {
			t.Errorf("undo did not return %q to %q", title, origin.Name)
		}
	}
	if titles := taskTitlesInProject(t, m, destination.ID); fmt.Sprint(titles) != fmt.Sprint([]string{"Beta rollout checklist"}) {
		t.Errorf("destination after undo = %v, want only its own task", titles)
	}
}

// TestMoveToProjectPickerWithholdsOnlyASingleSourceSelectionsOwnProject fixes
// what the destination list may hide. For one source project the current
// project is the one destination that could change nothing; for a selection
// spanning several, every project is a real destination for the tasks not
// already there, so withholding the first ref's project would drop a legitimate
// one on the accident of enumeration order.
func TestMoveToProjectPickerWithholdsOnlyASingleSourceSelectionsOwnProject(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	alpha, alphaProject := taskContentFixtureTask(t, m, "Alpha migration kickoff")
	errand, errandProject := taskContentFixtureTask(t, m, "Renew passport")

	sameProject := []TaskProjectRef{{Task: alpha, Project: alphaProject}}
	if got := valuepicker.CommonTaskProjectID(sameProject); got != alphaProject.ID {
		t.Errorf("single-source selection withholds %q, want %q", got, alphaProject.ID)
	}
	spanning := []TaskProjectRef{
		{Task: alpha, Project: alphaProject},
		{Task: errand, Project: errandProject},
	}
	if got := valuepicker.CommonTaskProjectID(spanning); got != "" {
		t.Errorf("multi-source selection withholds %q, want nothing withheld", got)
	}

	offered := make(map[string]bool)
	for _, item := range valuepicker.OperationProjectListSource(m.domain.runtime.AvailableProjects(), valuepicker.CommonTaskProjectID(spanning)).Items {
		offered[item.PrimaryText] = true
	}
	for _, name := range []string{alphaProject.Name, errandProject.Name} {
		if !offered[name] {
			t.Errorf("the picker omits %q, a real destination for the tasks not already in it", name)
		}
	}
}

// TestMoveToProjectWithoutASelectionKeepsTheSingleTaskPath is the no-regression
// half of the session: with nothing selected, `%` still commits the same
// task-line edit the `%project` sigil does, not a batch.
func TestMoveToProjectWithoutASelectionKeepsTheSingleTaskPath(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	source, sourceProject := taskContentFixtureTask(t, m, "Alpha migration kickoff")
	destination := taskContentFixtureProject(t, m, "Client Beta")

	if m.crossOutliner != nil && m.crossOutliner.IsMultiSelectActive() {
		t.Fatal("the fixture launched with a visual selection active")
	}

	context := m.operationCommandContext()
	context.Entity = outliner.TaskOperationEntity(source, sourceProject)
	result := m.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:       operation.OperationTaskMoveToProject,
		Context:  context,
		Value:    destination.ID,
		HasValue: true,
	})
	if result.Err != nil {
		t.Fatalf("execute Move to Project: %v", result.Err)
	}
	command, ok := result.Message.(operation.OperationCommandResultMsg)
	if !ok {
		t.Fatalf("Move to Project produced %T, want an operation command", result.Message)
	}
	msg := command.Cmd()
	edited, ok := msg.(coordinator.TaskLineEditedMsg)
	if !ok {
		t.Fatalf("a selection-free move produced %T, want the single task-line edit", msg)
	}
	if !edited.Result.Changed {
		t.Fatal("a selection-free cross-project move reported no change")
	}

	m.reloadProjectByID(sourceProject.ID)
	m.reloadProjectByID(destination.ID)
	if titles := taskTitlesInProject(t, m, sourceProject.ID); len(titles) != 0 {
		t.Errorf("the source project still holds %v", titles)
	}
	if moved := m.projectByID(destination.ID).GetTaskByID(source.ID); moved == nil {
		t.Error("the destination is missing the moved task")
	}
}

// TestMoveToProjectRefusesAnEmptyTargetSet keeps the multi-target shape honest
// at its lower bound: no identities is a refusal, never a silent no-op that
// leaves the user believing a move committed.
func TestMoveToProjectRefusesAnEmptyTargetSet(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	destination := taskContentFixtureProject(t, m, "Client Beta")

	cmd := m.moveTaskToProjectCmd(nil, destination.ID, nil)
	if cmd == nil {
		t.Fatal("an empty move produced no command")
	}
	if _, ok := cmd().(coordinator.TransactionErrorMsg); !ok {
		t.Fatalf("an empty move produced %T, want a rejection", cmd())
	}
}
