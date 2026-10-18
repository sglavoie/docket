package outliner

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// dependencyTestIndex builds the reference index the render path reads. A test
// builds it explicitly for the same reason the model caches one: the index is
// the population's reading of the graph, so naming the population is naming the
// answer.
func dependencyTestIndex(projects ...*models.Project) *app.DependencyIndex {
	return app.NewDependencyIndex(projects)
}

// dependencyPresentationFixture seeds a referenced project and a referring one
// so both directions of the detail, and the unavailable fallback, have a real
// subject. beta is deliberately returned separately: a caller that omits it from
// the loaded population is exercising exactly the "project not loaded" case.
func dependencyPresentationFixture(t *testing.T) (*models.Project, *models.Project) {
	t.Helper()
	source := renderFixtureTask(t, "source", 1, "Wire the importer", models.StateDoing, 0, models.StateDoing)
	sibling := renderFixtureTask(t, "sibling", 2, "Agree the schema", models.StateBacklog, 0, models.StateBacklog)
	remote := renderFixtureTask(t, "remote", 1, "Publish the contract", models.StateWaiting, 0, models.StateWaiting)
	alpha := renderFixtureProject(t, "proj-a", "alpha", "Alpha", []*models.Task{source, sibling})
	beta := renderFixtureProject(t, "proj-b", "beta", "Beta", []*models.Task{remote})
	source.Dependencies = []models.TaskReference{
		{ProjectID: "proj-a", TaskID: "sibling"},
		{ProjectID: "proj-b", TaskID: "remote"},
	}
	return alpha, beta
}

// menuBarCommandContext is the launch context: the Outliner presentation with
// the simplified grammar and a cursor that names nothing.
func menuBarCommandContext() operation.CommandContext {
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	return ctx
}

// cursorTaskID returns the task ID under the cursor, or "" when the cursor does
// not name a task row.
func cursorTaskID(m *OutlinerModel) string {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return ""
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return ""
	}
	return row.Task.ID
}

func requireCursorTask(t *testing.T, outliner *OutlinerModel, want taskref.TaskIdentity) {
	t.Helper()
	if got := outliner.CurrentTaskIdentity(); got != want {
		t.Fatalf("cursor task = %+v, want %+v (cursor=%d)", got, want, outliner.Cursor)
	}
}

func requireCursorRow(t *testing.T, outliner *OutlinerModel, taskID string) {
	t.Helper()
	if outliner.Cursor < 0 || outliner.Cursor >= len(outliner.Rows) {
		t.Fatalf("cursor %d is outside %d rows", outliner.Cursor, len(outliner.Rows))
	}
	row := outliner.Rows[outliner.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		t.Fatalf("cursor landed on a non-task row %+v", row)
	}
	if row.Task.ID != taskID {
		t.Fatalf("cursor task = %q, want %q", row.Task.ID, taskID)
	}
}

func cursorToTaskRow(t *testing.T, outliner *OutlinerModel, taskID string) {
	t.Helper()
	for i, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			outliner.Cursor = i
			return
		}
	}
	t.Fatalf("task %q has no task row", taskID)
}

// inboxModeOutliner builds the real Inbox row pipeline over the given
// projects. Every recovery test that wants the Inbox shape runs against these
// rows rather than a hand-rolled slice, so the [projectHeader][task...] group
// shape is the one the product renders.
func inboxModeOutliner(t *testing.T, projects ...*models.Project) *OutlinerModel {
	t.Helper()
	outliner, _ := primaryRowsOutliner(projects...)
	outliner.SelectOutlinerMode(chrome.ModeInbox)
	return outliner
}

func jumpHistoryFixture(taskIDs ...string) (*OutlinerModel, map[string]*models.Task) {
	const projectID = "alpha"
	tasks := make([]*models.Task, 0, len(taskIDs))
	byID := make(map[string]*models.Task, len(taskIDs))
	for _, taskID := range taskIDs {
		task := &models.Task{ID: taskID, Title: taskID, WorkflowState: models.StateBacklog}
		tasks = append(tasks, task)
		byID[taskID] = task
	}
	project := primaryRowsProject(projectID, tasks...)
	outliner, _ := primaryRowsOutliner(project)
	outliner.PrimaryMode = chrome.ModeAll
	setJumpHistoryRows(outliner, byID, taskIDs...)
	return outliner, byID
}

func setJumpHistoryRows(outliner *OutlinerModel, tasks map[string]*models.Task, taskIDs ...string) {
	outliner.Rows = outliner.Rows[:0]
	for _, taskID := range taskIDs {
		outliner.Rows = append(outliner.Rows, OutlinerRow{Task: tasks[taskID], ProjectID: "alpha"})
	}
}

func requireCurrentJumpTask(t *testing.T, outliner *OutlinerModel, want taskref.TaskIdentity) {
	t.Helper()
	if got := outliner.CurrentTaskIdentity(); got != want {
		t.Fatalf("current task = %+v, want %+v (cursor=%d)", got, want, outliner.Cursor)
	}
}

func visibleQualifiedTaskIDs(outliner *OutlinerModel) []string {
	var ids []string
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids = append(ids, row.ProjectID+"/"+row.Task.ID)
		}
	}
	return ids
}

// projectGroupOrder names the project groups a projection emitted, in render
// order. It reads the OutlinerRowGroup the rows carry rather than the headings
// above them: a focused projection's leading header groups nothing and folds nothing, so it carries no
// project half and is excluded by construction rather than by a row-kind test.
func projectGroupOrder(m *OutlinerModel) []string {
	groups := m.VisibleProjectGroups()
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ProjectID)
	}
	return ids
}

// projectGroupTaskCounts reads each project group's declared count off the
// header that opens it, expanded or folded. A merged singleton group declares
// none, so its count is the task rows it draws.
func projectGroupTaskCounts(m *OutlinerModel) map[string]int {
	counts := make(map[string]int)
	for _, row := range m.Rows {
		if row.IsProjectHeader && row.Group.ProjectGrouped() {
			counts[row.Group.ProjectID] += row.SectionCount
		}
		if row.LeadsMergedGroup {
			for _, member := range m.Rows {
				if member.IsTaskRow() && member.Group == row.Group {
					counts[row.Group.ProjectID]++
				}
			}
		}
	}
	return counts
}

// rowForTaskID finds one task row by the task it carries, so an assertion about
// that row survives any header the projection puts above it.
func rowForTaskID(m *OutlinerModel, taskID string) (OutlinerRow, bool) {
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			return row, true
		}
	}
	return OutlinerRow{}, false
}

// assertSoleFocusedProjectHeader is the whole scoped-header contract in one
// call: exactly one focused header, leading the projection, naming the focused
// project, carrying no fold key and no group count, and excluded from the task
// rows. label names the case under test so a table's failure says which row it
// came from.
func assertSoleFocusedProjectHeader(t *testing.T, m *OutlinerModel, projectID string, label any) {
	t.Helper()
	rows, indexes := focusedProjectHeaders(m)
	if len(rows) != 1 {
		t.Fatalf("%v focused headers = %d, want exactly one", label, len(rows))
	}
	header := rows[0]
	if indexes[0] != 0 {
		t.Fatalf("%v focused header sits at row %d, want the top of the projection", label, indexes[0])
	}
	if header.ProjectID != projectID {
		t.Fatalf("%v focused header project = %q, want %q", label, header.ProjectID, projectID)
	}
	if !header.IsProjectHeader {
		t.Fatalf("%v focused header is not a project header: %+v", label, header)
	}
	if header.IsCollapsibleHeader() || header.EffectiveCollapseKey() != "" {
		t.Fatalf("%v focused header owns a fold key %q", label, header.EffectiveCollapseKey())
	}
	if header.SectionCount != 0 || header.OverdueCount != 0 || header.TodayCount != 0 {
		t.Fatalf("%v focused header carries group counts: %+v", label, header)
	}
	if header.IsTaskRow() {
		t.Fatalf("%v focused header counts as a task row", label)
	}
}

// focusedProjectHeaders returns the focused-project header rows and the index
// each sits at, which is the assertion pair every scoped test needs: there is
// exactly one and it leads the projection.
func focusedProjectHeaders(m *OutlinerModel) ([]OutlinerRow, []int) {
	var rows []OutlinerRow
	var indexes []int
	for index, row := range m.Rows {
		if row.IsFocusedProjectHeader {
			rows = append(rows, row)
			indexes = append(indexes, index)
		}
	}
	return rows, indexes
}

func newGroupingFixture(t *testing.T, mode chrome.OutlinerModeID) groupingFixture {
	t.Helper()
	var alphaOne, alphaTwo, betaOne *models.Task
	switch mode {
	case chrome.ModeInbox:
		alphaOne = primaryRowsTask("alpha-one", models.StateInbox)
		alphaTwo = primaryRowsTask("alpha-two", models.StateInbox)
		betaOne = primaryRowsTask("beta-one", models.StateInbox)
	case chrome.ModeBacklog:
		alphaOne = primaryRowsTask("alpha-one", models.StateBacklog)
		alphaTwo = primaryRowsTask("alpha-two", models.StateBacklog)
		betaOne = primaryRowsTask("beta-one", models.StateBacklog)
	case chrome.ModeDue:
		alphaOne = groupingDueTask("alpha-one", -3)
		alphaTwo = groupingDueTask("alpha-two", -2)
		betaOne = groupingDueTask("beta-one", -1)
	case chrome.ModeRecent:
		alphaOne = recentTask("alpha-one", models.StateBacklog, 3)
		alphaTwo = recentTask("alpha-two", models.StateBacklog, 4)
		betaOne = recentTask("beta-one", models.StateBacklog, 5)
	default:
		t.Fatalf("mode %q already grouped before this contract; no fixture defined", mode)
		return groupingFixture{}
	}
	return groupingFixture{
		alpha: primaryRowsProject("alpha", alphaOne, alphaTwo),
		beta:  primaryRowsProject("beta", betaOne),
		tasks: []string{alphaOne.ID, alphaTwo.ID, betaOne.ID},
	}
}

// groupingFixture seeds two projects, alpha with two rows and beta with one, all
// visible in the named mode and ordered alpha-one, alpha-two, beta-one.
type groupingFixture struct {
	alpha *models.Project
	beta  *models.Project
	tasks []string
}

// groupingDueTask seeds an open task due dayOffset days from now.
func groupingDueTask(id string, dayOffset int) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	due := time.Now().AddDate(0, 0, dayOffset)
	task.DueDate = &due
	return task
}

func qualifiedVisibleTaskIDs(outliner *OutlinerModel) []taskref.TaskIdentity {
	result := make([]taskref.TaskIdentity, 0)
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil {
			result = append(result, taskref.TaskIdentity{ProjectID: row.ProjectID, TaskID: row.Task.ID})
		}
	}
	return result
}

func sectionHeaderNames(outliner *OutlinerModel) []string {
	names := make([]string, 0)
	for _, row := range outliner.Rows {
		if row.IsSectionHeader {
			names = append(names, row.SectionName)
		}
	}
	return names
}

func sectionHeaderIndex(t *testing.T, outliner *OutlinerModel, name string) int {
	t.Helper()
	for index, row := range outliner.Rows {
		if row.IsSectionHeader && row.SectionName == name {
			return index
		}
	}
	t.Fatalf("projection has no %q section header, sections are %v", name, sectionHeaderNames(outliner))
	return -1
}

func zoomIntoSection(t *testing.T, outliner *OutlinerModel, name string) {
	t.Helper()
	outliner.Cursor = sectionHeaderIndex(t, outliner, name)
	if !outliner.EnterSectionZoom(outliner.Rows[outliner.Cursor]) {
		t.Fatalf("zoom into %q was refused", name)
	}
}

func scopeTestProject(id, name string, taskIDs ...string) *models.Project {
	tasks := make([]*models.Task, 0, len(taskIDs))
	for i, taskID := range taskIDs {
		tasks = append(tasks, &models.Task{
			ID:            taskID,
			ShortID:       i + 1,
			Title:         taskID,
			WorkflowState: models.StateBacklog,
		})
	}
	project := &models.Project{
		ID:          id,
		Name:        name,
		Slug:        id,
		Tasks:       tasks,
		NextShortID: len(tasks) + 1,
	}
	project.BuildTaskTree()
	return project
}

func scopeTestOutliner(projects ...*models.Project) *OutlinerModel {
	runtime := testConfigWithProjects(projects...)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.Population.Projects = projects
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	return outliner
}

func findPlanningTaskRow(outliner *OutlinerModel, taskID string) (OutlinerRow, bool) {
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			return row, true
		}
	}
	return OutlinerRow{}, false
}

func visibleOutlinerTaskIDs(outliner *OutlinerModel) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids[row.Task.ID] = true
		}
	}
	return ids
}

func primaryRowsTask(id string, state models.WorkflowState) *models.Task {
	return &models.Task{
		ID:            id,
		Title:         id,
		WorkflowState: state,
		UpdatedAt:     time.Now(),
	}
}

func primaryRowsProject(id string, tasks ...*models.Task) *models.Project {
	project := &models.Project{ID: id, Name: id, Slug: id, Tasks: tasks}
	project.BuildTaskTree()
	return project
}

func primaryRowsOutliner(projects ...*models.Project) (*OutlinerModel, *config.RuntimeState) {
	runtime := testConfigWithProjects(projects...)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.Population.Projects = projects
	return outliner, runtime
}

// relationFixture seeds the whole reference shape the picker has to describe:
// a same-project target, a cross-project one, a locator into a project nothing
// loaded, and a reciprocal pair, so both directions and both refusals have a
// real subject.
func relationFixture(t *testing.T) (*models.Project, *models.Project) {
	t.Helper()
	source := renderFixtureTask(t, "source", 1, "Wire the importer", models.StateDoing, 0, models.StateDoing)
	sibling := renderFixtureTask(t, "sibling", 2, "Agree the schema", models.StateBacklog, 2, models.StateBacklog)
	remote := renderFixtureTask(t, "remote", 1, "Publish the contract", models.StateWaiting, 0, models.StateWaiting)
	closed := renderFixtureTask(t, "closed", 2, "Retire the shim", models.StateDone, 0, models.StateDone)
	alpha := renderFixtureProject(t, "proj-a", "alpha", "Alpha", []*models.Task{source, sibling})
	beta := renderFixtureProject(t, "proj-b", "beta", "Beta", []*models.Task{remote, closed})
	source.Dependencies = []models.TaskReference{
		{ProjectID: "proj-a", TaskID: "sibling"},
		{ProjectID: "proj-b", TaskID: "remote"},
		{ProjectID: "proj-z", TaskID: "ghost"},
	}
	// The reciprocal half: remote names source back, so the pair resolves to the
	// same locator in both directions.
	remote.Dependencies = []models.TaskReference{{ProjectID: "proj-a", TaskID: "source"}}
	closed.Dependencies = []models.TaskReference{{ProjectID: "proj-a", TaskID: "source"}}
	return alpha, beta
}
