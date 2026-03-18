package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// minimalOutliner returns a bare OutlinerModel suitable for unit tests that
// call buildCrossProjectRowsWithSectionHeaders, buildProjectGroupedRows, or
// flattenTaskTreeWithFoldsAndProjectID directly. It deliberately avoids
// config/storage so tests remain free of disk I/O.
func minimalOutliner() *OutlinerModel {
	return &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
	}
}

// makeTask creates a minimal task with the given title and section.
func makeTask(id, title, section string) *models.Task {
	return &models.Task{
		ID:      id,
		Title:   title,
		Section: section,
	}
}

// ── buildCrossProjectRowsWithSectionHeaders ──────────────────────────────────

// TestBuildCrossProjectRowsSectionOrdering verifies that section headers
// appear in canonical GTD order (Active/"", Inbox, Next, Waiting, Review,
// Standby, Someday) regardless of the order tasks are provided.
func TestBuildCrossProjectRowsSectionOrdering(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "proj-1",
			tasks: []*models.Task{
				makeTask("t1", "Someday task", models.SectionSomeday),
				makeTask("t2", "Inbox task", models.SectionInbox),
				makeTask("t3", "Active task", ""),
			},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	// Collect section header names in the order they appear in m.rows.
	var headerOrder []string
	for _, row := range m.rows {
		if row.IsSectionHeader {
			name := row.SectionName
			if name == "" {
				name = "Active" // canonical display name for empty string
			}
			headerOrder = append(headerOrder, name)
		}
	}

	want := []string{"Active", models.SectionInbox, models.SectionSomeday}
	if len(headerOrder) != len(want) {
		t.Fatalf("expected %d section headers, got %d: %v", len(want), len(headerOrder), headerOrder)
	}
	for i, name := range want {
		if headerOrder[i] != name {
			t.Errorf("header[%d]: want %q, got %q", i, name, headerOrder[i])
		}
	}
}

// TestBuildCrossProjectRowsEmptySectionSkipped verifies that sections with no
// tasks produce no header row.
func TestBuildCrossProjectRowsEmptySectionSkipped(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "proj-1",
			tasks: []*models.Task{
				makeTask("t1", "Next task", models.SectionNext),
			},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	for _, row := range m.rows {
		if row.IsSectionHeader && row.SectionName != models.SectionNext {
			t.Errorf("unexpected section header %q (expected only Next)", row.SectionName)
		}
	}

	// Exactly one header expected.
	headerCount := 0
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headerCount++
		}
	}
	if headerCount != 1 {
		t.Errorf("expected 1 section header, got %d", headerCount)
	}
}

// TestBuildCrossProjectRowsMultiProjectMerging verifies that tasks from two
// different projects in the same section appear under ONE shared section header
// (no per-project sub-headers), and that every task row carries the correct
// ProjectID for color-badge rendering.
func TestBuildCrossProjectRowsMultiProjectMerging(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "proj-a",
			tasks:     []*models.Task{makeTask("ta1", "A task", models.SectionNext)},
		},
		{
			projectID: "proj-b",
			tasks:     []*models.Task{makeTask("tb1", "B task", models.SectionNext)},
		},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	var headers []outlinerRow
	var taskRows []outlinerRow
	for _, row := range m.rows {
		if row.IsSectionHeader {
			headers = append(headers, row)
		} else if !row.IsSeparator {
			taskRows = append(taskRows, row)
		}
	}

	// Both projects share the same section — only ONE shared header expected.
	if len(headers) != 1 {
		t.Errorf("expected 1 shared section header for 2 projects in same section, got %d", len(headers))
	}

	// The shared header must have no ProjectID (no per-project label rendered).
	if len(headers) == 1 && headers[0].ProjectID != "" {
		t.Errorf("shared section header should have empty ProjectID, got %q", headers[0].ProjectID)
	}

	// The shared header CollapseKey must be the section name only (not composite).
	if len(headers) == 1 && headers[0].CollapseKey != models.SectionNext {
		t.Errorf("shared header CollapseKey: want %q, got %q", models.SectionNext, headers[0].CollapseKey)
	}

	// The shared header SectionCount must reflect all tasks across both projects.
	if len(headers) == 1 && headers[0].SectionCount != 2 {
		t.Errorf("shared header SectionCount: want 2, got %d", headers[0].SectionCount)
	}

	// Task rows must carry their respective project IDs for badge rendering.
	projectIDs := make(map[string]bool)
	for _, row := range taskRows {
		if row.ProjectID == "" {
			t.Errorf("task row %q has empty ProjectID", row.task.Title)
		}
		projectIDs[row.ProjectID] = true
	}
	if !projectIDs["proj-a"] || !projectIDs["proj-b"] {
		t.Error("expected task rows from both proj-a and proj-b")
	}
}

// TestBuildCrossProjectRowsAllGroupsEmpty verifies that when the input groups
// slice is empty (or has no tasks), m.rows is not modified.
func TestBuildCrossProjectRowsAllGroupsEmpty(t *testing.T) {
	m := minimalOutliner()

	m.buildCrossProjectRowsWithSectionHeaders(nil)

	if len(m.rows) != 0 {
		t.Errorf("expected empty rows for nil groups, got %d rows", len(m.rows))
	}

	m.buildCrossProjectRowsWithSectionHeaders([]projectTasksGroup{})

	if len(m.rows) != 0 {
		t.Errorf("expected empty rows for empty groups, got %d rows", len(m.rows))
	}
}

// ── buildProjectGroupedRows ───────────────────────────────────────────────────

// TestBuildProjectGroupedRowsSeparatorPlacement verifies that a single
// IsSeparator row is inserted between consecutive project groups, not before
// the first or after the last.
func TestBuildProjectGroupedRowsSeparatorPlacement(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	// Count separators.
	separatorCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			separatorCount++
		}
	}

	// With 3 groups, 2 separators expected (one between each pair).
	if separatorCount != 2 {
		t.Errorf("expected 2 separators for 3 groups, got %d", separatorCount)
	}

	// First row must not be a separator.
	if len(m.rows) == 0 {
		t.Fatal("no rows produced")
	}
	if m.rows[0].IsSeparator {
		t.Error("first row must not be a separator")
	}

	// Last row must not be a separator.
	if m.rows[len(m.rows)-1].IsSeparator {
		t.Error("last row must not be a separator")
	}
}

// TestBuildProjectGroupedRowsSingleGroup verifies that a single project group
// produces no separator rows.
func TestBuildProjectGroupedRowsSingleGroup(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
	}

	m.buildProjectGroupedRows(groups)

	for _, row := range m.rows {
		if row.IsSeparator {
			t.Error("single group must not produce any separator rows")
		}
	}
}

// TestBuildProjectGroupedRowsIndependentSorting verifies that each project's
// tasks appear as a contiguous block, preserving the insertion order supplied
// by the caller. Projects are NOT interleaved.
func TestBuildProjectGroupedRowsIndependentSorting(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{
			projectID: "p1",
			tasks: []*models.Task{
				makeTask("a1", "Alpha 1", ""),
				makeTask("a2", "Alpha 2", ""),
			},
		},
		{
			projectID: "p2",
			tasks: []*models.Task{
				makeTask("b1", "Beta 1", ""),
			},
		},
	}

	m.buildProjectGroupedRows(groups)

	// Collect ProjectIDs in order, skipping separators.
	var ids []string
	for _, row := range m.rows {
		if !row.IsSeparator {
			ids = append(ids, row.ProjectID)
		}
	}

	// Expect: p1, p1, p2 — no interleaving.
	want := []string{"p1", "p1", "p2"}
	if len(ids) != len(want) {
		t.Fatalf("expected %d task rows, got %d", len(want), len(ids))
	}
	for i, wantID := range want {
		if ids[i] != wantID {
			t.Errorf("row[%d] ProjectID: want %q, got %q", i, wantID, ids[i])
		}
	}
}

// TestBuildProjectGroupedRowsEmptyGroups verifies that an empty groups slice
// leaves m.rows untouched.
func TestBuildProjectGroupedRowsEmptyGroups(t *testing.T) {
	m := minimalOutliner()

	m.buildProjectGroupedRows(nil)

	if len(m.rows) != 0 {
		t.Errorf("expected no rows for nil groups, got %d", len(m.rows))
	}
}

// ── separator skip loop ───────────────────────────────────────────────────────

// TestSeparatorSkipLoop verifies that separator rows are distinguishable from
// task rows and section headers via the IsSeparator field, matching what the
// render loop uses to skip them.
func TestSeparatorSkipLoop(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task A", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task B", "")}},
	}

	m.buildProjectGroupedRows(groups)

	taskRowCount := 0
	separatorCount := 0
	for _, row := range m.rows {
		switch {
		case row.IsSeparator:
			separatorCount++
			// A separator row must have no task attached.
			if row.task != nil {
				t.Error("separator row must not carry a task")
			}
			// A separator must not also be a section header.
			if row.IsSectionHeader {
				t.Error("separator row must not also be IsSectionHeader")
			}
		case !row.IsSectionHeader:
			taskRowCount++
		}
	}

	if separatorCount != 1 {
		t.Errorf("expected 1 separator between 2 groups, got %d", separatorCount)
	}
	if taskRowCount != 2 {
		t.Errorf("expected 2 task rows, got %d", taskRowCount)
	}
}

// TestSeparatorIsSeparatorNotSectionHeader verifies that separator rows set
// IsSeparator=true and IsSectionHeader=false, matching what the render and
// cursor-navigation loops use to distinguish them. The caller
// (crossProjectRebuildRows) pre-filters empty groups before passing to
// buildProjectGroupedRows, so all groups in production contain at least one
// task. This test mirrors that invariant by using non-empty groups only.
func TestSeparatorIsSeparatorNotSectionHeader(t *testing.T) {
	m := minimalOutliner()

	groups := []projectTasksGroup{
		{projectID: "p1", tasks: []*models.Task{makeTask("t1", "Task 1", "")}},
		{projectID: "p2", tasks: []*models.Task{makeTask("t2", "Task 2", "")}},
		{projectID: "p3", tasks: []*models.Task{makeTask("t3", "Task 3", "")}},
	}

	m.buildProjectGroupedRows(groups)

	for i, row := range m.rows {
		if row.IsSeparator && row.IsSectionHeader {
			t.Errorf("row[%d] is both IsSeparator and IsSectionHeader — must be mutually exclusive", i)
		}
		if row.IsSeparator && row.task != nil {
			t.Errorf("row[%d] is a separator but has a non-nil task", i)
		}
	}

	// Exactly 2 separators between 3 groups.
	separatorCount := 0
	for _, row := range m.rows {
		if row.IsSeparator {
			separatorCount++
		}
	}
	if separatorCount != 2 {
		t.Errorf("expected 2 separators, got %d", separatorCount)
	}
}

// TestSectionCountAccuracy verifies that the SectionCount on each header row
// reflects the recursive task count, including any nested children.
func TestSectionCountAccuracy(t *testing.T) {
	m := minimalOutliner()

	child := makeTask("c1", "Child task", models.SectionNext)
	parent := makeTask("p1", "Parent task", models.SectionNext)
	parent.Children = []*models.Task{child}

	groups := []projectTasksGroup{
		{projectID: "proj-1", tasks: []*models.Task{parent}},
	}

	m.buildCrossProjectRowsWithSectionHeaders(groups)

	for _, row := range m.rows {
		if row.IsSectionHeader && row.SectionName == models.SectionNext {
			// parent + child = 2
			if row.SectionCount != 2 {
				t.Errorf("SectionCount: want 2, got %d", row.SectionCount)
			}
		}
	}
}

// ── Multi-project undo orchestration (integration, real disk I/O) ─────────────

// TestCrossProjectUndoOrchestration is an integration-level test that uses real
// disk I/O (t.TempDir()) to verify the full multi-project undo lifecycle:
//
//  1. Two projects are created with mixed task hierarchies (3-level epic→story→task
//     for Alpha, 2-level parent→children for Beta).
//  2. Both projects are saved to and reloaded from disk.
//  3. A bulk status mutation is staged and executed across both projects.
//  4. Backup files are verified on disk; tasks.md mutation is confirmed.
//  5. Undo restores each project; per-project outcome is verified in the status
//     message in the format "Undo: restored <name>, <name>".
func TestCrossProjectUndoOrchestration(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	// ── Project directories ───────────────────────────────────────────────────
	alphaDir := t.TempDir()
	betaDir := t.TempDir()

	// ── Alpha: 3-level hierarchy (Epic → Story → leaf Task) + standalone Task ─
	const (
		alphaEpicID       = "alpha-epic-001"
		alphaStoryID      = "alpha-story-001"
		alphaLeafID       = "alpha-task-001"
		alphaStandaloneID = "alpha-task-002"
	)

	alphaEpic, _ := models.NewTask(alphaEpicID, 1, models.TaskTypeEpic, "Epic: Authentication")
	alphaStory, _ := models.NewTask(alphaStoryID, 2, models.TaskTypeStory, "Story: Login Flow")
	alphaStory.ParentID = strPtr(alphaEpicID)
	alphaLeaf, _ := models.NewTask(alphaLeafID, 3, models.TaskTypeTask, "Implement login endpoint")
	alphaLeaf.ParentID = strPtr(alphaStoryID)
	alphaStandalone, _ := models.NewTask(alphaStandaloneID, 4, models.TaskTypeTask, "Fix typo in readme")

	alpha := &models.Project{ID: "alpha-proj", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaEpic, alphaStory, alphaLeaf, alphaStandalone}
	alpha.BuildTaskTree()
	if err := storage.SaveProject(alpha); err != nil {
		t.Fatalf("save alpha: %v", err)
	}

	// ── Beta: 2-level hierarchy (Parent → 2 children) + standalone Task ───────
	const (
		betaParentID     = "beta-task-001"
		betaChild1ID     = "beta-task-002"
		betaChild2ID     = "beta-task-003"
		betaStandaloneID = "beta-task-004"
	)

	betaParent, _ := models.NewTask(betaParentID, 1, models.TaskTypeTask, "Setup CI pipeline")
	betaChild1, _ := models.NewTask(betaChild1ID, 2, models.TaskTypeTask, "Configure lint")
	betaChild1.ParentID = strPtr(betaParentID)
	betaChild2, _ := models.NewTask(betaChild2ID, 3, models.TaskTypeTask, "Configure tests")
	betaChild2.ParentID = strPtr(betaParentID)
	betaStandalone, _ := models.NewTask(betaStandaloneID, 4, models.TaskTypeTask, "Write API docs")

	beta := &models.Project{ID: "beta-proj", Name: "Beta", Path: betaDir}
	beta.Tasks = []*models.Task{betaParent, betaChild1, betaChild2, betaStandalone}
	beta.BuildTaskTree()
	if err := storage.SaveProject(beta); err != nil {
		t.Fatalf("save beta: %v", err)
	}

	// Reload both projects from disk so Tasks slice reflects parsed state.
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("load alpha: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("load beta: %v", err)
	}

	// Record pre-mutation tasks.md content for comparison after undo.
	originalAlpha := readCrossTestFile(t, filepath.Join(alphaDir, "tasks.md"))
	originalBeta := readCrossTestFile(t, filepath.Join(betaDir, "tasks.md"))

	// ── Create cross-project outliner (project==nil → cross-project mode) ─────
	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:       NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1, // All perspective
		loadedProjects:    []*models.Project{alpha, beta},
	}

	// Stage bulk status change: one task from each project → in_progress.
	selectedAlpha := alpha.GetTaskByID(alphaLeafID)
	selectedBeta := beta.GetTaskByID(betaChild1ID)
	if selectedAlpha == nil || selectedBeta == nil {
		t.Fatal("could not find selected tasks in loaded projects after reload")
	}
	bulkTasks := []*models.Task{selectedAlpha, selectedBeta}
	m.bulkOp.SetOperation(models.StatusInProgress, bulkTasks, 0, nil)

	// ── Execute bulk mutation ─────────────────────────────────────────────────
	resultModel, _ := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)

	// ── Verify backup files created for both projects ─────────────────────────
	alphaBackupPath := storage.GetBackupFilePath(alpha)
	betaBackupPath := storage.GetBackupFilePath(beta)

	if _, err := os.Stat(alphaBackupPath); os.IsNotExist(err) {
		t.Error("Alpha backup file should exist after bulk mutation")
	}
	if _, err := os.Stat(betaBackupPath); os.IsNotExist(err) {
		t.Error("Beta backup file should exist after bulk mutation")
	}

	// Backup content must match the pre-mutation snapshot.
	if got := readCrossTestFile(t, alphaBackupPath); got != originalAlpha {
		t.Error("Alpha backup should contain pre-mutation tasks.md content")
	}
	if got := readCrossTestFile(t, betaBackupPath); got != originalBeta {
		t.Error("Beta backup should contain pre-mutation tasks.md content")
	}

	// tasks.md must now differ from the original (mutation persisted to disk).
	if got := readCrossTestFile(t, filepath.Join(alphaDir, "tasks.md")); got == originalAlpha {
		t.Error("Alpha tasks.md should be mutated after bulk status change")
	}
	if got := readCrossTestFile(t, filepath.Join(betaDir, "tasks.md")); got == originalBeta {
		t.Error("Beta tasks.md should be mutated after bulk status change")
	}

	// Status message after mutation: "Changed N tasks to in_progress".
	wantMutMsg := fmt.Sprintf("Changed %d tasks to %s", len(bulkTasks), models.StatusInProgress)
	if m.statusMessage != wantMutMsg {
		t.Errorf("status after mutation: want %q, got %q", wantMutMsg, m.statusMessage)
	}

	// Undo context must record both project IDs.
	if got := len(m.crossUndoCtx.ProjectIDs); got != 2 {
		t.Errorf("undo context: want 2 project IDs, got %d", got)
	}

	// ── Execute undo ──────────────────────────────────────────────────────────
	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false in cross-project mode")
	}
	m = undoResult.(*OutlinerModel)

	// Status message format: "Undo: restored <name>[, <name>]"
	// Projects are restored in reverse ID order, so both must appear.
	undoMsg := m.statusMessage
	if !strings.HasPrefix(undoMsg, "Undo: restored ") {
		t.Errorf("undo status should start with 'Undo: restored ', got %q", undoMsg)
	}
	if !strings.Contains(undoMsg, "Alpha") {
		t.Errorf("undo status should contain project name 'Alpha', got %q", undoMsg)
	}
	if !strings.Contains(undoMsg, "Beta") {
		t.Errorf("undo status should contain project name 'Beta', got %q", undoMsg)
	}
	if strings.Contains(undoMsg, "FAILED") {
		t.Errorf("undo status should not contain 'FAILED', got %q", undoMsg)
	}

	// tasks.md for both projects must be restored to pre-mutation content.
	if got := readCrossTestFile(t, filepath.Join(alphaDir, "tasks.md")); got != originalAlpha {
		t.Error("Alpha tasks.md should be restored to pre-mutation content after undo")
	}
	if got := readCrossTestFile(t, filepath.Join(betaDir, "tasks.md")); got != originalBeta {
		t.Error("Beta tasks.md should be restored to pre-mutation content after undo")
	}

	// Undo context should be cleared after a successful undo.
	if got := len(m.crossUndoCtx.ProjectIDs); got != 0 {
		t.Errorf("undo context should be cleared after undo, got %d IDs", got)
	}
}

// TestValidateCrossZoomStackContextChange verifies that when the context changes
// and the zoomed project is no longer in loadedProjects, validateCrossZoomStack
// pops the stale entry and resets the cursor to 0. After the reset, rebuilding
// rows from the remaining loaded projects produces a non-empty row list.
func TestValidateCrossZoomStackContextChange(t *testing.T) {
	// Set up an outliner zoomed into proj-a, but loadedProjects only has proj-b
	// (simulating a context change that removed proj-a from the active context).
	projB := &models.Project{
		ID:    "proj-b",
		Name:  "Beta",
		Tasks: []*models.Task{makeTask("tb1", "Beta task", "")},
	}
	projB.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		loadedProjects:    []*models.Project{projB},
		crossZoomStack: []CrossProjectZoomEntry{
			{TaskID: "ta1", ProjectID: "proj-a", CursorIndex: 5},
		},
	}
	m.cursor = 5

	m.validateCrossZoomStack()

	// Zoom stack must be cleared: proj-a is not in loadedProjects.
	if len(m.crossZoomStack) != 0 {
		t.Errorf("expected empty zoom stack after context change removed zoomed project, got %d entries", len(m.crossZoomStack))
	}

	// Cursor must reset to 0.
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after zoom stack reset, got %d", m.cursor)
	}

	// Row rebuild from the remaining project must produce a non-empty list.
	m.activePerspective = 1 // All perspective
	m.crossProjectRebuildRows()
	if len(m.rows) == 0 {
		t.Error("expected non-empty rows from remaining loaded project after zoom stack reset")
	}
}

// TestValidateCrossZoomStackProjectIDScoping verifies that validateCrossZoomStack
// resolves zoom entries by ProjectID, not by searching the global task list.
// Two projects share the same task UUID; a CrossProjectZoomEntry is scoped to
// one project. The test confirms:
//   - The stack is preserved when the scoped project contains the task.
//   - The stack is cleared when the scoped project is removed from loadedProjects,
//     even though the duplicate UUID still exists in the other project.
func TestValidateCrossZoomStackProjectIDScoping(t *testing.T) {
	const sharedUUID = "shared-task-uuid"

	projA := &models.Project{
		ID:    "proj-a",
		Name:  "Alpha",
		Tasks: []*models.Task{makeTask(sharedUUID, "Task in Alpha", "")},
	}
	projA.BuildTaskTree()

	projB := &models.Project{
		ID:    "proj-b",
		Name:  "Beta",
		Tasks: []*models.Task{makeTask(sharedUUID, "Task in Beta", "")},
	}
	projB.BuildTaskTree()

	// ── Part 1: Stack is preserved when scoped project is present ─────────────
	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		loadedProjects:    []*models.Project{projA, projB},
		crossZoomStack: []CrossProjectZoomEntry{
			{TaskID: sharedUUID, ProjectID: "proj-a", CursorIndex: 3},
		},
	}
	m.cursor = 3

	m.validateCrossZoomStack()

	if len(m.crossZoomStack) != 1 {
		t.Errorf("stack should be preserved when scoped project has the task: got %d entries", len(m.crossZoomStack))
	}
	if m.cursor != 3 {
		t.Errorf("cursor should be unchanged when stack is valid: want 3, got %d", m.cursor)
	}

	// ── Part 2: Stack is cleared when scoped project is removed ───────────────
	// proj-b still has sharedUUID, but the entry is scoped to proj-a (now gone).
	m2 := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		loadedProjects:    []*models.Project{projB}, // proj-a removed
		crossZoomStack: []CrossProjectZoomEntry{
			{TaskID: sharedUUID, ProjectID: "proj-a", CursorIndex: 3},
		},
	}
	m2.cursor = 3

	m2.validateCrossZoomStack()

	if len(m2.crossZoomStack) != 0 {
		t.Errorf("stack should be cleared when scoped project is absent, even if UUID exists elsewhere: got %d entries", len(m2.crossZoomStack))
	}
	if m2.cursor != 0 {
		t.Errorf("cursor should reset to 0 after stack cleared: got %d", m2.cursor)
	}
}

// TestCrossProjectStatePreservationAfterReload verifies that activePerspective,
// collapsedSections, and cursor position survive a cross-project reload cycle.
// After handleCrossProjectsLoaded processes new project data, activePerspective
// and collapsedSections must remain exactly as set, and the cursor must fall
// within the valid index range for the rebuilt row list.
func TestCrossProjectStatePreservationAfterReload(t *testing.T) {
	const collapsedSection = models.SectionNext

	proj := &models.Project{
		ID:   "proj-reload",
		Name: "Reload",
		Tasks: []*models.Task{
			makeTask("r1", "Next task 1", models.SectionNext),
			makeTask("r2", "Next task 2", models.SectionNext),
			makeTask("r3", "Inbox task", models.SectionInbox),
		},
	}
	proj.BuildTaskTree()

	m := &OutlinerModel{
		collapsedSections: map[string]bool{collapsedSection: true},
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 3,
		crossLoadGen:      0, // generation 0 — msg must match
	}
	m.cursor = 1

	// New project data arrives via a completed async load (same generation).
	newProj := &models.Project{
		ID:   "proj-reload-2",
		Name: "Reload2",
		Tasks: []*models.Task{
			makeTask("n1", "New task", models.SectionNext),
		},
	}
	newProj.BuildTaskTree()

	msg := crossProjectsLoadedMsg{
		projects:   []*models.Project{newProj},
		generation: 0,
	}

	m.handleCrossProjectsLoaded(msg) //nolint:errcheck — returns tea.Cmd, not needed here

	// activePerspective must be unchanged.
	if m.activePerspective != 3 {
		t.Errorf("activePerspective: want 3, got %d", m.activePerspective)
	}

	// collapsedSections must be unchanged.
	if !m.collapsedSections[collapsedSection] {
		t.Errorf("collapsedSections[%q] should still be true after reload", collapsedSection)
	}

	// cursor must be within valid bounds: [0, len(rows)-1] or 0 when rows is empty.
	rowCount := len(m.rows)
	if rowCount == 0 {
		if m.cursor != 0 {
			t.Errorf("cursor must be 0 when no rows exist, got %d", m.cursor)
		}
	} else {
		if m.cursor < 0 || m.cursor >= rowCount {
			t.Errorf("cursor %d is out of bounds for %d rows", m.cursor, rowCount)
		}
	}
}

// TestProjectIDPopulationInvariant verifies the core ProjectID invariant across
// both row-building functions:
//   - Every task row carries a non-empty ProjectID that matches its source project.
//   - Every IsSectionHeader row has an empty ProjectID.
//   - Every IsSeparator row has an empty ProjectID.
//
// This is tested for both buildCrossProjectRowsWithSectionHeaders (GTD-section
// mode) and buildProjectGroupedRows (project-grouped mode).
func TestProjectIDPopulationInvariant(t *testing.T) {
	groups := []projectTasksGroup{
		{
			projectID: "proj-alpha",
			tasks: []*models.Task{
				makeTask("ta1", "Alpha active", ""),
				makeTask("ta2", "Alpha next", models.SectionNext),
			},
		},
		{
			projectID: "proj-beta",
			tasks: []*models.Task{
				makeTask("tb1", "Beta next", models.SectionNext),
				makeTask("tb2", "Beta inbox", models.SectionInbox),
			},
		},
	}

	t.Run("buildCrossProjectRowsWithSectionHeaders", func(t *testing.T) {
		m := minimalOutliner()
		m.buildCrossProjectRowsWithSectionHeaders(groups)

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows")
		}

		for i, row := range m.rows {
			switch {
			case row.IsSectionHeader:
				if row.ProjectID != "" {
					t.Errorf("row[%d] IsSectionHeader but has non-empty ProjectID %q", i, row.ProjectID)
				}
			case row.IsSeparator:
				if row.ProjectID != "" {
					t.Errorf("row[%d] IsSeparator but has non-empty ProjectID %q", i, row.ProjectID)
				}
			default:
				// task row
				if row.ProjectID == "" {
					title := ""
					if row.task != nil {
						title = row.task.Title
					}
					t.Errorf("row[%d] task row %q has empty ProjectID", i, title)
				}
				// ProjectID must match the source project for this task.
				if row.task != nil {
					var wantProjectID string
					for _, g := range groups {
						for _, task := range g.tasks {
							if task.ID == row.task.ID {
								wantProjectID = g.projectID
							}
						}
					}
					if wantProjectID != "" && row.ProjectID != wantProjectID {
						t.Errorf("row[%d] task %q: ProjectID want %q, got %q", i, row.task.Title, wantProjectID, row.ProjectID)
					}
				}
			}
		}
	})

	t.Run("buildProjectGroupedRows", func(t *testing.T) {
		m := minimalOutliner()
		m.buildProjectGroupedRows(groups)

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows")
		}

		for i, row := range m.rows {
			switch {
			case row.IsSectionHeader:
				if row.ProjectID != "" {
					t.Errorf("row[%d] IsSectionHeader but has non-empty ProjectID %q", i, row.ProjectID)
				}
			case row.IsSeparator:
				if row.ProjectID != "" {
					t.Errorf("row[%d] IsSeparator but has non-empty ProjectID %q", i, row.ProjectID)
				}
			default:
				// task row
				if row.ProjectID == "" {
					title := ""
					if row.task != nil {
						title = row.task.Title
					}
					t.Errorf("row[%d] task row %q has empty ProjectID", i, title)
				}
				// ProjectID must match the source project for this task.
				if row.task != nil {
					var wantProjectID string
					for _, g := range groups {
						for _, task := range g.tasks {
							if task.ID == row.task.ID {
								wantProjectID = g.projectID
							}
						}
					}
					if wantProjectID != "" && row.ProjectID != wantProjectID {
						t.Errorf("row[%d] task %q: ProjectID want %q, got %q", i, row.task.Title, wantProjectID, row.ProjectID)
					}
				}
			}
		}
	})
}

// TestCrossProjectRebuildRowsProjectIDInvariant verifies that every task row
// produced by crossProjectRebuildRows carries a ProjectID that matches the
// project owning that task, and that IsSectionHeader and IsSeparator rows
// always have an empty ProjectID.
func TestCrossProjectRebuildRowsProjectIDInvariant(t *testing.T) {
	projAlpha := &models.Project{
		ID:   "proj-alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			makeTask("ta1", "Alpha task 1", ""),
			makeTask("ta2", "Alpha task 2", models.SectionNext),
		},
	}
	projAlpha.BuildTaskTree()

	projBeta := &models.Project{
		ID:   "proj-beta",
		Name: "Beta",
		Tasks: []*models.Task{
			makeTask("tb1", "Beta task 1", models.SectionInbox),
			makeTask("tb2", "Beta task 2", models.SectionNext),
		},
	}
	projBeta.BuildTaskTree()

	// Build a task-ID → project-ID lookup to assert each row's ProjectID.
	taskOwner := make(map[string]string)
	for _, proj := range []*models.Project{projAlpha, projBeta} {
		for _, task := range proj.Tasks {
			taskOwner[task.ID] = proj.ID
		}
	}

	m := &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1, // All perspective — shows every task
		loadedProjects:    []*models.Project{projAlpha, projBeta},
	}

	m.crossProjectRebuildRows()

	if len(m.rows) == 0 {
		t.Fatal("crossProjectRebuildRows produced no rows")
	}

	for i, row := range m.rows {
		switch {
		case row.IsSectionHeader:
			if row.ProjectID != "" {
				t.Errorf("row[%d] IsSectionHeader but has non-empty ProjectID %q", i, row.ProjectID)
			}
		case row.IsSeparator:
			if row.ProjectID != "" {
				t.Errorf("row[%d] IsSeparator but has non-empty ProjectID %q", i, row.ProjectID)
			}
		default:
			// task row
			if row.ProjectID == "" {
				title := ""
				if row.task != nil {
					title = row.task.Title
				}
				t.Errorf("row[%d] task row %q has empty ProjectID", i, title)
			}
			if row.task != nil {
				wantProjectID := taskOwner[row.task.ID]
				if wantProjectID != "" && row.ProjectID != wantProjectID {
					t.Errorf("row[%d] task %q: ProjectID want %q, got %q",
						i, row.task.Title, wantProjectID, row.ProjectID)
				}
			}
		}
	}
}

// readCrossTestFile reads a file's contents for integration test assertions.
func readCrossTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
