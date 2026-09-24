package outliner

import (
	"fmt"
	"go/ast"
	"go/token"
	"image/color"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
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
// header that opens it, expanded or folded.
func projectGroupTaskCounts(m *OutlinerModel) map[string]int {
	counts := make(map[string]int)
	for _, row := range m.Rows {
		if row.IsProjectHeader && row.Group.ProjectGrouped() {
			counts[row.Group.ProjectID] += row.SectionCount
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

// searchACOutliner builds a scoped Outliner already in search mode, which is the
// only state the suggestion pane can exist in.
func searchACOutliner(projects ...*models.Project) *OutlinerModel {
	outliner := scopeTestOutliner(projects...)
	outliner.SetSize(120, 40)
	outliner.ActivateSearch()
	return outliner
}

// typeSearchAC puts a whole query in the box with the caret at its end, which is
// the ordinary typing case.
func typeSearchAC(outliner *OutlinerModel, value string) {
	typeSearchACAt(outliner, value, utf8.RuneCountInString(value))
}

func searchACValues(outliner *OutlinerModel) []string {
	values := make([]string, 0, len(outliner.Search.Autocomplete.Ac.Items))
	for _, item := range outliner.Search.Autocomplete.Ac.Items {
		values = append(values, item.Value)
	}
	return values
}

func searchACTask(id string, tags []string, context string) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.Title = id
	task.Tags = tags
	task.Context = context
	return task
}

// typeSearchACAt is the mid-query case: the caret lands on a rune index inside a
// settled query rather than after it.
func typeSearchACAt(outliner *OutlinerModel, value string, runeCursor int) {
	outliner.SetSearchInputValue(value)
	outliner.SetSearchInputCursor(runeCursor)
	outliner.RefreshSearchAutocomplete()
}

// cueTask is a bland task: nothing in it renders a `·` of its own, so any middle
// dot in the row came from the match cue.
func cueTask() *models.Task {
	return &models.Task{
		ID:            "cue",
		ShortID:       1,
		Title:         "Untitled row",
		WorkflowState: models.StateBacklog,
	}
}

// searchGateProjects builds one two-project scope whose task order is the source
// order a sigil-only query must preserve.
func searchGateProjects() (*models.Project, *models.Project) {
	today := time.Now()
	later := today.AddDate(0, 0, 3)

	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Title = "Pin candidate"
	pinned.Pinned = true
	pinned.Tags = []string{"backend"}
	priorityOne := primaryRowsTask("priority-one", models.StateBacklog)
	priorityOne.Title = "Ship auth"
	priorityOne.Priority = models.PriorityHigh
	priorityOne.DueDate = &today
	contexted := primaryRowsTask("contexted", models.StateBacklog)
	contexted.Title = "Client work"
	contexted.Context = "client-acme"
	contexted.DueDate = &later
	plannedToday := primaryRowsTask("planned-today", models.StateBacklog)
	plannedToday.Title = "Planned today"
	plannedNext := primaryRowsTask("planned-next", models.StateBacklog)
	plannedNext.Title = "Planned next"
	finished := primaryRowsTask("finished", models.StateDone)
	finished.Title = "Old auth work"
	alpha := primaryRowsProject("alpha", pinned, priorityOne, contexted, plannedToday, plannedNext, finished)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta auth"
	betaTask.Tags = []string{"backend"}
	beta := primaryRowsProject("beta", betaTask)

	return alpha, beta
}

func searchGateOutliner() *OutlinerModel {
	alpha, beta := searchGateProjects()
	outliner := scopeTestOutliner(alpha, beta)
	outliner.Runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-today"},
	})
	outliner.Runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-next"},
	})
	return outliner
}

// searchHistoryOutliner builds a scoped Outliner with the slash input open,
// which is the only state the walk exists in.
func searchHistoryOutliner() *OutlinerModel {
	outliner := scopeTestOutliner(scopeTestProject("work", "Work", "alpha", "beta"))
	outliner.SetSize(120, 40)
	outliner.ActivateSearch()
	return outliner
}

func guidanceHintLabels(spec chrome.GuidanceSpec) string {
	labels := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		labels = append(labels, strings.TrimSpace(hint.Key+" "+hint.Label))
	}
	return strings.Join(labels, " | ")
}

func noteIndexRow(location app.NoteLocation, label, title, filename string) notes.LocatedNoteRow {
	return notes.LocatedNoteRow{
		Note:       &models.Note{Title: title, Filename: filename},
		Location:   location,
		OwnerLabel: label,
	}
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

// searchRankedList holds a query and shows it as the flat ranked list. The tier,
// score, and seating assertions are all about that presentation; a query alone
// narrows the mode's own lanes instead, which the D7 tests cover separately.
func searchRankedList(outliner *OutlinerModel, query string) {
	outliner.SetSearchQuery(query)
	if !outliner.ShowsRankedList() {
		outliner.ToggleRankedList()
	}
}

func searchVisibleTaskIDs(outliner *OutlinerModel) []string {
	result := make([]string, 0)
	for _, identity := range qualifiedVisibleTaskIDs(outliner) {
		result = append(result, identity.TaskID)
	}
	return result
}

// recentTask seeds a task whose last activity is dayOffset days in the past.
func recentTask(id string, state models.WorkflowState, dayOffset int) *models.Task {
	task := primaryRowsTask(id, state)
	task.UpdatedAt = time.Now().AddDate(0, 0, -dayOffset)
	return task
}

func renderFixtureProject(t *testing.T, id, slug, name string, tasks []*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("NewProject(%q): %v", id, err)
	}
	project.Slug = slug
	project.Tasks = tasks
	project.BuildTaskTree()
	return project
}

func renderFixtureTask(
	t *testing.T,
	id string,
	shortID int,
	title string,
	status models.WorkflowState,
	priority int,
	section models.WorkflowState,
) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.WorkflowState = legacyStateForStatusSection(status, string(section))
	task.Priority = priority
	return task
}

func newTestConfig() *config.RuntimeState {
	return newTestConfigAt("")
}

func newTestConfigAt(dataDir string) *config.RuntimeState {
	return config.NewRuntimeState(dataDir, config.NewDomains())
}

func testConfigWithProjects(projects ...*models.Project) *config.RuntimeState {
	runtime := newTestConfig()
	setTestProjects(runtime, projects)
	return runtime
}

func setTestProjects(runtime *config.RuntimeState, projects []*models.Project) {
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		if err := catalog.UpsertProject(project); err != nil {
			panic(err)
		}
	}
	runtime.SetCatalogInMemory(catalog)
}

func testRuntimeAt(dataDir string, runtime *config.RuntimeState) *config.RuntimeState {
	if runtime == nil {
		return config.EmptyRuntimeState(dataDir)
	}
	domains := &config.Domains{
		Catalog:  runtime.Catalog(),
		Planning: runtime.Planning(),
		Machine:  runtime.Machine(),
	}
	if dataDir != "" {
		// Checked non-catalog commits now require the authoritative catalog
		// identity. Persist that one foundation artifact for real-directory test
		// runtimes instead of letting each test mint an unrelated domain ID.
		manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
		if err := manager.CommitCatalog(domains.Catalog); err != nil {
			panic(fmt.Sprintf("seed test workspace catalog: %v", err))
		}
	}
	return config.NewRuntimeState(dataDir, domains)
}

func newCrossProjectTestEnv(t *testing.T, taskGroups ...[]*models.Task) *crossProjectTestEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	projects := make([]*models.Project, len(taskGroups))
	for i, tasks := range taskGroups {
		slug := fmt.Sprintf("out%d", i)
		project, err := models.NewProject(uuid.New().String(), slug, "")
		if err != nil {
			t.Fatalf("NewProject: %v", err)
		}
		project.Slug = slug
		project.Path = filepath.Join(dataDir, "projects", slug)
		project.Tasks = tasks
		next := 1
		for _, task := range tasks {
			if task.ShortID >= next {
				next = task.ShortID + 1
			}
		}
		project.NextShortID = next
		project.BuildTaskTree()
		projects[i] = project
	}

	repo := newScenarioRepo(commitSvc, projects...)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	coords := coordinator.NewSecondaryCoordinators(svc)
	return &crossProjectTestEnv{
		dataDir: dataDir, ledger: ledger, svc: svc,
		bulk: coords.Bulk, taskItem: coords.TaskItem, projects: projects,
	}
}

// crossProjectTestEnv seeds projects on disk and wires the coordinators used
// by cross-project Outliner mutation tests.

// crossProjectTestEnv seeds projects on disk and wires the coordinators used
// by cross-project Outliner mutation tests.
type crossProjectTestEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	bulk     coordinator.BulkCoordinator
	taskItem coordinator.TaskItemCoordinator
	projects []*models.Project
}

func (e *crossProjectTestEnv) reload(t *testing.T, idx int) *models.Project {
	t.Helper()
	project := e.projects[idx]
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %d: %v", idx, err)
	}
	return fresh
}

func (e *crossProjectTestEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

// runResult executes a coordinator command and asserts it produced a committed
// transaction-result message, returning the application outcome.

// runResult executes a coordinator command and asserts it produced a committed
// transaction-result message, returning the application outcome.
func runResult(t *testing.T, cmd tea.Cmd) app.TaskResult {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case coordinator.TransactionResultMsg:
		return msg.Result
	case coordinator.TransactionErrorMsg:
		t.Fatalf("expected a committed transaction, got error: %v", msg.Err)
	default:
		t.Fatalf("expected coordinator.TransactionResultMsg, got %T", msg)
	}
	return app.TaskResult{}
}

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
// It resets by replacement rather than field by field: the armed markers behind
// the state and completion-age facets are the carrier's own, so a fresh carrier
// is the only spelling of "no facet at all" that cannot leave one of them set.

// clearCommonFilters resets a carrier to its defaults. Production clears an
// axis by removing its clauses from the query, so this lives here: it is how a
// test that authors a carrier puts it back, not a second clearing path the
// Outliner can take.
// It resets by replacement rather than field by field: the armed markers behind
// the state and completion-age facets are the carrier's own, so a fresh carrier
// is the only spelling of "no facet at all" that cannot leave one of them set.
func clearCommonFilters(f *query.CommonFilterState) {
	*f = query.NewCommonFilterState()
}

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.

// passesCommonFiltersAt answers the row facets the way the Outliner does: the
// carrier is lifted into its clauses and models.Query.Admits decides. Tests
// still author carriers, so this is the one place that lift happens for them,
// and it is a test helper precisely because production has no such direction.
//
// The hierarchy fields are dropped for the same reason rowPredicateQuery drops
// them: their census lives in passesHierarchyFilter, not in Admits.
func passesCommonFiltersAt(task *models.Task, filter *query.CommonFilterState, now time.Time) bool {
	predicate := query.WithoutFields(
		models.Query{Clauses: query.CommonFilterClauses(filter)},
		query.FilterQueryHierarchyFields...,
	)
	return predicate.Admits(task, query.EmptyFilterQueryProject, now)
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// legacyStateForStatusSection collapses a legacy (status, section) pair onto a
// single canonical WorkflowState, applying the migration precedence used across
// the refactored tests: canceled > done/Completed > in_progress >
// blocked/Waiting > standby/Standby > maybe/Someday > Inbox > Next > ready.
// Earlier rules win when multiple legacy signals are present.

// legacyStateForStatusSection collapses a legacy (status, section) pair onto a
// single canonical WorkflowState, applying the migration precedence used across
// the refactored tests: canceled > done/Completed > in_progress >
// blocked/Waiting > standby/Standby > maybe/Someday > Inbox > Next > ready.
// Earlier rules win when multiple legacy signals are present.
func legacyStateForStatusSection(status models.WorkflowState, section string) models.WorkflowState {
	switch {
	case status == models.StateCanceled:
		return models.StateCanceled
	case status == models.StateDone || section == string(models.StateDone):
		return models.StateDone
	case status == models.StateDoing:
		return models.StateDoing
	case status == models.StateWaiting || section == string(models.StateWaiting):
		return models.StateWaiting
	case status == models.StatePaused || section == string(models.StatePaused):
		return models.StatePaused
	case status == models.StateSomeday || section == string(models.StateSomeday):
		return models.StateSomeday
	case section == string(models.StateInbox):
		return models.StateInbox
	case section == string(models.StateBacklog):
		return models.StateBacklog
	default:
		return models.StateBacklog
	}
}

// backgroundParams returns the SGR parameters lipgloss emits for a background
// color. A gutter piece merges its own foreground into one sequence, so a test
// asks for the parameters rather than a standalone escape.
func backgroundParams(t *testing.T, c color.Color) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Background(c).Render(" ")
	open, _, ok := strings.Cut(rendered, "m")
	if !ok {
		t.Fatalf("background render carried no SGR sequence: %q", rendered)
	}
	return strings.TrimPrefix(open, "\x1b[")
}

// withoutTheCursorBand strips the band's own arming sequences off a marked row,
// leaving the bytes the row drew. PaintBackground re-arms the band after every
// reset the content carries, so a segment assembled from several styles — a due
// chip is a padded run of them — stops being one contiguous string on the line
// even though each of its pieces still carries its own style whole. Removing the
// arms is how a test asks about the segment instead of about the band.
func withoutTheCursorBand(t *testing.T, line string) string {
	t.Helper()
	for _, bg := range []color.Color{theme.ColorCursorRowBg, theme.ColorFlashDimBg} {
		line = strings.ReplaceAll(line, "\x1b["+backgroundParams(t, bg)+"m", "")
	}
	return line
}

func cursorHighlightOutliner(t *testing.T, tasks ...*models.Task) (*OutlinerModel, *models.Project) {
	t.Helper()
	return cursorHighlightOutlinerTheme(t, "dark", tasks...)
}

func cursorHighlightOutlinerTheme(t *testing.T, themeName string, tasks ...*models.Task) (*OutlinerModel, *models.Project) {
	t.Helper()
	theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	return NewOutlinerModel(project, nil, newTestConfig()), project
}

// visibleTaskIDs returns the task IDs of the outliner's current task rows in
// render order, ignoring headers and separators.

// visibleTaskIDs returns the task IDs of the outliner's current task rows in
// render order, ignoring headers and separators.
func visibleTaskIDs(m *OutlinerModel) []string {
	var ids []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids = append(ids, row.Task.ID)
		}
	}
	return ids
}

// runBatchResult executes a coordinator command and asserts it produced a
// committed batch-transaction message, returning it.

func testWorkflowStateSet(states ...models.WorkflowState) models.WorkflowStateSet {
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		panic(err)
	}
	return set
}

func grammarLiteralsInNode(node ast.Node, triggers, destinations map[string]bool) (map[string]bool, map[string]bool) {
	foundTriggers := make(map[string]bool)
	foundDestinations := make(map[string]bool)
	ast.Inspect(node, func(child ast.Node) bool {
		literal, ok := child.(*ast.BasicLit)
		if !ok || (literal.Kind != token.STRING && literal.Kind != token.CHAR) {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		if triggers[value] {
			foundTriggers[value] = true
		}
		if destinations[value] {
			foundDestinations[value] = true
		}
		return true
	})
	return foundTriggers, foundDestinations
}

func inlineFormTriggerCount(literal string, triggers map[string]bool) int {
	found := make(map[string]bool)
	for _, word := range strings.Fields(literal) {
		normalized := strings.Trim(word, "\t\n\r ,.;:!?()[]{}\"'")
		longest := ""
		for trigger := range triggers {
			if trigger == "%" && isFormattingDirective(normalized) {
				continue
			}
			if len(normalized) > len(trigger) && len(trigger) > len(longest) && strings.HasPrefix(normalized, trigger) {
				longest = trigger
			}
		}
		if longest != "" {
			found[longest] = true
		}
	}
	return len(found)
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}

func newScenarioRepo(svc *storage.CommitService, projects ...*models.Project) *scenarioRepo {
	return &scenarioRepo{svc: svc, projects: projects, revisions: make(map[string]string)}
}

func isFormattingDirective(value string) bool {
	if len(value) != 2 || value[0] != '%' {
		return false
	}
	return strings.ContainsRune("vTtbcdoOqxXUeEfFgGsp", rune(value[1]))
}

// scenarioRepo is a storage-backed app.Repository that drives coordinator
// mutations through the transactional commit service with the workspace undo
// ledger attached, mirroring the CLI/TUI production repository. It tracks each
// project's revision from the commit results so optimistic concurrency holds.
type scenarioRepo struct {
	svc       *storage.CommitService
	projects  []*models.Project
	revisions map[string]string
}

func (r *scenarioRepo) seed(t *testing.T) {
	t.Helper()
	changesets := make([]storage.ChangeSet, len(r.projects))
	for i, p := range r.projects {
		changesets[i] = storage.ChangeSet{Project: p, UndoAction: "seed"}
	}
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	for i, p := range r.projects {
		r.revisions[p.ID] = results[i].Revision
	}
}

// Snapshot serves the in-memory project pointers with each project's revision
// read from DISK, exactly like the production cli/tui repositories. Reusing the
// cached commit-result revision is not safe across chained mutations: the
// result revision is computed from the in-memory model, whose canonical form
// can differ from a reparse of its own written file (a task edited within the
// minute of its creation serializes `updated:` from the in-memory model but
// parses back as updated==created, which the canonical renderer omits).

// Snapshot serves the in-memory project pointers with each project's revision
// read from DISK, exactly like the production cli/tui repositories. Reusing the
// cached commit-result revision is not safe across chained mutations: the
// result revision is computed from the in-memory model, whose canonical form
// can differ from a reparse of its own written file (a task edited within the
// minute of its creation serializes `updated:` from the in-memory model but
// parses back as updated==created, which the canonical renderer omits).
func (r *scenarioRepo) Snapshot() (app.Snapshot, error) {
	revs := make(map[string]string, len(r.projects))
	docs := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, p := range r.projects {
		doc, err := storage.LoadProjectDocument(p)
		if err != nil {
			return nil, err
		}
		revs[p.ID] = doc.Revision
		docs[p.ID] = doc
	}
	return app.NewSnapshot(r.projects, revs, docs), nil
}

func (r *scenarioRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		return nil, err
	}
	for i := range changesets {
		r.revisions[changesets[i].Project.ID] = results[i].Revision
	}
	return results, nil
}

// CommitArtifactTransaction makes the scenario repository exercise the same
// note-only commit and ledger boundary as the production TUI repository.

// CommitArtifactTransaction makes the scenario repository exercise the same
// note-only commit and ledger boundary as the production TUI repository.
func (r *scenarioRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	return r.svc.CommitArtifactTransaction(request)
}

// testPeekWindowRows is the window a document-level test measures against, and
// testPeekWindow the geometry carrying it. The window is the frame's answer
// rather than the document's, so a test that builds a document without a frame
// has to say which window it wants; this is that declaration, and it is one
// declaration so every one of those tests reads the same height.
const testPeekWindowRows = 5

func testPeekWindow() peekDockGeometry {
	return peekDockGeometry{bodyRows: testPeekWindowRows}
}
