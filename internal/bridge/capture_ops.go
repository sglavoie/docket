package bridge

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

// capturePayload is one capture: either text in the capture grammar, read line
// by line exactly as the desktop composer reads it, or one structured task whose
// title is literal. project_id is the source project for both; absent is the
// protected Unfiled project.
type capturePayload struct {
	ProjectID string `json:"project_id,omitempty"`
	Text      string `json:"text,omitempty"`

	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	ResumeNote  string   `json:"resume_note,omitempty"`
	Destination string   `json:"destination,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Context     string   `json:"context,omitempty"`
	Due         string   `json:"due,omitempty"`
	Defer       string   `json:"defer,omitempty"`
	Pinned      bool     `json:"pinned,omitempty"`
	Review      bool     `json:"review,omitempty"`
}

// structured reports whether the payload names any structured field, which a
// grammar capture may not combine with its text.
func (p capturePayload) structured() bool {
	return p.Title != "" || p.Description != "" || p.ResumeNote != "" || p.Destination != "" ||
		p.Priority != 0 || len(p.Tags) > 0 || p.Context != "" || p.Due != "" || p.Defer != "" ||
		p.Pinned || p.Review
}

type captureResult struct {
	Task      taskView   `json:"task"`
	Tasks     []taskView `json:"tasks"`
	Committed committed  `json:"committed"`
}

// lineError is one refusal against one capture line, its column zero-based in
// runes and zero when no token owns it.
type lineError struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

func viewLineErrors(problems []capturetext.LineError) []lineError {
	out := make([]lineError, 0, len(problems))
	for _, problem := range problems {
		out = append(out, lineError{Line: problem.Line, Column: problem.Column, Message: problem.Message})
	}
	return out
}

// commandCapture creates every task the payload describes in one transaction.
// A grammar refusal names each refused line and commits nothing.
func (b *Bridge) commandCapture(request Request) (any, error) {
	var payload capturePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	hasText := strings.TrimSpace(payload.Text) != ""
	switch {
	case hasText && payload.structured():
		return nil, newError(CodeInvalidRequest, "text is the whole capture; it takes no structured fields beside project_id")
	case !hasText && payload.Title == "" && payload.Text != "":
		return nil, newError(CodeValidation, "no tasks to create").
			with("lines", []lineError{{Line: 1, Message: "no tasks to create"}})
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	source, err := b.captureSource(payload.ProjectID)
	if err != nil {
		return nil, err
	}
	var reqs []app.CaptureRequest
	if hasText {
		var problems []capturetext.LineError
		reqs, problems = b.captureRequests(payload.Text, source)
		if len(problems) > 0 {
			return nil, newError(CodeValidation, "%s", lineSummary(problems)).with("lines", viewLineErrors(problems))
		}
	} else {
		req, err := b.structuredCapture(payload, source)
		if err != nil {
			return nil, err
		}
		reqs = []app.CaptureRequest{req}
	}
	return b.commitCapture(reqs)
}

// captureSource resolves the project a capture starts in.
func (b *Bridge) captureSource(projectID string) (*models.Project, error) {
	state := b.workspace.runtime.State()
	if projectID == "" {
		unfiled := state.SystemUnfiledProject()
		if unfiled == nil {
			return nil, newError(CodeNotFound, "the workspace has no Unfiled project")
		}
		projectID = unfiled.ID
	}
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, err
	}
	project := snapshot.ProjectByID(projectID)
	if project == nil {
		return nil, newError(CodeNotFound, "project %s is not in the workspace", projectID).with("project_id", projectID)
	}
	return project, nil
}

// captureRequests reads capture text through the composer's own request
// builder, against the projects a capture may route to.
// The snapshot's projects are the available ones with their tasks loaded,
// which is what a parent, child, or dependency reference resolves against.
func (b *Bridge) captureRequests(text string, source *models.Project) ([]app.CaptureRequest, []capturetext.LineError) {
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, []capturetext.LineError{{Line: 1, Message: err.Error()}}
	}
	return capturetext.Requests(text, source, snapshot.Projects(), capturetext.Defaults{Now: b.now()})
}

func lineSummary(problems []capturetext.LineError) string {
	if len(problems) == 1 {
		return "line " + strconv.Itoa(problems[0].Line) + ": " + problems[0].Message
	}
	return strconv.Itoa(len(problems)) + " lines were refused; the first, line " + strconv.Itoa(problems[0].Line) + ": " + problems[0].Message
}

// structuredCapture builds one request from structured fields. Dates are read
// with the grammar's own value readers, and a deadline falling today routes to
// Today exactly as the desktop's expanded form does.
func (b *Bridge) structuredCapture(payload capturePayload, source *models.Project) (app.CaptureRequest, error) {
	req := app.CaptureRequest{
		ProjectID:   source.ID,
		Title:       payload.Title,
		Description: payload.Description,
		ResumeNote:  payload.ResumeNote,
		Priority:    payload.Priority,
		Tags:        payload.Tags,
		Context:     payload.Context,
		Pinned:      payload.Pinned,
		Review:      payload.Review,
		Prepend:     true,
	}
	if payload.Destination != "" {
		destination, ok := app.ParseCaptureDestination(payload.Destination)
		if !ok {
			return app.CaptureRequest{}, newError(CodeValidation, "%v", app.InvalidCaptureDestinationError(payload.Destination)).
				with("field", "destination")
		}
		req.Destination = destination
	}
	if payload.Due != "" {
		due, err := capturetext.ParseDueDateValue(payload.Due)
		if err != nil {
			return app.CaptureRequest{}, newError(CodeValidation, "invalid due date %q: %v", payload.Due, err).with("field", "due")
		}
		req.Due = &due
		if capturetext.DueIsToday(&due, b.now()) {
			req.Destination = app.CaptureDestinationToday
		}
	}
	if payload.Defer != "" {
		until, err := deferInstant(payload.Defer, b.now())
		if err != nil {
			return app.CaptureRequest{}, newError(CodeValidation, "invalid deferral %q: %v", payload.Defer, err).with("field", "defer")
		}
		req.DeferSet, req.Defer = true, &until
	}
	return req, nil
}

// commitCapture commits the requests as one transaction and reports every
// created task in request order.
func (b *Bridge) commitCapture(reqs []app.CaptureRequest) (captureResult, error) {
	runtime := b.workspace.runtime
	before, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return captureResult{}, err
	}
	results, err := runtime.Service().CaptureMany(reqs)
	if err != nil {
		return captureResult{}, err
	}
	after, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return captureResult{}, err
	}
	out := captureResult{Tasks: make([]taskView, 0, len(results))}
	out.Committed = committedEffect(results[0], before, after)
	for _, result := range results {
		out.Committed.Projects[result.ProjectID] = result.Revision
		read, err := b.readTask(app.TaskLocator{ProjectID: result.ProjectID, TaskID: result.TaskID})
		if err != nil {
			return captureResult{}, err
		}
		out.Tasks = append(out.Tasks, read.Task)
	}
	out.Task = out.Tasks[0]
	return out, nil
}

type capturePreviewPayload struct {
	ProjectID string `json:"project_id,omitempty"`
	Text      string `json:"text"`
}

// previewLine is what one capture line would create, named the way the
// desktop composer's route summary names it: a plan beside a status.
type previewLine struct {
	Line         int        `json:"line"`
	Title        string     `json:"title"`
	ProjectID    string     `json:"project_id"`
	ProjectName  string     `json:"project_name"`
	Destination  string     `json:"destination"`
	Plan         string     `json:"plan,omitempty"`
	State        string     `json:"state"`
	Priority     int        `json:"priority"`
	Tags         []string   `json:"tags"`
	Context      string     `json:"context,omitempty"`
	Due          *time.Time `json:"due,omitempty"`
	Defer        *time.Time `json:"defer,omitempty"`
	Pinned       bool       `json:"pinned"`
	Review       bool       `json:"review"`
	ParentID     string     `json:"parent_id,omitempty"`
	Children     int        `json:"children"`
	Dependencies int        `json:"dependencies"`
	Attachments  int        `json:"attachments"`
	Comments     int        `json:"comments"`
}

type capturePreviewResult struct {
	ProjectID string        `json:"project_id"`
	Lines     []previewLine `json:"lines"`
	Errors    []lineError   `json:"errors"`
}

// queryCapturePreview reads capture text without committing: what each line
// would create, or every refusal by line. A refused draft is an answer, not a
// failure, so it returns ok with its errors.
func (b *Bridge) queryCapturePreview(request Request) (any, error) {
	var payload capturePreviewPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := b.workspace.runtime.Refresh(); err != nil {
		return nil, err
	}
	source, err := b.captureSource(payload.ProjectID)
	if err != nil {
		return nil, err
	}
	result := capturePreviewResult{ProjectID: source.ID, Lines: []previewLine{}, Errors: []lineError{}}
	if strings.TrimSpace(payload.Text) == "" {
		return result, nil
	}
	reqs, problems := b.captureRequests(payload.Text, source)
	if len(problems) > 0 {
		result.Errors = viewLineErrors(problems)
		return result, nil
	}
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, err
	}
	for _, req := range reqs {
		line := previewLine{
			Line:         req.SourceLine,
			Title:        req.Title,
			ProjectID:    req.ProjectID,
			Destination:  string(req.Destination),
			State:        string(req.Destination.WorkflowState()),
			Priority:     req.Priority,
			Tags:         append([]string{}, req.Tags...),
			Context:      req.Context,
			Due:          req.Due,
			Defer:        req.Defer,
			Pinned:       req.Pinned,
			Review:       req.Review,
			Children:     len(req.ChildIDs),
			Dependencies: len(req.Dependencies),
			Attachments:  len(req.Attachments),
			Comments:     len(req.Comments),
		}
		if req.Destination.IsPlanning() {
			line.Plan = string(req.Destination)
		}
		if req.ParentID != nil {
			line.ParentID = *req.ParentID
		}
		if project := snapshot.ProjectByID(req.ProjectID); project != nil {
			line.ProjectName = project.Name
		}
		result.Lines = append(result.Lines, line)
	}
	return result, nil
}

// editPayload is one structured edit of one task. Every field but task is
// optional and an absent field is left as stored; an empty string clears a
// clearable one (description, resume note, context, due, defer). tags is the
// complete new set.
type editPayload struct {
	Task        locator   `json:"task"`
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	ResumeNote  *string   `json:"resume_note,omitempty"`
	Priority    *int      `json:"priority,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Context     *string   `json:"context,omitempty"`
	Pinned      *bool     `json:"pinned,omitempty"`
	Review      *bool     `json:"review,omitempty"`
	Due         *string   `json:"due,omitempty"`
	Defer       *string   `json:"defer,omitempty"`
}

type editResult struct {
	Task      taskView  `json:"task"`
	Changed   bool      `json:"changed"`
	Committed committed `json:"committed"`
}

// commandEdit applies every named field as one transaction through the same
// line-edit use case the desktop's inline editor submits, so each field passes
// that surface's validation. An edit that changes nothing commits nothing.
func (b *Bridge) commandEdit(request Request) (any, error) {
	var payload editPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	current, err := b.readTask(payload.Task.app())
	if err != nil {
		return nil, err
	}
	req := app.TaskLineEditRequest{
		Source:      payload.Task.app(),
		Title:       current.Task.Title,
		Description: payload.Description,
		ResumeNote:  payload.ResumeNote,
		Priority:    payload.Priority,
		Context:     payload.Context,
		Pinned:      payload.Pinned,
		Review:      payload.Review,
		Action:      "Edit Task",
	}
	if payload.Title != nil {
		req.Title = *payload.Title
	}
	if payload.Tags != nil {
		req.TagsToAdd, req.TagsToRemove, err = tagPatch(current.Task.Tags, *payload.Tags)
		if err != nil {
			return nil, err
		}
	}
	if payload.Due != nil {
		req.DueSet = true
		if *payload.Due != "" {
			due, dueErr := capturetext.ParseDueDateValue(*payload.Due)
			if dueErr != nil {
				return nil, newError(CodeValidation, "invalid due date %q: %v", *payload.Due, dueErr).with("field", "due")
			}
			req.Due = &due
		}
	}
	if payload.Defer != nil {
		req.DeferSet = true
		if *payload.Defer != "" {
			until, deferErr := deferInstant(*payload.Defer, b.now())
			if deferErr != nil {
				return nil, newError(CodeValidation, "invalid deferral %q: %v", *payload.Defer, deferErr).with("field", "defer")
			}
			req.Defer = &until
		}
	}
	runtime := b.workspace.runtime
	before, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return nil, err
	}
	result, err := runtime.Service().EditTaskLine(req)
	if err != nil {
		return nil, err
	}
	after, err := domainRevisions(runtime.DataDir())
	if err != nil {
		return nil, err
	}
	read, err := b.readTask(result.Task)
	if err != nil {
		return nil, err
	}
	out := editResult{Task: read.Task, Changed: result.Changed, Committed: committed{
		TransactionID: result.TransactionID,
		Projects:      map[string]string{},
		Domains:       map[string]string{},
	}}
	if result.Changed {
		out.Committed = committedEffect(app.TaskResult{
			ProjectID: read.Task.ProjectID, Revision: read.Revision, TransactionID: result.TransactionID,
		}, before, after)
	}
	return out, nil
}

// deferInstant reads a deferral as the edit grammar does, or as an RFC 3339
// instant, which is how a client names an exact moment such as one picked on
// a clock (a deferral is an instant, so the grammar's day and hour forms are
// spellings of one rather than the whole of what it may hold).
func deferInstant(value string, now time.Time) (time.Time, error) {
	if instant, err := time.Parse(time.RFC3339, value); err == nil {
		return instant, nil
	}
	return util.ParseDeferInstantAt(value, now)
}

// tagPatch turns a complete tag set into the additions and removals the line
// edit applies, normalizing both sides so a case change is no change.
func tagPatch(stored, wanted []string) (add, remove []string, err error) {
	normalized, err := models.NormalizeTaskTags(wanted)
	if err != nil {
		return nil, nil, newError(CodeValidation, "invalid tags: %v", err).with("field", "tags")
	}
	for _, tag := range normalized {
		if !slices.Contains(stored, tag) {
			add = append(add, tag)
		}
	}
	for _, tag := range stored {
		if !slices.Contains(normalized, tag) {
			remove = append(remove, tag)
		}
	}
	return add, remove, nil
}

type planPayload struct {
	Task   locator `json:"task"`
	Action string  `json:"action"`
}

// planActions are the canonical horizon intents a client may name.
var planActions = map[string]app.PlanningAction{
	string(app.PlanningAddToday):    app.PlanningAddToday,
	string(app.PlanningRemoveToday): app.PlanningRemoveToday,
	string(app.PlanningAddNext):     app.PlanningAddNext,
	string(app.PlanningRemoveNext):  app.PlanningRemoveNext,
	string(app.PlanningBacklog):     app.PlanningBacklog,
}

// commandPlan applies one planning action through the single planning
// boundary, which owns Today/Next exclusivity and every state side effect.
func (b *Bridge) commandPlan(request Request) (any, error) {
	var payload planPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	action, ok := planActions[payload.Action]
	if !ok {
		return nil, newError(CodeInvalidRequest, "action %q is not one of add_today, remove_today, add_next, remove_next, backlog", payload.Action)
	}
	return b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		return service.ApplyPlanning(app.PlanningRequest{Locator: payload.Task.app(), Action: action})
	})
}

func (b *Bridge) now() time.Time {
	if b.options.Clock != nil {
		return b.options.Clock.Now()
	}
	return time.Now()
}
