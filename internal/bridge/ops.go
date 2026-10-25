package bridge

import (
	"path/filepath"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/syncexchange"
)

type openPayload struct {
	DataDir         string `json:"data_dir"`
	RuntimeDir      string `json:"runtime_dir"`
	DeviceID        string `json:"device_id"`
	CreateIfMissing bool   `json:"create_if_missing,omitempty"`
	// TimeZone is the device's IANA zone. A due date is a local calendar day
	// and Today is a local day, so the engine must agree with the phone about
	// which day it is; a process that inherits no zone runs in UTC otherwise.
	TimeZone string `json:"time_zone,omitempty"`
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
	if err := adoptTimeZone(payload.TimeZone); err != nil {
		return nil, err
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
	if err := b.adopt(runtime, payload.DeviceID, payload.RuntimeDir); err != nil {
		return nil, err
	}
	return b.status()
}

// adopt makes an opened runtime the bridge's one workspace.
func (b *Bridge) adopt(runtime *engine.Runtime, deviceID, runtimeDir string) error {
	if _, err := runtime.ReconcilePlanning(); err != nil {
		return err
	}
	catalog := runtime.State().Catalog()
	if catalog == nil || catalog.WorkspaceID == "" {
		return newError(CodeInternal, "workspace at %s carries no identity", runtime.DataDir())
	}
	b.workspace = &openWorkspace{
		runtime:    runtime,
		id:         catalog.WorkspaceID,
		deviceID:   deviceID,
		runtimeDir: runtimeDir,
		replay:     newReplayCache(),
	}
	return nil
}

type joinPayload struct {
	DataDir     string `json:"data_dir"`
	RuntimeDir  string `json:"runtime_dir"`
	DeviceID    string `json:"device_id"`
	WorkspaceID string `json:"workspace_id"`
	Exchange    string `json:"exchange"`
	InboxDir    string `json:"inbox_dir"`
	TimeZone    string `json:"time_zone,omitempty"`
}

type joinResult struct {
	statusResult
	Import *syncexchange.Report `json:"import"`
}

// workspaceJoin builds this device's working copy of an enrolled workspace
// from the history the transport staged in inbox_dir, then opens it. The
// working copy must not exist yet or be empty; a join that delivered no
// workspace is refused and opens nothing.
func (b *Bridge) workspaceJoin(request Request) (any, error) {
	var payload joinPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	switch {
	case payload.DataDir == "" || payload.RuntimeDir == "" || payload.DeviceID == "" ||
		payload.WorkspaceID == "" || payload.Exchange == "" || payload.InboxDir == "":
		return nil, newError(CodeInvalidRequest, "data_dir, runtime_dir, device_id, workspace_id, exchange, and inbox_dir are required")
	case !filepath.IsAbs(payload.DataDir) || !filepath.IsAbs(payload.InboxDir):
		return nil, newError(CodeInvalidRequest, "data_dir and inbox_dir must be absolute")
	}
	if err := adoptTimeZone(payload.TimeZone); err != nil {
		return nil, err
	}
	b.workspace = nil
	runtime, report, err := engine.JoinInbox(engine.JoinInboxConfig{
		DataDir:     payload.DataDir,
		Exchange:    payload.Exchange,
		WorkspaceID: payload.WorkspaceID,
		InboxDir:    payload.InboxDir,
		Machine:     engine.ExplicitMachine(payload.RuntimeDir, payload.DeviceID),
		Clock:       b.options.Clock,
		IDs:         b.options.IDs,
	})
	if err != nil {
		refusal := newError(CodeRefused, "%v", err)
		if report != nil {
			refusal = refusal.with("import", report)
		}
		return nil, refusal
	}
	if adoptErr := b.adopt(runtime, payload.DeviceID, payload.RuntimeDir); adoptErr != nil {
		return nil, adoptErr
	}
	status, err := b.status()
	if err != nil {
		return nil, err
	}
	return joinResult{statusResult: status, Import: report}, nil
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
	// Lens reopens a saved lens: its mode, scope, query, and sort, which is
	// why it stands alone among those four.
	Lens string `json:"lens,omitempty"`
	// Search narrows whatever the rest selects, read as the desktop's search
	// box reads it.
	Search string `json:"search,omitempty"`
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
	query, err := b.tasksQuery(payload)
	if err != nil {
		return nil, err
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

// tasksQuery builds the one query a tasks request names: a lens or the
// explicit axes, narrowed by the search box.
func (b *Bridge) tasksQuery(payload tasksPayload) (models.Query, error) {
	search, refusal := compileSearch(payload.Search)
	if refusal != nil {
		return models.Query{}, refusal
	}
	if payload.Lens != "" {
		if payload.Mode != "" || payload.Scope != "" || payload.Query != "" || payload.Sort != nil {
			return models.Query{}, newError(CodeInvalidRequest, "lens names its own mode, scope, query, and sort")
		}
		lens, err := b.findLens(payload.Lens)
		if err != nil {
			return models.Query{}, err
		}
		query, err := lensQuery(lens)
		if err != nil {
			return models.Query{}, err
		}
		query.Clauses = append(query.Clauses, search...)
		return query.Canonical(), nil
	}
	parsed, err := models.ParseQuery(payload.Query)
	if err != nil {
		return models.Query{}, newError(CodeValidation, "query: %v", err)
	}
	var sort models.QuerySort
	if payload.Sort != nil {
		sort = models.QuerySort{Field: models.QuerySortField(payload.Sort.Field), Ascending: payload.Sort.Ascending}
	}
	query, err := models.NewQuery(models.QueryMode(payload.Mode), payload.Scope, sort, append(parsed.Clauses, search...)...)
	if err != nil {
		return models.Query{}, newError(CodeValidation, "query: %v", err)
	}
	return query, nil
}

type taskPayload struct {
	Task locator `json:"task"`
}

type taskResult struct {
	Task      taskView    `json:"task"`
	Project   projectView `json:"project"`
	Revision  string      `json:"revision"`
	Relations relations   `json:"relations"`
	// Today and Next are the task's planning memberships, read from the same
	// planning state the Today and Next modes list.
	Today bool `json:"today"`
	Next  bool `json:"next"`
	// Comments and Attachments are the task's rich records whole, and
	// AttachmentDir the workspace path its attachment bytes live under.
	Comments      []commentView    `json:"comments"`
	Attachments   []attachmentView `json:"attachments"`
	AttachmentDir string           `json:"attachment_dir"`
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
	state := runtime.State()
	return taskResult{
		Task:      viewTask(ref.Project.ID, ref.Task),
		Project:   viewProject(state, ref.Project),
		Revision:  snapshot.Revision(ref.Project.ID),
		Relations: taskRelations(snapshot, ref.Project, ref.Task),
		Today:     plannedIn(state.Planning().GetToday(), target),
		Next:      plannedIn(state.NextOrder(), target),

		Comments:      viewTaskComments(ref.Task),
		Attachments:   viewAttachments(runtime.DataDir(), ref.Project, ref.Task),
		AttachmentDir: attachmentDir(runtime.DataDir(), ref.Project, ref.Task),
	}, nil
}

func plannedIn(members []config.ProjectTaskLocator, target app.TaskLocator) bool {
	for _, member := range members {
		if member.ProjectID == target.ProjectID && member.TaskID == target.TaskID {
			return true
		}
	}
	return false
}

type mutationResult struct {
	Task      taskView  `json:"task"`
	Committed committed `json:"committed"`
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

// adoptTimeZone makes the named IANA zone the process's local time. The bridge
// serves one device, so its zone is the process's; an empty name keeps
// whatever the process already has.
func adoptTimeZone(name string) error {
	if name == "" {
		return nil
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return newError(CodeInvalidRequest, "time_zone %q: %v", name, err)
	}
	time.Local = location
	return nil
}
