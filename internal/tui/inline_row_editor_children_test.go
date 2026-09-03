package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// appendInlineTaskChain adds a root task followed by nodes-1 descendants, each
// the child of the previous one. The head's subtree height is therefore nodes-1,
// which is what the depth rulings are expressed in.
func appendInlineTaskChain(project *models.Project, prefix string, firstShortID, nodes int) {
	var previous *models.Task
	for offset := 0; offset < nodes; offset++ {
		node := &models.Task{
			ID:            fmt.Sprintf("%s-%d", strings.ToLower(prefix), offset),
			ShortID:       firstShortID + offset,
			Title:         fmt.Sprintf("%s %d", prefix, offset),
			WorkflowState: models.StateBacklog,
		}
		if previous != nil {
			node.ParentID = &previous.ID
		}
		project.Tasks = append(project.Tasks, node)
		previous = node
	}
}

// inlineEditorChildFixture extends the shared inline-editor fixture with one
// candidate per rule child completion has to apply. The edited task is still the
// fixture's Child (#2), which sits at depth one under Parent (#1), so a candidate
// subtree of height five overflows the limit and one of height four does not.
func inlineEditorChildFixture(t *testing.T) (*OutlinerModel, *models.Project, *models.Task) {
	t.Helper()
	out, project, _, child := inlineEditorFixture(t)
	project.Tasks = append(project.Tasks,
		&models.Task{ID: "sibling", ShortID: 3, Title: "Sibling", WorkflowState: models.StateBacklog},
		&models.Task{ID: "finished", ShortID: 4, Title: "Finished", WorkflowState: models.StateDone},
	)
	appendInlineTaskChain(project, "Tall", 10, 6)
	appendInlineTaskChain(project, "Fits", 20, 5)
	project.BuildTaskTree()
	out.rebuildRows()
	for index, row := range out.rows {
		if row.task != nil && row.task.ID == child.ID && row.isTaskRow() {
			out.cursor = index
			break
		}
	}
	return out, project, child
}

// draftedRowEditor returns a row editor holding draft with the cursor at the end
// and its autocomplete refreshed, which is the state a keystroke leaves behind.
func draftedRowEditor(t *testing.T, draft string, task *models.Task, project *models.Project, projects []*models.Project) *grammar.QuickEditComponent {
	t.Helper()
	q := grammar.NewQuickEditComponent()
	q.EnterWithContext(task, project, projects, false)
	q.Input.SetValue(draft)
	q.Input.CursorEnd()
	q.RefreshDraft()
	return q
}

func suggestionValues(items []grammar.ACSuggestion) []string {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = item.Value
	}
	return values
}

func TestInlineRowEditorChildSuggestionsWithholdEveryReferenceStrictParsingRefuses(t *testing.T) {
	_, project, child := inlineEditorChildFixture(t)
	projects := []*models.Project{project}
	q := draftedRowEditor(t, "Child <", child, project, projects)

	if q.AC.Mode != grammar.ACModeChild {
		t.Fatalf("autocomplete mode = %d, want acModeChild", q.AC.Mode)
	}
	displays := suggestionDisplays(q.AC.Items)
	if len(displays) == 0 || displays[0] != "<3 Sibling" {
		t.Fatalf("child suggestions = %v, want the child sigil in project order", displays)
	}
	for _, display := range displays {
		if strings.HasPrefix(display, "<0") {
			t.Fatalf("child completion offered a clear row it cannot parse: %v", displays)
		}
	}

	values := suggestionValues(q.AC.Items)
	for _, withheld := range []string{"1", "2", "4", "10"} {
		if slices.Contains(values, withheld) {
			t.Errorf("child suggestions offered #%s: %v", withheld, values)
		}
	}
	for _, offered := range []string{"3", "11", "20"} {
		if !slices.Contains(values, offered) {
			t.Errorf("child suggestions omitted #%s: %v", offered, values)
		}
	}

	// Every withheld candidate except the terminal one is withheld because strict
	// parsing refuses it. A completed task stays typeable, exactly as capture.
	refusals := map[string]string{
		"<1":  "is an ancestor of the edited task",
		"<2":  "task cannot be its own child",
		"<10": "exceed maximum nesting depth",
	}
	for draft, want := range refusals {
		if _, err := grammar.ParseQuickEdit("Child "+draft, child, project, projects); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("parseQuickEdit(%q) error = %v, want %q", draft, err, want)
		}
	}
	if _, err := grammar.ParseQuickEdit("Child <4", child, project, projects); err != nil {
		t.Errorf("manually typed completed-task adoption: %v", err)
	}
}

func TestInlineRowEditorChildSuggestionsFollowTheDraftedParentAndProjectRoute(t *testing.T) {
	_, project, child := inlineEditorChildFixture(t)
	routed := &models.Project{
		ID: "routed", Slug: "routed", Name: "Routed",
		Tasks: []*models.Task{
			{ID: "routed-1", ShortID: 1, Title: "Routed alpha", WorkflowState: models.StateBacklog},
			{ID: "routed-2", ShortID: 2, Title: "Routed beta", WorkflowState: models.StateBacklog},
		},
	}
	routed.BuildTaskTree()
	projects := []*models.Project{project, routed}

	// A drafted parent is an ancestor of the finished line, so it leaves the
	// candidate set; the current parent it replaces rejoins it.
	reparented := draftedRowEditor(t, "Child >3 <", child, project, projects)
	values := suggestionValues(reparented.AC.Items)
	if slices.Contains(values, "3") {
		t.Errorf("drafted parent #3 remained a child candidate: %v", values)
	}
	if !slices.Contains(values, "1") {
		t.Errorf("replaced parent #1 stayed withheld after >3: %v", values)
	}

	crossProject := draftedRowEditor(t, "Child %routed <", child, project, projects)
	if got, want := suggestionDisplays(crossProject.AC.Items), []string{"<1 Routed alpha", "<2 Routed beta"}; !slices.Equal(got, want) {
		t.Errorf("routed child suggestions = %v, want %v", got, want)
	}

	unresolved := draftedRowEditor(t, "Child %missing <", child, project, projects)
	if unresolved.AC.IsActive() {
		t.Errorf("unresolvable project route still offered children: %v", suggestionDisplays(unresolved.AC.Items))
	}
}

func TestInlineRowEditorChildCompletionIgnoresTheTokenUnderTheCursor(t *testing.T) {
	_, project, child := inlineEditorChildFixture(t)
	projects := []*models.Project{project}

	inProgress := draftedRowEditor(t, "Child <3", child, project, projects)
	if values := suggestionValues(inProgress.AC.Items); !slices.Contains(values, "3") {
		t.Fatalf("the alias being typed withheld its own task: %v", values)
	}

	q := draftedRowEditor(t, "Child <3 <20 tail", child, project, projects)
	q.Input.SetCursor(len([]rune("Child <3")))
	q.RefreshDraft()
	values := suggestionValues(q.AC.Items)
	if !slices.Contains(values, "3") {
		t.Errorf("mid-line completion withheld the task under the cursor: %v", values)
	}
	if slices.Contains(values, "20") {
		t.Errorf("mid-line completion offered a task a later token already adopts: %v", values)
	}
}

func TestInlineRowEditorChildAcceptanceRepeatsWhileParentStaysSingleton(t *testing.T) {
	_, project, child := inlineEditorChildFixture(t)
	projects := []*models.Project{project}

	q := draftedRowEditor(t, "Child <", child, project, projects)
	if transition, _, _ := q.Handle(tea.KeyPressMsg{Code: tea.KeyTab}); transition != grammar.QuickEditContinue {
		t.Fatalf("first child acceptance transition = %v, want continue", transition)
	}
	if got := q.Input.Value(); got != "Child <3 " {
		t.Fatalf("first acceptance = %q, want \"Child <3 \"", got)
	}

	q.Input.SetValue(q.Input.Value() + "<")
	q.Input.CursorEnd()
	q.RefreshDraft()
	if q.AC.Mode != grammar.ACModeChild {
		t.Fatalf("child completion did not repeat after acceptance: mode %d items %v", q.AC.Mode, suggestionDisplays(q.AC.Items))
	}
	if values := suggestionValues(q.AC.Items); slices.Contains(values, "3") {
		t.Fatalf("accepted child reappeared: %v", values)
	}
	q.Handle(tea.KeyPressMsg{Code: tea.KeyTab})

	transition, result, _ := q.Handle(tea.KeyPressMsg{Code: tea.KeyEnter})
	if transition != grammar.QuickEditCommit || result == nil {
		t.Fatalf("commit transition = %v result = %v", transition, result)
	}
	if got := childIDsOf(result); len(got) != 2 || got[0] != "sibling" {
		t.Fatalf("committed children = %v, want two adoptions led by the sibling", got)
	}
	if result.Title != "Child" {
		t.Fatalf("committed title = %q, want the reference tokens stripped", result.Title)
	}

	singleton := draftedRowEditor(t, "Child >3 >", child, project, projects)
	if singleton.AC.IsActive() {
		t.Fatalf("parent completion repeated after >3: %v", suggestionDisplays(singleton.AC.Items))
	}
}

func TestInlineRowEditorChildDraftRendersWithinMinimumWidth(t *testing.T) {
	out, project, child := inlineEditorChildFixture(t)
	// The shared fixture defers the edited task, which the default projection
	// hides; the row has to exist before the editor can be rendered into it.
	child.DeferredUntil = nil
	out.SetSize(minTerminalWidth, 14)
	out.rebuildRows()
	for index, row := range out.rows {
		if row.task != nil && row.task.ID == child.ID && row.isTaskRow() {
			out.cursor = index
			break
		}
	}
	out.quickEdit.EnterWithContext(child, project, []*models.Project{project}, false)
	out.quickEdit.Input.SetValue("Child <")
	out.quickEdit.Input.CursorEnd()
	out.quickEdit.RefreshDraft()

	rendered := ansi.Strip(out.renderOutlinerTreeContent(minTerminalWidth))
	if !strings.Contains(rendered, "EDIT TASK") || !strings.Contains(rendered, "Child <") {
		t.Fatalf("minimum-width editor missing:\n%s", rendered)
	}
	if !strings.Contains(rendered, "<3 Sibling") {
		t.Fatalf("minimum-width editor dropped the child dropdown:\n%s", rendered)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > minTerminalWidth {
			t.Errorf("line is %d columns at minimum width: %q", got, line)
		}
	}
	if got := lipgloss.Height(rendered); got > 14 {
		t.Fatalf("child dropdown escaped the viewport height=%d:\n%s", got, rendered)
	}
}

// requireCommittedInlineAdoption asserts the post-commit state a successful
// adoption owes the row editor: the draft is gone, the reloaded project nests
// the adoptee under its new owner, the rebuilt rows show that nesting, and the
// projection fingerprint is untouched so relocation jump history survives.
func requireCommittedInlineAdoption(t *testing.T, m *AppModel, out *OutlinerModel, task, adoptee *models.Task, projection string) {
	t.Helper()
	if out.quickEdit.IsActive() {
		t.Fatal("successful adoption left the row editor active")
	}
	reloaded := m.currentProject.GetTaskByID(adoptee.ID)
	if reloaded == nil || reloaded.ParentID == nil || *reloaded.ParentID != task.ID {
		t.Fatalf("reloaded hierarchy did not adopt the task: %+v", reloaded)
	}
	owner := m.currentProject.GetTaskByID(task.ID)
	if !slices.ContainsFunc(owner.Children, func(candidate *models.Task) bool { return candidate.ID == adoptee.ID }) {
		t.Fatalf("reloaded owner children = %+v, want the adopted task", owner.Children)
	}
	if got := out.jumpProjectionFingerprint(); got != projection {
		t.Fatalf("adoption changed projection state:\n before %s\n after  %s", projection, got)
	}
	ownerRow, adopteeRow := -1, -1
	for index, row := range out.rows {
		if row.task == nil || !row.isTaskRow() {
			continue
		}
		switch row.task.ID {
		case task.ID:
			ownerRow = index
		case adoptee.ID:
			adopteeRow = index
		}
	}
	if ownerRow < 0 || adopteeRow < ownerRow || len(out.rows[adopteeRow].prefix) <= len(out.rows[ownerRow].prefix) {
		t.Fatalf("rebuilt rows do not nest the adopted task under its new owner: owner=%d adoptee=%d", ownerRow, adopteeRow)
	}
}

func TestInlineRowEditorChildAdoptionCommitsRefreshesHierarchyAndUndoes(t *testing.T) {
	m, repo, task, _ := newPlanningLedgerApp(t, models.StateBacklog)
	adoptee := seedTask(t, 2, "Adoptee", models.StateBacklog)
	m.currentProject.Tasks = append(m.currentProject.Tasks, adoptee)
	m.currentProject.NextShortID = 3
	m.currentProject.BuildTaskTree()
	repo.seed(t)

	out := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
	out.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	out.SetSize(100, 20)
	out.rebuildRows()
	out.collapsedSections["unrelated"] = true
	m.crossOutliner = out
	m.navigation.ResetRoot()
	projection := out.jumpProjectionFingerprint()

	out.Update(StartTaskLineEditMsg{Task: task, Project: m.currentProject})
	out.quickEdit.Input.SetValue(task.Title + " <")
	out.quickEdit.Input.CursorEnd()
	out.quickEdit.RefreshDraft()
	if out.quickEdit.AC.Mode != grammar.ACModeChild {
		t.Fatalf("child completion inactive in the live row editor: mode %d", out.quickEdit.AC.Mode)
	}
	out.handleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyTab})
	if got := out.quickEdit.Input.Value(); !strings.Contains(got, "<2 ") {
		t.Fatalf("accepted draft = %q, want the adopted short ID", got)
	}

	_, cmd := out.handleQuickEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !out.quickEdit.Pending {
		t.Fatalf("adoption commit did not retain a pending draft: cmd=%v pending=%t", cmd != nil, out.quickEdit.Pending)
	}
	committed, ok := cmd().(coordinator.TaskLineEditedMsg)
	if !ok || !committed.Result.Changed || committed.Result.TransactionID == "" {
		t.Fatalf("adoption command result = %#v", committed)
	}
	m.handleTaskLineEdited(committed)

	requireCommittedInlineAdoption(t, m, out, task, adoptee, projection)

	if _, err := m.deps.Ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("undo adoption: %v", err)
	}
	if parentID := diskTask(t, m.currentProject, adoptee.ID).ParentID; parentID != nil {
		t.Fatalf("undo left the adoption on disk: %v", *parentID)
	}
	if _, err := m.deps.Ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("redo adoption: %v", err)
	}
	if parentID := diskTask(t, m.currentProject, adoptee.ID).ParentID; parentID == nil || *parentID != task.ID {
		t.Fatalf("redo did not restore the adoption: %v", parentID)
	}

	live := m.currentProject.GetTaskByID(task.ID)
	out.Update(StartTaskLineEditMsg{Task: live, Project: m.currentProject})
	out.quickEdit.Input.SetValue(live.Title + " <2")
	out.quickEdit.Input.CursorEnd()
	out.quickEdit.RefreshDraft()
	out.quickEdit.BeginCommit()
	failure := errors.New("simulated revision conflict")
	m.handleTaskLineEditFailed(coordinator.TaskLineEditFailedMsg{
		Source: taskref.For(live, m.currentProject), Err: failure,
	})
	if !out.quickEdit.IsActive() || out.quickEdit.Pending || !strings.Contains(out.quickEdit.Input.Value(), "<2") {
		t.Fatalf("failed adoption lost the draft: active=%t pending=%t value=%q",
			out.quickEdit.IsActive(), out.quickEdit.Pending, out.quickEdit.Input.Value())
	}
	if !strings.Contains(out.quickEdit.GetPreviewError(), failure.Error()) {
		t.Fatalf("inline adoption failure = %q, want %q", out.quickEdit.GetPreviewError(), failure)
	}
}
