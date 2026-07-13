package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// crossPriorityFixture builds two disk-backed projects with priority-eligible
// tasks under one data dir, wires the single-task coordinator over the same
// in-memory project pointers, enters cross-project Priority mode, rebuilds rows,
// and returns the model along with handles to each project and the cursor index
// of the first task row (skipping the leading section header). Migrated field
// mutations commit through the coordinator against these same pointers, so an
// assertion can run the returned command and read the in-memory task back.
func crossPriorityFixture(t *testing.T) (m *OutlinerModel, alpha, beta *models.Project, alphaTaskID, betaTaskID string, firstTaskRow int) {
	t.Helper()

	dataDir := t.TempDir()
	alphaDir := filepath.Join(dataDir, "projects", "alpha-proj")
	betaDir := filepath.Join(dataDir, "projects", "beta-proj")

	alphaTask, err := models.NewTask("alpha-task-1", 1, models.TaskTypeTask, "Alpha task")
	if err != nil {
		t.Fatalf("NewTask alpha: %v", err)
	}
	alphaTask.Priority = 3
	alphaTask.SetSection(models.SectionNext)
	today := time.Now()
	alphaTask.DueDate = &today
	alpha = &models.Project{ID: "alpha-proj", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = []*models.Task{alphaTask}
	alpha.NextShortID = 2
	alpha.BuildTaskTree()
	if saveErr := storage.SaveProject(alpha); saveErr != nil {
		t.Fatalf("save alpha: %v", saveErr)
	}

	betaTask, err := models.NewTask("beta-task-1", 1, models.TaskTypeTask, "Beta task")
	if err != nil {
		t.Fatalf("NewTask beta: %v", err)
	}
	betaTask.Priority = 3
	betaTask.SetSection(models.SectionNext)
	betaTask.DueDate = &today
	beta = &models.Project{ID: "beta-proj", Name: "Beta", Path: betaDir}
	beta.Tasks = []*models.Task{betaTask}
	beta.NextShortID = 2
	beta.BuildTaskTree()
	if saveErr := storage.SaveProject(beta); saveErr != nil {
		t.Fatalf("save beta: %v", saveErr)
	}

	if loadErr := storage.LoadProject(alpha); loadErr != nil {
		t.Fatalf("load alpha: %v", loadErr)
	}
	if loadErr := storage.LoadProject(beta); loadErr != nil {
		t.Fatalf("load beta: %v", loadErr)
	}

	m = &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{alpha, beta},
		filterProjects:    make(map[string]bool),
	}

	// Wire the single-task coordinator over the same in-memory project pointers,
	// so migrated priority-mode mutations commit through the application layer and
	// the ledger while remaining observable on alpha/beta directly.
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, alpha, beta)
	svc := app.NewService(repo, nil, nil)
	coords := newSecondaryCoordinators(svc, nil)
	m.taskItem = coords.TaskItem
	m.bulk = coords.Bulk

	m.enterCrossPriorityMode()
	m.crossProjectRebuildRows()

	firstTaskRow = -1
	for i, r := range m.rows {
		if r.isTaskRow() && r.task != nil {
			firstTaskRow = i
			break
		}
	}
	if firstTaskRow < 0 {
		t.Fatal("priority fixture: no task row produced after rebuild")
	}

	return m, alpha, beta, "alpha-task-1", "beta-task-1", firstTaskRow
}

func TestCrossPriorityEnterOnGroupHeaderTogglesCollapse(t *testing.T) {
	m, _, _, _, _, _ := crossPriorityFixture(t)

	headerIdx := -1
	for i, r := range m.rows {
		if r.IsSectionHeader {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("no priority group header found")
	}
	m.cursor = headerIdx

	countTaskRows := func() int {
		n := 0
		for _, r := range m.rows {
			if r.isTaskRow() && r.task != nil {
				n++
			}
		}
		return n
	}
	before := countTaskRows()
	if before == 0 {
		t.Fatal("expected at least one task row before collapse")
	}

	_, _, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false on priority group header")
	}
	if got := countTaskRows(); got >= before {
		t.Errorf("after collapse: want fewer than %d task rows, got %d", before, got)
	}

	// Re-locate the header (rows may have shifted after collapse) and re-toggle.
	for i, r := range m.rows {
		if r.IsSectionHeader {
			m.cursor = i
			break
		}
	}
	_, _, handled = m.crossHandleEnterKey()
	if !handled {
		t.Fatal("crossHandleEnterKey returned handled=false on second toggle")
	}
	if got := countTaskRows(); got != before {
		t.Errorf("after expand: want %d task rows, got %d", before, got)
	}
}

func TestCrossPriorityEnterOnLeafTaskOpensDetail(t *testing.T) {
	m, alpha, _, _, _, taskRowIdx := crossPriorityFixture(t)
	m.cursor = taskRowIdx
	row := m.rows[m.cursor]

	_, cmd, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("expected handled=true in cross-project mode")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to emit ShowTaskDetailMsg")
	}
	msg, ok := cmd().(ShowTaskDetailMsg)
	if !ok {
		t.Fatalf("expected ShowTaskDetailMsg, got %T", cmd())
	}
	if msg.Task != row.task {
		t.Errorf("ShowTaskDetailMsg.Task: want %v, got %v", row.task, msg.Task)
	}
	// First task row in fixture order belongs to alpha (project display order).
	if msg.Project != alpha {
		t.Errorf("ShowTaskDetailMsg.Project: want %q, got %q", alpha.Name, msg.Project.Name)
	}
}

func TestCrossPriorityEnterOnParentTaskOpensDetailNotZoom(t *testing.T) {
	// Build a single project with a parent task containing a child. Both should
	// land in priority rows and Enter on the parent must open detail (not zoom).
	dir := t.TempDir()

	parentTask, err := models.NewTask("parent-1", 1, models.TaskTypeEpic, "Parent epic")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parentTask.Priority = 3
	parentTask.SetSection(models.SectionNext)
	today := time.Now()
	parentTask.DueDate = &today

	childTask, err := models.NewTask("child-1", 2, models.TaskTypeTask, "Child task")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	childTask.Priority = 3
	childTask.SetSection(models.SectionNext)
	childTask.DueDate = &today
	parentID := parentTask.ID
	childTask.ParentID = &parentID

	proj := &models.Project{ID: "proj-parent", Name: "Parent Proj", Path: dir}
	proj.Tasks = []*models.Task{parentTask, childTask}
	proj.NextShortID = 3
	proj.BuildTaskTree()
	if saveErr := storage.SaveProject(proj); saveErr != nil {
		t.Fatalf("save proj: %v", saveErr)
	}
	if loadErr := storage.LoadProject(proj); loadErr != nil {
		t.Fatalf("load proj: %v", loadErr)
	}

	m := &OutlinerModel{
		PrimaryViewBundle: PrimaryViewBundle{
			projectService: NewProjectService(),
			bulkOp:         NewBulkOperationComponent(),
			quickEdit:      NewQuickEditComponent(),
		},
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filterProjects:    make(map[string]bool),
	}
	m.enterCrossPriorityMode()
	m.crossProjectRebuildRows()

	// Find the parent task row by ID.
	parentRowIdx := -1
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == parentTask.ID {
			parentRowIdx = i
			break
		}
	}
	if parentRowIdx < 0 {
		t.Fatal("parent task not present in priority rows")
	}
	if len(parentTask.Children) == 0 {
		t.Fatal("expected parent task to have children after BuildTaskTree")
	}
	m.cursor = parentRowIdx
	zoomBefore := len(m.crossZoomStack)

	_, cmd, handled := m.crossHandleEnterKey()
	if !handled {
		t.Fatal("expected handled=true in cross-project mode")
	}
	if len(m.crossZoomStack) != zoomBefore {
		t.Errorf("priority mode Enter on parent must not push zoom stack: before=%d after=%d", zoomBefore, len(m.crossZoomStack))
	}
	if cmd == nil {
		t.Fatal("expected ShowTaskDetailMsg cmd for parent row in priority mode")
	}
	msg, ok := cmd().(ShowTaskDetailMsg)
	if !ok {
		t.Fatalf("expected ShowTaskDetailMsg, got %T", cmd())
	}
	if msg.Task.ID != parentTask.ID {
		t.Errorf("ShowTaskDetailMsg.Task.ID: want %q, got %q", parentTask.ID, msg.Task.ID)
	}
}

func TestCrossPriorityTaskRowsCarryProjectID(t *testing.T) {
	m, alpha, beta, alphaTaskID, betaTaskID, _ := crossPriorityFixture(t)

	got := make(map[string]string)
	for _, row := range m.rows {
		if row.task != nil {
			got[row.task.ID] = row.ProjectID
		}
	}
	if got[alphaTaskID] != alpha.ID {
		t.Errorf("%s ProjectID: want %q, got %q", alphaTaskID, alpha.ID, got[alphaTaskID])
	}
	if got[betaTaskID] != beta.ID {
		t.Errorf("%s ProjectID: want %q, got %q", betaTaskID, beta.ID, got[betaTaskID])
	}
}

func TestCrossPriorityMutationSavesToOwningProject(t *testing.T) {
	m, _, beta, _, betaTaskID, _ := crossPriorityFixture(t)

	// Move cursor to the beta task row.
	betaRowIdx := -1
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == betaTaskID {
			betaRowIdx = i
			break
		}
	}
	if betaRowIdx < 0 {
		t.Fatal("beta task not present in priority rows")
	}
	m.cursor = betaRowIdx

	cmd, ok := m.crossHandlePriorityMutation("1")
	if !ok {
		t.Fatal("expected ok=true for priority key '1'")
	}
	runResult(t, cmd)

	task := beta.GetTaskByID(betaTaskID)
	if task == nil {
		t.Fatal("beta task not found after mutation")
	}
	if task.Priority != 1 {
		t.Errorf("priority: want 1, got %d", task.Priority)
	}
}

func TestCrossPriorityDueDateMutationRecategorizes(t *testing.T) {
	m, _, _, _, betaTaskID, _ := crossPriorityFixture(t)

	// Cursor on the beta task (initially "Today").
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == betaTaskID {
			m.cursor = i
			break
		}
	}

	// Set due date to "next month" (key "n" → ~30 days).
	cmd, ok := m.crossHandleDueDateMutation("n")
	if !ok {
		t.Fatal("expected ok=true for due-date key 'n'")
	}
	runResult(t, cmd)

	// The migrated handler commits asynchronously and leaves the row rebuild to
	// the shell; rebuild explicitly here, then assert the task lives under LATER.
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	currentGroup := groupHeaderForTask(m, betaTaskID)
	if currentGroup == "" {
		t.Fatal("beta task no longer present in priority rows after due-date change")
	}
	want := priorityGroupHeaderLabel(PriorityGroupLater)
	if currentGroup != want {
		t.Errorf("post-mutation group: want %q, got %q", want, currentGroup)
	}
}

func TestCrossPriorityBlockedStatusMovesToWaitingGroup(t *testing.T) {
	m, _, beta, _, betaTaskID, _ := crossPriorityFixture(t)

	// Blocked/Waiting starts collapsed; expand so we can confirm group placement.
	m.crossPriorityCollapsedGroups[PriorityGroupWaitingOn] = false

	// Cursor on the beta task.
	for i, r := range m.rows {
		if r.task != nil && r.task.ID == betaTaskID {
			m.cursor = i
			break
		}
	}

	_, cmd, handled := m.crossHandleStateMutation(models.StateWaiting)
	if !handled {
		t.Fatal("expected status mutation to be handled in cross-project mode")
	}
	// The migrated handler commits the transition asynchronously through the
	// single-task coordinator (blocked → waiting state); run the command to apply
	// it against the shared in-memory pointers.
	runResult(t, cmd)

	task := beta.GetTaskByID(betaTaskID)
	if task == nil {
		t.Fatal("beta task not found after status mutation")
	}
	if models.LegacyStatusForState(task.State()) != models.StatusBlocked {
		t.Errorf("status: want %q, got %q", models.StatusBlocked, models.LegacyStatusForState(task.State()))
	}

	// The migrated handler commits and leaves the row rebuild to the shell;
	// trigger the rebuild here to verify the priority builder re-categorizes the
	// blocked task.
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	group := groupHeaderForTask(m, betaTaskID)
	want := priorityGroupHeaderLabel(PriorityGroupWaitingOn)
	if group != want {
		t.Errorf("blocked task group: want %q, got %q", want, group)
	}
}

func TestCrossPrioritySearchFiltersRows(t *testing.T) {
	m, _, _, alphaTaskID, betaTaskID, _ := crossPriorityFixture(t)

	m.SetSearchQuery("Alpha")
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	seen := make(map[string]bool)
	for _, r := range m.rows {
		if r.task != nil {
			seen[r.task.ID] = true
		}
	}
	if !seen[alphaTaskID] {
		t.Error("expected alpha task to remain visible under search 'Alpha'")
	}
	if seen[betaTaskID] {
		t.Error("expected beta task to be filtered out under search 'Alpha'")
	}
}

func TestCrossPriorityProjectFilterScopesRows(t *testing.T) {
	m, alpha, _, alphaTaskID, betaTaskID, _ := crossPriorityFixture(t)

	m.filterProjects = map[string]bool{alpha.ID: true}
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	seen := make(map[string]bool)
	for _, r := range m.rows {
		if r.task != nil {
			seen[r.task.ID] = true
		}
	}
	if !seen[alphaTaskID] {
		t.Error("expected alpha task visible when project filter includes alpha")
	}
	if seen[betaTaskID] {
		t.Error("expected beta task filtered out when project filter excludes beta")
	}
}

func TestCrossPriorityActionableFilterScopesRows(t *testing.T) {
	m, alpha, beta, alphaTaskID, betaTaskID, _ := crossPriorityFixture(t)

	// Move alpha to an actionable-friendly section (empty == "active"); the
	// actionable filter explicitly excludes Next/Inbox/Waiting/Someday/Review/
	// Standby. Mark beta as blocked so the actionable filter drops it.
	alphaTask := alpha.GetTaskByID(alphaTaskID)
	if alphaTask == nil {
		t.Fatal("alpha task missing")
	}
	// Use the syncing mutators (not raw field writes): the v2 writer serializes
	// the derived WorkflowState, so a raw Section/Status write without a resync
	// would persist the stale prior state.
	alphaTask.SetSection("")
	if err := storage.SaveProject(alpha); err != nil {
		t.Fatalf("save alpha after section change: %v", err)
	}
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("reload alpha after section change: %v", err)
	}

	betaTask := beta.GetTaskByID(betaTaskID)
	if betaTask == nil {
		t.Fatal("beta task missing")
	}
	betaTask.Block()
	if err := storage.SaveProject(beta); err != nil {
		t.Fatalf("save beta after blocking: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("reload beta after blocking: %v", err)
	}

	m.filter.Actionable = true
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	seen := make(map[string]bool)
	for _, r := range m.rows {
		if r.task != nil {
			seen[r.task.ID] = true
		}
	}
	if !seen[alphaTaskID] {
		t.Error("expected alpha task (actionable) to remain visible")
	}
	if seen[betaTaskID] {
		t.Error("expected blocked beta task to be filtered out by actionable filter")
	}
}

func TestCrossPriorityMultiSelectSurvivesRebuild(t *testing.T) {
	m, _, _, alphaTaskID, betaTaskID, _ := crossPriorityFixture(t)

	m.multiSelectMode = true
	m.selectedTasks[alphaTaskID] = true
	m.selectedTasks[betaTaskID] = true

	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	if !m.multiSelectMode {
		t.Error("multiSelectMode should persist across rebuild")
	}
	if !m.selectedTasks[alphaTaskID] {
		t.Error("alpha selection should survive rebuild because the row remains visible")
	}
	if !m.selectedTasks[betaTaskID] {
		t.Error("beta selection should survive rebuild because the row remains visible")
	}
}

// TestCrossPriorityPinnedTagDoesNotDragNonMatchingParent guards the
// Priority-mode predicate against filterTaskTree's context-preserving rule.
// Before the fix, a non-matching eligible parent (no `#backend` tag) was
// retained by the chained filterTaskTree calls because one of its children
// matched, and buildCrossPriorityRows then bucketed the parent itself
// because IsEligibleForPriority(parent) was true. With the per-task
// predicate, only the child appears as a flat Priority row while the parent
// stays available as ParentTitle context on the child entry.
func TestCrossPriorityPinnedTagDoesNotDragNonMatchingParent(t *testing.T) {
	dir := t.TempDir()

	parentTask, err := models.NewTask("parent-epic", 1, models.TaskTypeEpic, "Parent epic (no tag)")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parentTask.Priority = 3
	parentTask.SetSection(models.SectionNext)
	today := time.Now()
	parentTask.DueDate = &today

	childTask, err := models.NewTask("backend-child", 2, models.TaskTypeTask, "Backend child")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	childTask.Priority = 3
	childTask.SetSection(models.SectionNext)
	childTask.DueDate = &today
	childTask.Tags = []string{"backend"}
	parentID := parentTask.ID
	childTask.ParentID = &parentID

	proj := &models.Project{ID: "proj-pinned-parent", Name: "PinnedParent", Path: dir}
	proj.Tasks = []*models.Task{parentTask, childTask}
	proj.NextShortID = 3
	proj.BuildTaskTree()
	if saveErr := storage.SaveProject(proj); saveErr != nil {
		t.Fatalf("save proj: %v", saveErr)
	}
	if loadErr := storage.LoadProject(proj); loadErr != nil {
		t.Fatalf("load proj: %v", loadErr)
	}

	cfg := config.NewConfig()
	cfg.GlobalPinnedTags = []string{"backend"}
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: PerspectiveAll,
		loadedProjects:    []*models.Project{proj},
		filterProjects:    make(map[string]bool),
		filter:            NewFilterState(),
		pinnedTagTab:      make(map[string]int),
		sort:              NewSortState(),
	}
	m.setPinnedTagTabIndex(1) // Select the #backend tab.
	m.enterCrossPriorityMode()
	m.crossProjectRebuildRows()

	if got := m.activePinnedTagFilter(); got != "backend" {
		t.Fatalf("activePinnedTagFilter() = %q, want backend", got)
	}

	seen := make(map[string]bool)
	var childRow *outlinerRow
	for i := range m.rows {
		row := &m.rows[i]
		if row.task != nil {
			seen[row.task.ID] = true
			if row.task.ID == childTask.ID {
				childRow = row
			}
		}
	}
	if seen[parentTask.ID] {
		t.Errorf("parent without #backend must not appear as a Priority row when the #backend pinned tag is active")
	}
	if !seen[childTask.ID] {
		t.Errorf("child with #backend should appear as a flat Priority row, got rows=%v", seen)
	}

	// Parent context is still computed from the project graph (not from the
	// row's neighbors), so dropping the parent from the rows must not also
	// drop the cached ParentTitle on the child's PriorityTask entry.
	predicate := m.crossPriorityRowPredicate(proj)
	if predicate(parentTask) {
		t.Error("predicate must reject the parent without #backend")
	}
	if !predicate(childTask) {
		t.Fatal("predicate must accept the child with #backend")
	}
	if childRow == nil {
		t.Fatal("child row should be present")
	}
	if childRow.priorityTask == nil {
		t.Fatal("child row should retain its PriorityTask metadata")
	}
	if childRow.priorityTask.Task != childRow.task {
		t.Errorf("PriorityTask.Task: want emitted row task pointer, got %+v", childRow.priorityTask.Task)
	}
	if childRow.priorityTask.ParentTitle != parentTask.Title {
		t.Errorf("PriorityTask.ParentTitle: want %q, got %q", parentTask.Title, childRow.priorityTask.ParentTitle)
	}
}

// groupHeaderForTask returns the section header label that precedes the task
// row with the given task ID, or "" when the task is not present in m.rows.
func groupHeaderForTask(m *OutlinerModel, taskID string) string {
	lastHeader := ""
	for _, row := range m.rows {
		if row.IsSectionHeader {
			lastHeader = row.SectionName
			continue
		}
		if row.task != nil && row.task.ID == taskID {
			return lastHeader
		}
	}
	return ""
}
