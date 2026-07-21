package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestOutlinerCtrlTOpensGlobalTagManagerInCrossProjectMode(t *testing.T) {
	m := &OutlinerModel{}

	_, cmd, handled := m.handleUtilityKey("ctrl+t")
	if !handled {
		t.Fatal("ctrl+t should be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+t to emit a command")
	}
	msg := cmd()
	if _, ok := msg.(ShowGlobalTagManagerMsg); !ok {
		t.Fatalf("ctrl+t emitted %T, want ShowGlobalTagManagerMsg", msg)
	}
}

func TestGlobalTagManagerIncludesCatalogOnlyTags(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"catalog-only", "shared"}
	task := newGlobalTagTestTask(t, "task-1", "Tagged task")
	task.Tags = []string{"shared", "task-only"}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()
	saveGlobalTagTestProject(t, project)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})

	manager := NewGlobalTagManagerModel([]*models.Project{project}, testRuntimeAt(cfg.DataDir(), cfg), TagCoordinator{})

	catalogOnly := findGlobalTagInfo(t, manager.tags, "catalog-only")
	if catalogOnly.TaskCount != 0 {
		t.Fatalf("catalog-only TaskCount = %d, want 0", catalogOnly.TaskCount)
	}
	if len(catalogOnly.Projects) != 1 || catalogOnly.Projects[0].Project != project || catalogOnly.Projects[0].TaskCount != 0 {
		t.Fatalf("catalog-only Projects = %#v, want owning project with zero task count", catalogOnly.Projects)
	}

	shared := findGlobalTagInfo(t, manager.tags, "shared")
	if shared.TaskCount != 1 {
		t.Fatalf("shared TaskCount = %d, want 1", shared.TaskCount)
	}

	taskOnly := findGlobalTagInfo(t, manager.tags, "task-only")
	if taskOnly.TaskCount != 1 {
		t.Fatalf("task-only TaskCount = %d, want 1", taskOnly.TaskCount)
	}
}

func TestGlobalTagManagerBuildListMarksPinnedTags(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"backend", "frontend"}
	saveGlobalTagTestProject(t, project)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})
	cfg.Settings().GlobalPinnedTags = []string{"frontend"}

	manager := NewGlobalTagManagerModel([]*models.Project{project}, testRuntimeAt(cfg.DataDir(), cfg), TagCoordinator{})

	backend := findGlobalTagInfo(t, manager.tags, "backend")
	if backend.Pinned {
		t.Fatalf("backend Pinned = true, want false")
	}
	frontend := findGlobalTagInfo(t, manager.tags, "frontend")
	if !frontend.Pinned {
		t.Fatalf("frontend Pinned = false, want true")
	}
}

func TestGlobalTagManagerViewShowsPinnedMarker(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"backend"}
	saveGlobalTagTestProject(t, project)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})
	cfg.Settings().GlobalPinnedTags = []string{"backend"}
	manager := NewGlobalTagManagerModel([]*models.Project{project}, testRuntimeAt(cfg.DataDir(), cfg), TagCoordinator{})

	plain := ansi.Strip(manager.View().Content)
	if !strings.Contains(plain, "* backend") {
		t.Fatalf("global tag manager view missing pinned marker:\n%s", plain)
	}
	if !strings.Contains(plain, "p pin") {
		t.Fatalf("global tag manager footer missing pin key:\n%s", plain)
	}
}

func TestGlobalTagManagerMinimumLayoutScrollsTagsNoOverflow(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	for i := range 24 {
		project.Tags = append(project.Tags, fmt.Sprintf("tag-%02d", i))
	}
	saveGlobalTagTestProject(t, project)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})
	manager := NewGlobalTagManagerModel([]*models.Project{project}, testRuntimeAt(cfg.DataDir(), cfg), TagCoordinator{})
	manager.SetSize(72, 18)
	manager.cursor = globalTagIndex(t, manager.tags, "tag-23")

	plain, issues := normalizeScreen(manager.View().Content, 72, 18)
	if len(issues) > 0 {
		t.Fatalf("global tag manager should fit 72x18 view budget, got issues %+v:\n%s", issues, plain)
	}
	if !strings.Contains(plain, "tag-23") {
		t.Fatalf("selected tag should remain visible after scroll:\n%s", plain)
	}
	if !strings.Contains(plain, "█") && !strings.Contains(plain, "░") {
		t.Fatalf("overflowing tag list should render a scrollbar:\n%s", plain)
	}
}

func TestGlobalTagManagerProjectPickerMinimumLayoutNoOverflow(t *testing.T) {
	cfg := newTestConfig()
	projects := make([]*models.Project, 0, 20)
	for i := range 20 {
		project := newGlobalTagTestProject(t, fmt.Sprintf("project-%02d", i), fmt.Sprintf("Project %02d", i))
		project.Tags = []string{"shared"}
		saveGlobalTagTestProject(t, project)
		addTestProject(cfg, project)
		projects = append(projects, project)
	}

	manager := NewGlobalTagManagerModel(projects, testRuntimeAt(cfg.DataDir(), cfg), TagCoordinator{})
	manager.SetSize(72, 18)
	manager.showProjectPicker = true
	manager.selectedTag = "shared"
	manager.projectCursor = 19

	plain, issues := normalizeScreen(manager.View().Content, 72, 18)
	if len(issues) > 0 {
		t.Fatalf("global tag project picker should fit 72x18 view budget, got issues %+v:\n%s", issues, plain)
	}
	if !strings.Contains(plain, "Project 19") {
		t.Fatalf("selected project should remain visible after scroll:\n%s", plain)
	}
	if !strings.Contains(plain, "█") && !strings.Contains(plain, "░") {
		t.Fatalf("overflowing project list should render a scrollbar:\n%s", plain)
	}
}

func TestGlobalTagManagerTogglePinnedTagPersists(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"backend"}
	saveGlobalTagTestProject(t, project)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})
	rs := testRuntimeAt(t.TempDir(), cfg)
	model := NewGlobalTagManagerModel([]*models.Project{project}, rs, TagCoordinator{})
	model.cursor = globalTagIndex(t, model.tags, "backend")

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	model = updated.(*GlobalTagManagerModel)
	if cmd == nil {
		t.Fatal("p key returned nil command, want pin toggle")
	}
	msg := cmd()
	pinned, ok := msg.(GlobalTagPinnedMsg)
	if !ok {
		t.Fatalf("p key message = %T, want GlobalTagPinnedMsg (%v)", msg, msg)
	}
	if pinned.Name != "backend" || !pinned.Pinned {
		t.Fatalf("GlobalTagPinnedMsg = %#v, want backend pinned", pinned)
	}
	updated, _ = model.Update(msg)
	model = updated.(*GlobalTagManagerModel)

	if !rs.Settings().IsGlobalTagPinned("backend") {
		t.Fatal("backend should be globally pinned in the settings domain")
	}
	if got := findGlobalTagInfo(t, model.tags, "backend"); !got.Pinned {
		t.Fatalf("manager tag = %#v, want pinned backend", got)
	}
	reloaded, err := config.LoadRuntimeState(rs.DataDir())
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	if got, want := reloaded.Settings().GlobalPinnedTagList(), []string{"backend"}; !slices.Equal(got, want) {
		t.Fatalf("persisted global pinned tags = %v, want %v", got, want)
	}
}

func TestGlobalTagManagerNavigatesCatalogOnlyTag(t *testing.T) {
	project := newGlobalTagTestProject(t, "alpha", "Alpha")
	project.Tags = []string{"catalog-only"}
	saveGlobalTagTestProject(t, project)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})

	manager := NewGlobalTagManagerModel([]*models.Project{project}, testRuntimeAt(cfg.DataDir(), cfg), TagCoordinator{})
	manager.cursor = globalTagIndex(t, manager.tags, "catalog-only")

	_, cmd := manager.handleNormalMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on catalog-only tag to emit navigation command")
	}
	msg := cmd()
	nav, ok := msg.(GlobalTagNavigateMsg)
	if !ok {
		t.Fatalf("enter emitted %T, want GlobalTagNavigateMsg", msg)
	}
	if nav.Project != project || nav.Tag != "catalog-only" {
		t.Fatalf("navigation = (%v, %q), want (%v, catalog-only)", nav.Project, nav.Tag, project)
	}
}

func TestGlobalTagNavigateRespectsDefaultOutlinerView(t *testing.T) {
	app, project := newTestAppModelWithProject()
	syncTestRuntime(app)
	project.Tags = []string{"catalog-only"}
	mockPersister := NewMockPersister()
	mockPersister.Projects[project.ID] = project
	app.persister = mockPersister
	app.pushView(ViewGlobalTagManager)

	handled, _ := app.handleTagMessages(GlobalTagNavigateMsg{Project: project, Tag: "catalog-only"})

	if !handled {
		t.Fatal("GlobalTagNavigateMsg should be handled")
	}
	if app.currentView() != ViewOutliner {
		t.Fatalf("current view = %s, want %s", app.currentView(), ViewOutliner)
	}
	if app.outliner == nil {
		t.Fatal("outliner should be initialized")
	}
	if !app.outliner.filter.Tags["catalog-only"] {
		t.Fatalf("outliner tag filter = %v, want catalog-only", app.outliner.filter.Tags)
	}
}

// TestGlobalTagRenameCommitsWorkspaceWideAndUndoes drives the migrated global
// tag rename through the coordinator/app.Service seam over the real
// transactional storage stack with a workspace undo ledger. It proves the rename
// commits every affected project (catalog-only usage in one, task usage in
// another) as ONE grouped transaction and that a single ledger undo/redo reverses
// and re-applies the whole workspace-wide rename.
func TestGlobalTagRenameCommitsWorkspaceWideAndUndoes(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	catalog := newLedgerTagProject(t, dataDir, "catalog")
	catalog.Tags = []string{"legacy"}

	task := newGlobalTagTestTask(t, "task-1", "Uses legacy")
	task.Tags = []string{"legacy"}
	tasksProject := newLedgerTagProject(t, dataDir, "tasks", task)

	repo := newScenarioRepo(commitSvc, catalog, tasksProject)
	repo.seed(t)
	svc := app.NewService(repo, nil, nil)
	coords := newSecondaryCoordinators(svc, nil)

	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{catalog, tasksProject})
	manager := NewGlobalTagManagerModel([]*models.Project{catalog, tasksProject}, testRuntimeAt(cfg.DataDir(), cfg), coords.Tag)

	cmd := manager.renameGlobalTag("legacy", "modern")
	if cmd == nil {
		t.Fatal("expected rename command")
	}
	msg := cmd()
	renamed, ok := msg.(GlobalTagRenamedMsg)
	if !ok {
		t.Fatalf("rename emitted %T, want GlobalTagRenamedMsg", msg)
	}
	if renamed.OldName != "legacy" || renamed.NewName != "modern" {
		t.Fatalf("rename names = (%q -> %q), want legacy -> modern", renamed.OldName, renamed.NewName)
	}
	if renamed.ProjectCount != 2 || renamed.TaskCount != 1 {
		t.Fatalf("rename counts = (%d projects, %d tasks), want (2 projects, 1 task)", renamed.ProjectCount, renamed.TaskCount)
	}

	catReload := reloadLedgerProject(t, catalog)
	if !catReload.HasCatalogTag("modern") || catReload.HasCatalogTag("legacy") {
		t.Fatalf("catalog tags after rename = %v, want modern only", catReload.Tags)
	}
	taskReload := reloadLedgerProject(t, tasksProject)
	if len(taskReload.Tasks) != 1 || !taskReload.Tasks[0].HasTag("modern") || taskReload.Tasks[0].HasTag("legacy") {
		t.Fatalf("task tags after rename = %v, want modern only", taskReload.Tasks)
	}

	// A single grouped ledger undo reverses the whole workspace-wide rename.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	catReload = reloadLedgerProject(t, catalog)
	if !catReload.HasCatalogTag("legacy") || catReload.HasCatalogTag("modern") {
		t.Fatalf("catalog tags after undo = %v, want legacy only", catReload.Tags)
	}
	taskReload = reloadLedgerProject(t, tasksProject)
	if !taskReload.Tasks[0].HasTag("legacy") || taskReload.Tasks[0].HasTag("modern") {
		t.Fatalf("task tags after undo = %v, want legacy only", taskReload.Tasks[0].Tags)
	}

	// Redo re-applies the whole rename.
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger redo: %v", err)
	}
	catReload = reloadLedgerProject(t, catalog)
	if !catReload.HasCatalogTag("modern") || catReload.HasCatalogTag("legacy") {
		t.Fatalf("catalog tags after redo = %v, want modern only", catReload.Tags)
	}
	taskReload = reloadLedgerProject(t, tasksProject)
	if !taskReload.Tasks[0].HasTag("modern") || taskReload.Tasks[0].HasTag("legacy") {
		t.Fatalf("task tags after redo = %v, want modern only", taskReload.Tasks[0].Tags)
	}
}

// newLedgerTagProject builds a project rooted under dataDir/projects/slug so the
// transactional commit service accepts it as an approved-root target.
func newLedgerTagProject(t *testing.T, dataDir, slug string, tasks ...*models.Task) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Path = filepath.Join(dataDir, "projects", slug)
	p.Tasks = tasks
	next := 1
	for _, task := range tasks {
		if task.ShortID >= next {
			next = task.ShortID + 1
		}
	}
	p.NextShortID = next
	p.EnsureDefaultSections()
	p.BuildTaskTree()
	return p
}

// reloadLedgerProject reads a project fresh from disk so an assertion sees the
// committed bytes rather than the in-memory pointer the coordinator mutated.
func reloadLedgerProject(t *testing.T, p *models.Project) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Path: p.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %s: %v", p.Slug, err)
	}
	return fresh
}

func newGlobalTagTestProject(t *testing.T, id, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	return project
}

func newGlobalTagTestTask(t *testing.T, id, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	return task
}

func saveGlobalTagTestProject(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%s): %v", project.Name, err)
	}
}

func findGlobalTagInfo(t *testing.T, tags []GlobalTagInfo, name string) GlobalTagInfo {
	t.Helper()
	for _, tag := range tags {
		if tag.Name == name {
			return tag
		}
	}
	t.Fatalf("tag %q not found in %#v", name, tags)
	return GlobalTagInfo{}
}

func globalTagIndex(t *testing.T, tags []GlobalTagInfo, name string) int {
	t.Helper()
	for i, tag := range tags {
		if tag.Name == name {
			return i
		}
	}
	t.Fatalf("tag %q not found in %#v", name, tags)
	return 0
}
