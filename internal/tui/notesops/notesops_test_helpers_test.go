package notesops

import (
	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/projecthub"
)

// testShell is the narrowest thing that satisfies Shell: an Outliner and a
// runtime, which is all the index tests read. Every other port answers nil,
// because a note read that reached one would be exercising the shell rather
// than the index.
type testShell struct {
	outlinerView *outliner.OutlinerModel
	runtime      *config.RuntimeState
}

func (s *testShell) Outliner() *outliner.OutlinerModel    { return s.outlinerView }
func (s *testShell) DomainRuntime() *config.RuntimeState  { return s.runtime }
func (s *testShell) DomainService() *app.Service          { return nil }
func (s *testShell) AvailableProjects() []*models.Project { return nil }
func (s *testShell) ProjectByID(string) *models.Project   { return nil }

func (s *testShell) Registry() *operation.OperationRegistry { return nil }
func (s *testShell) Executor() *operation.OperationExecutor { return nil }
func (s *testShell) ManageHub() *projecthub.ManageHubModel  { return nil }

func (s *testShell) RegisterOperation(operation.OperationDescriptor, operation.OperationBinding) error {
	return nil
}

func (s *testShell) MountActionSheet(*listsurface.ContextActionSheetModel) {}

func testOutliner(projects ...*models.Project) *outliner.OutlinerModel {
	runtime := testRuntimeWithProjects(projects...)
	outlinerView := outliner.NewOutlinerModel(nil, nil, runtime)
	outlinerView.Population.Projects = projects
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	return outlinerView
}

func testRuntimeWithProjects(projects ...*models.Project) *config.RuntimeState {
	runtime := config.NewRuntimeState("", config.NewDomains())
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		if err := catalog.UpsertProject(project); err != nil {
			panic(err)
		}
	}
	runtime.SetCatalogInMemory(catalog)
	return runtime
}

func testTask(id string, state models.WorkflowState) *models.Task {
	return &models.Task{ID: id, Title: id, WorkflowState: state}
}

func testProject(id string, tasks ...*models.Task) *models.Project {
	project := &models.Project{ID: id, Name: id, Slug: id, Tasks: tasks}
	project.BuildTaskTree()
	return project
}

// searchRankedList holds a query and shows it as the flat ranked list, which is
// the presentation the notes tier appears in.
func searchRankedList(outlinerView *outliner.OutlinerModel, query string) {
	outlinerView.SetSearchQuery(query)
	if !outlinerView.ShowsRankedList() {
		outlinerView.ToggleRankedList()
	}
}

// indexNotesService is the in-memory note collection the index tests read from.
// It is a local copy rather than an import: a test fixture exported to cross a
// package boundary is production-unused API, which scripts/dead_code.sh
// rejects.
type indexNotesService struct {
	byLocation  map[app.NoteLocation][]*models.Note
	errors      map[app.NoteLocation]error
	snapshotErr error
}

func (s *indexNotesService) NotesAtLocations(locations []app.NoteLocation) ([]app.NoteCollectionResult, error) {
	if s.snapshotErr != nil {
		return nil, s.snapshotErr
	}
	results := make([]app.NoteCollectionResult, len(locations))
	for i, location := range locations {
		results[i] = app.NoteCollectionResult{
			Location: location,
			Notes:    append([]*models.Note(nil), s.byLocation[location]...),
			Err:      s.errors[location],
		}
	}
	return results, nil
}

func (s *indexNotesService) ApplyNoteBatch(app.NoteBatchRequest) (app.NoteBatchResult, error) {
	return app.NoteBatchResult{}, nil
}
