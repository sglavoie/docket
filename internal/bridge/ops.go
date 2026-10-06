package bridge

import (
	"path/filepath"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type openPayload struct {
	DataDir         string `json:"data_dir"`
	RuntimeDir      string `json:"runtime_dir"`
	DeviceID        string `json:"device_id"`
	CreateIfMissing bool   `json:"create_if_missing,omitempty"`
}

type schemas struct {
	Bridge         int `json:"bridge"`
	Catalog        int `json:"catalog"`
	TaskMarkdown   int `json:"task_markdown"`
	DomainEnvelope int `json:"domain_envelope"`
}

type statusResult struct {
	Workspace string    `json:"workspace"`
	DataDir   string    `json:"data_dir"`
	DeviceID  string    `json:"device_id"`
	Schemas   schemas   `json:"schemas"`
	Revisions revisions `json:"revisions"`
}

// workspaceOpen opens the workspace the payload names under the app's own
// machine identity. It recovers interrupted journals, which the engine always
// does first, but refuses a workspace it would have to migrate. Opening while
// another workspace is open replaces it.
func (b *Bridge) workspaceOpen(request Request) (any, error) {
	var payload openPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	switch {
	case payload.DataDir == "" || payload.RuntimeDir == "" || payload.DeviceID == "":
		return nil, newError(CodeInvalidRequest, "data_dir, runtime_dir, and device_id are required")
	case !filepath.IsAbs(payload.DataDir):
		return nil, newError(CodeInvalidRequest, "data_dir %q is not absolute", payload.DataDir)
	}
	b.workspace = nil
	runtime, err := engine.Open(engine.Config{
		DataDir:         payload.DataDir,
		Machine:         engine.ExplicitMachine(payload.RuntimeDir, payload.DeviceID),
		Clock:           b.options.Clock,
		IDs:             b.options.IDs,
		CreateIfMissing: payload.CreateIfMissing,
		RefuseMigration: true,
	})
	if err != nil {
		return nil, err
	}
	if _, err = runtime.ReconcilePlanning(); err != nil {
		return nil, err
	}
	catalog := runtime.State().Catalog()
	if catalog == nil || catalog.WorkspaceID == "" {
		return nil, newError(CodeInternal, "workspace at %s carries no identity", payload.DataDir)
	}
	b.workspace = &openWorkspace{
		runtime:  runtime,
		id:       catalog.WorkspaceID,
		deviceID: payload.DeviceID,
		replay:   newReplayCache(),
	}
	return b.status()
}

func (b *Bridge) workspaceStatus(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	return b.status()
}

func (b *Bridge) status() (statusResult, error) {
	workspace := b.workspace
	if err := workspace.runtime.Refresh(); err != nil {
		return statusResult{}, err
	}
	current, err := currentRevisions(workspace.runtime)
	if err != nil {
		return statusResult{}, err
	}
	return statusResult{
		Workspace: workspace.id,
		DataDir:   workspace.runtime.DataDir(),
		DeviceID:  workspace.deviceID,
		Schemas: schemas{
			Bridge:         Version,
			Catalog:        workspace.runtime.State().Catalog().SchemaVersion,
			TaskMarkdown:   storage.CurrentTaskSchemaVersion,
			DomainEnvelope: config.CurrentDomainSchemaVersion,
		},
		Revisions: current,
	}, nil
}

// workspaceClose forgets the open workspace. The engine holds no lock or
// handle between requests, so there is nothing else to release.
func (b *Bridge) workspaceClose(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	b.workspace = nil
	return struct {
		Closed bool `json:"closed"`
	}{true}, nil
}

type projectsResult struct {
	Active   []projectView `json:"active"`
	Parked   []projectView `json:"parked"`
	Archived []projectView `json:"archived"`
}

// queryProjects reports the three populations apart: the task workspace, the
// parked projects AvailableProjects adds to it, and the archived ones.
func (b *Bridge) queryProjects(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	state := runtime.State()
	result := projectsResult{Active: []projectView{}, Parked: []projectView{}, Archived: []projectView{}}
	active := make(map[string]bool)
	for _, project := range state.TaskWorkspaceProjects() {
		active[project.ID] = true
		result.Active = append(result.Active, viewProject(state, project))
	}
	for _, project := range state.AvailableProjects() {
		if !active[project.ID] {
			result.Parked = append(result.Parked, viewProject(state, project))
		}
	}
	for _, project := range state.ArchivedProjects() {
		result.Archived = append(result.Archived, viewProject(state, project))
	}
	return result, nil
}

type sortPayload struct {
	Field     string `json:"field"`
	Ascending bool   `json:"ascending,omitempty"`
}

type tasksPayload struct {
	Mode  string       `json:"mode,omitempty"`
	Scope string       `json:"scope,omitempty"`
	Query string       `json:"query,omitempty"`
	Sort  *sortPayload `json:"sort,omitempty"`
}

type queryItemView struct {
	Task  taskView `json:"task"`
	Today bool     `json:"today"`
	Next  bool     `json:"next"`
}

type tasksResult struct {
	Query       string                   `json:"query"`
	Items       []queryItemView          `json:"items"`
	Diagnostics []engine.QueryDiagnostic `json:"diagnostics"`
}

// queryTasks evaluates one canonical query: its clause string in the strict
// canonical grammar, with mode, scope, and sort beside it.
func (b *Bridge) queryTasks(request Request) (any, error) {
	var payload tasksPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	parsed, err := models.ParseQuery(payload.Query)
	if err != nil {
		return nil, newError(CodeValidation, "query: %v", err)
	}
	var sort models.QuerySort
	if payload.Sort != nil {
		sort = models.QuerySort{Field: models.QuerySortField(payload.Sort.Field), Ascending: payload.Sort.Ascending}
	}
	query, err := models.NewQuery(models.QueryMode(payload.Mode), payload.Scope, sort, parsed.Clauses...)
	if err != nil {
		return nil, newError(CodeValidation, "query: %v", err)
	}
	answer, err := b.workspace.runtime.Query(query)
	if err != nil {
		return nil, err
	}
	result := tasksResult{
		Query:       answer.Query.String(),
		Items:       make([]queryItemView, 0, len(answer.Items)),
		Diagnostics: append([]engine.QueryDiagnostic{}, answer.Diagnostics...),
	}
	for _, item := range answer.Items {
		result.Items = append(result.Items, queryItemView{
			Task:  viewTask(item.Locator().ProjectID, item.Task),
			Today: item.Today,
			Next:  item.Next,
		})
	}
	return result, nil
}

type taskPayload struct {
	Task locator `json:"task"`
}

type taskResult struct {
	Task     taskView    `json:"task"`
	Project  projectView `json:"project"`
	Revision string      `json:"revision"`
}

// queryTask reads one task by its qualified identity, with the canonical
// revision of the project that owns it.
func (b *Bridge) queryTask(request Request) (any, error) {
	var payload taskPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	return b.readTask(payload.Task.app())
}

func (b *Bridge) readTask(target app.TaskLocator) (taskResult, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return taskResult{}, err
	}
	snapshot, err := runtime.Snapshot()
	if err != nil {
		return taskResult{}, err
	}
	ref, ok := snapshot.LookupLocator(target)
	if !ok {
		return taskResult{}, newError(CodeNotFound, "task %s:%s does not resolve", target.ProjectID, target.TaskID).
			with("project_id", target.ProjectID).with("task_id", target.TaskID)
	}
	return taskResult{
		Task:     viewTask(ref.Project.ID, ref.Task),
		Project:  viewProject(runtime.State(), ref.Project),
		Revision: snapshot.Revision(ref.Project.ID),
	}, nil
}

type capturePayload struct {
	ProjectID   string   `json:"project_id,omitempty"`
	Title       string   `json:"title"`
	Destination string   `json:"destination,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type mutationResult struct {
	Task      taskView  `json:"task"`
	Committed committed `json:"committed"`
}

// commandCapture creates one task. The title is literal: the capture grammar
// is not parsed here yet, so a sigil in it stays text. An absent project is the
// protected Unfiled project, and an absent destination the capture default.
func (b *Bridge) commandCapture(request Request) (any, error) {
	var payload capturePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	projectID := payload.ProjectID
	if projectID == "" {
		unfiled := runtime.State().SystemUnfiledProject()
		if unfiled == nil {
			return nil, newError(CodeNotFound, "the workspace has no Unfiled project")
		}
		projectID = unfiled.ID
	}
	var destination app.CaptureDestination
	if payload.Destination != "" {
		parsed, ok := app.ParseCaptureDestination(payload.Destination)
		if !ok {
			return nil, newError(CodeValidation, "%v", app.InvalidCaptureDestinationError(payload.Destination))
		}
		destination = parsed
	}
	return b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		return service.Capture(app.CaptureRequest{
			ProjectID:   projectID,
			Title:       payload.Title,
			Priority:    payload.Priority,
			Tags:        payload.Tags,
			Destination: destination,
		})
	})
}

type transitionPayload struct {
	Task  locator `json:"task"`
	State string  `json:"state"`
}

// commandTransition moves one task through the single lifecycle primitive.
func (b *Bridge) commandTransition(request Request) (any, error) {
	var payload transitionPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	return b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		return service.Transition(app.StateRequest{Locator: payload.Task.app(), Target: models.WorkflowState(payload.State)})
	})
}

// commitTask runs one task mutation and reports the committed task with the
// revisions the commit produced.
func (b *Bridge) commitTask(mutate func(*app.Service) (app.TaskResult, error)) (mutationResult, error) {
	runtime := b.workspace.runtime
	before, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return mutationResult{}, err
	}
	result, err := mutate(runtime.Service())
	if err != nil {
		return mutationResult{}, err
	}
	after, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return mutationResult{}, err
	}
	read, err := b.readTask(app.TaskLocator{ProjectID: result.ProjectID, TaskID: result.TaskID})
	if err != nil {
		return mutationResult{}, err
	}
	return mutationResult{Task: read.Task, Committed: committedEffect(result, before, after)}, nil
}
