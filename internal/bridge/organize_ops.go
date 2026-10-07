package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// The organizing ops: checklists, check-ins, the outline's hierarchy, actions
// over a selection, and projects. Each names the same use case the desktop's
// key does, so the shared engine judges both.

// tasksResultView is what an op touching several tasks reports: every task it
// named as it now stands, the address of every task it moved, and the commit.
type tasksResultView struct {
	Tasks     []taskView    `json:"tasks"`
	Moves     []addressMove `json:"moves"`
	Committed committed     `json:"committed"`
}

// addressMove is one task's address before and after a cross-project move.
type addressMove struct {
	From locator `json:"from"`
	To   locator `json:"to"`
}

// commitMany runs one mutation and reports what it touched. The mutation
// returns the locators to read back, the projects it touched, its transaction,
// and its moves.
func (b *Bridge) commitMany(mutate func(*app.Service) (manyEffect, error)) (tasksResultView, error) {
	runtime := b.workspace.runtime
	before, err := currentRevisions(runtime)
	if err != nil {
		return tasksResultView{}, err
	}
	effect, err := mutate(runtime.Service())
	if err != nil {
		return tasksResultView{}, err
	}
	return b.reportMany(effect, before)
}

// manyEffect is the committed evidence of one multi-task mutation.
type manyEffect struct {
	targets       []app.TaskLocator
	transactionID string
	moves         []app.TaskAddressChange
}

// changedRevisions is every revision after that differs from before: what a
// commit rewrote, including the projects a reference rewrite reached.
func changedRevisions(before, after map[string]string) map[string]string {
	changed := make(map[string]string)
	for name, revision := range after {
		if before[name] != revision {
			changed[name] = revision
		}
	}
	return changed
}

func effectOf(result app.TaskOperationResult) manyEffect {
	effect := manyEffect{transactionID: result.TransactionID, moves: result.Moves}
	for _, task := range result.Tasks {
		effect.targets = append(effect.targets, app.TaskLocator{ProjectID: task.ProjectID, TaskID: task.TaskID})
	}
	return effect
}

func bulkEffectOf(result app.BulkResult) manyEffect {
	effect := manyEffect{transactionID: result.TransactionID, moves: result.Moves}
	for _, task := range result.Tasks {
		effect.targets = append(effect.targets, app.TaskLocator{ProjectID: task.ProjectID, TaskID: task.TaskID})
	}
	return effect
}

func (b *Bridge) reportMany(effect manyEffect, before revisions) (tasksResultView, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return tasksResultView{}, err
	}
	snapshot, err := runtime.Snapshot()
	if err != nil {
		return tasksResultView{}, err
	}
	after, err := currentRevisions(runtime)
	if err != nil {
		return tasksResultView{}, err
	}
	out := tasksResultView{
		Tasks: []taskView{},
		Moves: []addressMove{},
		Committed: committed{
			TransactionID: effect.transactionID,
			Projects:      changedRevisions(before.Projects, after.Projects),
			Domains:       changedRevisions(before.Domains, after.Domains),
		},
	}
	for _, target := range effect.targets {
		if ref, ok := snapshot.LookupLocator(target); ok {
			out.Tasks = append(out.Tasks, viewTask(ref.Project.ID, ref.Task))
		}
	}
	for _, move := range effect.moves {
		out.Moves = append(out.Moves, addressMove{
			From: locator{ProjectID: move.From.ProjectID, TaskID: move.From.TaskID},
			To:   locator{ProjectID: move.To.ProjectID, TaskID: move.To.TaskID},
		})
	}
	return out, nil
}

// targetsOf validates and converts a selection.
func targetsOf(field string, tasks []locator) ([]app.TaskLocator, error) {
	if len(tasks) == 0 {
		return nil, newError(CodeInvalidRequest, "%s names no task", field)
	}
	targets := make([]app.TaskLocator, 0, len(tasks))
	for _, task := range tasks {
		if err := task.validate(field); err != nil {
			return nil, err
		}
		targets = append(targets, task.app())
	}
	return targets, nil
}

// checklistPayload is one checklist action on one task. An item has no stored
// identity, so every action naming one by index also carries the complete
// checklist as the client read it, and is refused if the task's checklist is
// no longer exactly that: a concurrent edit, tick, reorder, or removal would
// otherwise move the index onto another item.
type checklistPayload struct {
	Task      locator      `json:"task"`
	Action    string       `json:"action"`
	Text      string       `json:"text,omitempty"`
	Index     *int         `json:"index,omitempty"`
	To        *int         `json:"to,omitempty"`
	Checklist *[]checkItem `json:"checklist,omitempty"`
}

type checklistResult struct {
	Task      taskView  `json:"task"`
	Child     *taskView `json:"child,omitempty"`
	Committed committed `json:"committed"`
}

// commandChecklist adds, edits, ticks, removes, moves, or promotes one
// checklist item.
func (b *Bridge) commandChecklist(request Request) (any, error) {
	var payload checklistPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	target := payload.Task.app()
	if payload.Action == "add" {
		return b.commitChecklist(func(service *app.Service) (app.TaskResult, *app.TaskResult, error) {
			result, err := service.AddChecklistItem(target, payload.Text)
			return result.Task, nil, err
		})
	}
	if payload.Index == nil || payload.Checklist == nil {
		return nil, newError(CodeInvalidRequest, "checklist %s needs index and the checklist as read", payload.Action)
	}
	index := *payload.Index
	frozen := make([]models.ChecklistItem, 0, len(*payload.Checklist))
	for _, item := range *payload.Checklist {
		frozen = append(frozen, models.ChecklistItem{Text: item.Text, Done: item.Done})
	}
	if payload.Action == "promote" {
		return b.commitChecklist(func(service *app.Service) (app.TaskResult, *app.TaskResult, error) {
			result, err := service.PromoteChecklistItem(app.ChecklistPromotionRequest{Owner: target, Index: index, Checklist: frozen})
			return result.Owner, &result.Child, err
		})
	}
	var act func(*app.Service) (app.ChecklistResult, error)
	switch payload.Action {
	case "edit":
		act = func(service *app.Service) (app.ChecklistResult, error) {
			return service.EditChecklistItem(target, index, payload.Text)
		}
	case "toggle":
		act = func(service *app.Service) (app.ChecklistResult, error) {
			return service.ToggleChecklistItem(target, index)
		}
	case "remove":
		act = func(service *app.Service) (app.ChecklistResult, error) {
			return service.RemoveChecklistItem(target, index)
		}
	case "move":
		if payload.To == nil {
			return nil, newError(CodeInvalidRequest, "checklist move needs to")
		}
		act = func(service *app.Service) (app.ChecklistResult, error) {
			return service.MoveChecklistItem(target, index, *payload.To)
		}
	default:
		return nil, newError(CodeInvalidRequest, "action %q is not one of add, edit, toggle, remove, move, promote", payload.Action)
	}
	current, err := b.readTask(target)
	if err != nil {
		return nil, err
	}
	if !sameChecklist(current.Task.Checklist, frozen) {
		return nil, &app.InvalidInputError{
			Field:  "checklist",
			Reason: fmt.Sprintf("the checklist of #%d changed since it was read; nothing was changed", current.Task.ShortID),
		}
	}
	return b.commitChecklist(func(service *app.Service) (app.TaskResult, *app.TaskResult, error) {
		result, err := act(service)
		return result.Task, nil, err
	})
}

func sameChecklist(current []checkItem, frozen []models.ChecklistItem) bool {
	if len(current) != len(frozen) {
		return false
	}
	for i, item := range current {
		if item.Text != frozen[i].Text || item.Done != frozen[i].Done {
			return false
		}
	}
	return true
}

func (b *Bridge) commitChecklist(mutate func(*app.Service) (app.TaskResult, *app.TaskResult, error)) (checklistResult, error) {
	var child *app.TaskResult
	owner, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		result, promoted, err := mutate(service)
		child = promoted
		return result, err
	})
	if err != nil {
		return checklistResult{}, err
	}
	out := checklistResult{Task: owner.Task, Committed: owner.Committed}
	if child != nil {
		read, err := b.readTask(app.TaskLocator{ProjectID: child.ProjectID, TaskID: child.TaskID})
		if err != nil {
			return checklistResult{}, err
		}
		out.Child = &read.Task
	}
	return out, nil
}

// checkInPayload schedules, clears, or acknowledges check-ins. set and clear
// take any selection; acknowledge names one task, as the review key does.
type checkInPayload struct {
	Tasks      []locator `json:"tasks"`
	Action     string    `json:"action"`
	Date       string    `json:"date,omitempty"`
	RepeatDays int       `json:"repeat_days,omitempty"`
}

// commandCheckIn writes one schedule across a selection, or acknowledges one
// task's review, which moves a repeating schedule on from today and clears a
// one-off.
func (b *Bridge) commandCheckIn(request Request) (any, error) {
	var payload checkInPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	targets, err := targetsOf("tasks", payload.Tasks)
	if err != nil {
		return nil, err
	}
	var schedule models.CheckInSchedule
	switch payload.Action {
	case "acknowledge":
		if len(targets) != 1 {
			return nil, newError(CodeInvalidRequest, "acknowledge names exactly one task")
		}
		return b.commitMany(func(service *app.Service) (manyEffect, error) {
			result, err := service.MarkReviewed(targets[0])
			return manyEffect{targets: targets, transactionID: result.TransactionID}, err
		})
	case "clear":
		if payload.Date != "" || payload.RepeatDays != 0 {
			return nil, newError(CodeInvalidRequest, "clear takes no date or repeat")
		}
	case "set":
		next, parseErr := util.ParseCheckInDateAt(payload.Date, b.now())
		if parseErr != nil {
			return nil, &app.InvalidInputError{Field: "check_in", Reason: parseErr.Error()}
		}
		schedule = models.CheckInSchedule{Next: &next, RepeatDays: payload.RepeatDays}
		if err := models.ValidateCheckInSchedule(schedule); err != nil {
			return nil, &app.InvalidInputError{Field: "check_in", Reason: err.Error()}
		}
	default:
		return nil, newError(CodeInvalidRequest, "action %q is not one of set, clear, acknowledge", payload.Action)
	}
	return b.commitMany(func(service *app.Service) (manyEffect, error) {
		result, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
			Operation: app.TaskOperationSetCheckIn, Targets: targets, CheckIn: schedule,
		})
		effect := effectOf(result)
		effect.targets = targets
		return effect, err
	})
}

// hierarchyPayload is one change to a task's place in its project's outline.
type hierarchyPayload struct {
	Task   locator  `json:"task"`
	Action string   `json:"action"`
	Title  string   `json:"title,omitempty"`
	Parent *locator `json:"parent,omitempty"`
}

type hierarchyResult struct {
	Task      taskView  `json:"task"`
	Created   *taskView `json:"created,omitempty"`
	Committed committed `json:"committed"`
}

// commandHierarchy adds a subtask, sets or clears a parent, indents, outdents,
// or moves a task among its siblings. The engine holds the depth limit and
// refuses a cycle.
func (b *Bridge) commandHierarchy(request Request) (any, error) {
	var payload hierarchyPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if err := payload.Task.validate("task"); err != nil {
		return nil, err
	}
	target := payload.Task.app()
	var mutate func(*app.Service) (app.TaskResult, error)
	switch payload.Action {
	case "add_child":
		created, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
			result, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationCreateChild, Targets: []app.TaskLocator{target}, Text: payload.Title,
			})
			if err != nil {
				return app.TaskResult{}, err
			}
			if len(result.Tasks) != 1 {
				return app.TaskResult{}, fmt.Errorf("bridge: a subtask creation reported %d tasks", len(result.Tasks))
			}
			return result.Tasks[0], nil
		})
		if err != nil {
			return nil, err
		}
		parent, err := b.readTask(target)
		if err != nil {
			return nil, err
		}
		return hierarchyResult{Task: parent.Task, Created: &created.Task, Committed: created.Committed}, nil
	case "set_parent":
		if payload.Parent == nil {
			return nil, newError(CodeInvalidRequest, "set_parent needs parent")
		}
		if err := payload.Parent.validate("parent"); err != nil {
			return nil, err
		}
		if payload.Parent.ProjectID != target.ProjectID {
			return nil, &app.InvalidInputError{Field: "parent", Reason: "a parent must be in the same project; move the task first"}
		}
		parentID := payload.Parent.TaskID
		mutate = func(service *app.Service) (app.TaskResult, error) { return service.Reparent(target, &parentID) }
	case "clear_parent":
		mutate = func(service *app.Service) (app.TaskResult, error) { return service.Reparent(target, nil) }
	case "indent":
		mutate = func(service *app.Service) (app.TaskResult, error) { return service.IndentTask(target) }
	case "outdent":
		mutate = func(service *app.Service) (app.TaskResult, error) { return service.OutdentTask(target) }
	case "move_up":
		mutate = func(service *app.Service) (app.TaskResult, error) { return service.MoveTaskUp(target) }
	case "move_down":
		mutate = func(service *app.Service) (app.TaskResult, error) { return service.MoveTaskDown(target) }
	default:
		return nil, newError(CodeInvalidRequest,
			"action %q is not one of add_child, set_parent, clear_parent, indent, outdent, move_up, move_down", payload.Action)
	}
	result, err := b.commitTask(mutate)
	if err != nil {
		return nil, err
	}
	return hierarchyResult{Task: result.Task, Committed: result.Committed}, nil
}

// bulkPayload is one action over a selection of any size: a planning action,
// a status, a tag patch, or a destination project.
type bulkPayload struct {
	Tasks      []locator `json:"tasks"`
	Action     string    `json:"action"`
	Plan       string    `json:"plan,omitempty"`
	State      string    `json:"state,omitempty"`
	AddTags    []string  `json:"add_tags,omitempty"`
	RemoveTags []string  `json:"remove_tags,omitempty"`
	ProjectID  string    `json:"project_id,omitempty"`
}

// commandBulk applies one action to a whole selection as one transaction and
// one undo entry; any refusal refuses the selection entire.
func (b *Bridge) commandBulk(request Request) (any, error) {
	var payload bulkPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	targets, err := targetsOf("tasks", payload.Tasks)
	if err != nil {
		return nil, err
	}
	switch payload.Action {
	case "plan":
		action, ok := planActions[payload.Plan]
		if !ok {
			return nil, newError(CodeInvalidRequest, "plan %q is not one of add_today, remove_today, add_next, remove_next, backlog", payload.Plan)
		}
		return b.commitMany(func(service *app.Service) (manyEffect, error) {
			result, err := service.ApplyPlanningBulk(app.PlanningBulkRequest{Targets: targets, Action: action})
			return bulkEffectOf(result), err
		})
	case "state":
		return b.commitMany(func(service *app.Service) (manyEffect, error) {
			result, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetState, Targets: targets, State: models.WorkflowState(payload.State),
			})
			return effectOf(result), err
		})
	case "tags":
		if len(payload.AddTags) == 0 && len(payload.RemoveTags) == 0 {
			return nil, newError(CodeInvalidRequest, "tags needs add_tags or remove_tags")
		}
		return b.commitMany(func(service *app.Service) (manyEffect, error) {
			result, err := service.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationSetTags, Targets: targets,
				TagPatch: true, TagsToAdd: payload.AddTags, TagsToRemove: payload.RemoveTags,
			})
			return effectOf(result), err
		})
	case "move":
		if payload.ProjectID == "" {
			return nil, newError(CodeInvalidRequest, "move needs project_id")
		}
		return b.commitMany(func(service *app.Service) (manyEffect, error) {
			return b.moveTasks(service, targets, payload.ProjectID)
		})
	}
	return nil, newError(CodeInvalidRequest, "action %q is not one of plan, state, tags, move", payload.Action)
}

// moveTasks moves a selection into one project. Cardinality alone chooses the
// transaction, as on the desktop: one task takes the single task-line edit,
// two or more the batch, and neither changes what the move means.
func (b *Bridge) moveTasks(service *app.Service, targets []app.TaskLocator, projectID string) (manyEffect, error) {
	if len(targets) > 1 {
		result, err := service.BulkMoveToProject(targets, projectID)
		return bulkEffectOf(result), err
	}
	current, err := b.readTask(targets[0])
	if err != nil {
		return manyEffect{}, err
	}
	result, err := service.EditTaskLine(app.TaskLineEditRequest{
		Source:               targets[0],
		Title:                current.Task.Title,
		DestinationProjectID: projectID,
		Action:               "Move to Project",
	})
	if err != nil {
		return manyEffect{}, err
	}
	return manyEffect{
		targets:       []app.TaskLocator{result.Task},
		transactionID: result.TransactionID,
		moves:         result.Moves,
	}, nil
}

// impactView is what deleting a project would remove, with a digest of the
// project's stored files. A delete must echo the impact it showed, so one
// that another device's work changed in the meantime is refused rather than
// taking more than the person agreed to.
type impactView struct {
	Tasks       int    `json:"tasks"`
	Notes       int    `json:"notes"`
	Attachments int    `json:"attachments"`
	Digest      string `json:"digest"`
}

type projectDetail struct {
	Project projectView `json:"project"`
	Impact  impactView  `json:"impact"`
}

type projectPayload struct {
	ProjectID string `json:"project_id"`
}

// queryProject reads one project, archived ones included, with what deleting
// it would remove.
func (b *Bridge) queryProject(request Request) (any, error) {
	var payload projectPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	if payload.ProjectID == "" {
		return nil, newError(CodeInvalidRequest, "project_id is required")
	}
	return b.readProject(payload.ProjectID)
}

func (b *Bridge) readProject(projectID string) (projectDetail, error) {
	runtime := b.workspace.runtime
	project, impact, err := runtime.ProjectImpact(projectID)
	if err != nil {
		return projectDetail{}, err
	}
	digest, err := projectDigest(project.Path, impact)
	if err != nil {
		return projectDetail{}, err
	}
	return projectDetail{
		Project: viewProject(runtime.State(), project),
		Impact:  impactView{Tasks: impact.Tasks, Notes: impact.Notes, Attachments: impact.Attachments, Digest: digest},
	}, nil
}

// projectDigest fingerprints the stored task Markdown beside the counts, so
// an edit that kept every count still changes it.
func projectDigest(projectPath string, impact engine.ProjectImpact) (string, error) {
	data, err := os.ReadFile(storage.TasksFilePath(projectPath))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%d|%d|%d", storage.RawArtifactRevision(data), impact.Tasks, impact.Notes, impact.Attachments))
	return hex.EncodeToString(sum[:]), nil
}

// projectCommandPayload is one project lifecycle action. edit keeps whatever
// it does not name; tags is always the complete set.
type projectCommandPayload struct {
	Action      string      `json:"action"`
	ProjectID   string      `json:"project_id,omitempty"`
	Name        *string     `json:"name,omitempty"`
	Description *string     `json:"description,omitempty"`
	Tags        *[]string   `json:"tags,omitempty"`
	Confirm     *impactView `json:"confirm,omitempty"`
}

type projectResult struct {
	Project   projectView `json:"project"`
	Committed committed   `json:"committed"`
}

// commandProject creates, edits, retags, parks, archives, restores, or deletes
// a project through the engine's one project lifecycle.
func (b *Bridge) commandProject(request Request) (any, error) {
	var payload projectCommandPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	runtime := b.workspace.runtime
	if payload.Action != "create" && payload.ProjectID == "" {
		return nil, newError(CodeInvalidRequest, "%s needs project_id", payload.Action)
	}
	var mutate func() (engine.ProjectChange, error)
	switch payload.Action {
	case "create":
		if payload.Name == nil {
			return nil, newError(CodeInvalidRequest, "create needs name")
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.CreateProject(*payload.Name) }
	case "edit":
		current, err := b.readProject(payload.ProjectID)
		if err != nil {
			return nil, err
		}
		edit := engine.ProjectEdit{Name: current.Project.Name, Description: current.Project.Description, Tags: current.Project.Tags}
		if payload.Name != nil {
			edit.Name = *payload.Name
		}
		if payload.Description != nil {
			edit.Description = *payload.Description
		}
		if payload.Tags != nil {
			edit.Tags = *payload.Tags
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.EditProject(payload.ProjectID, edit) }
	case "set_tags":
		if payload.Tags == nil {
			return nil, newError(CodeInvalidRequest, "set_tags needs tags")
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.SetProjectTags(payload.ProjectID, *payload.Tags) }
	case "archive":
		mutate = func() (engine.ProjectChange, error) { return runtime.ArchiveProject(payload.ProjectID) }
	case "unarchive":
		mutate = func() (engine.ProjectChange, error) { return runtime.UnarchiveProject(payload.ProjectID) }
	case "delete":
		if err := b.confirmDelete(payload); err != nil {
			return nil, err
		}
		mutate = func() (engine.ProjectChange, error) { return runtime.DeleteProject(payload.ProjectID) }
	default:
		return nil, newError(CodeInvalidRequest, "action %q is not one of create, edit, set_tags, archive, unarchive, delete", payload.Action)
	}
	return b.commitProject(mutate)
}

// confirmDelete refuses a delete that does not echo the project's current
// impact exactly.
func (b *Bridge) confirmDelete(payload projectCommandPayload) error {
	if payload.Confirm == nil {
		return newError(CodeInvalidRequest, "delete needs confirm: the impact the confirmation showed")
	}
	current, err := b.readProject(payload.ProjectID)
	if err != nil {
		return err
	}
	if *payload.Confirm != current.Impact {
		return newError(CodeStaleRevision, "%s changed since the delete was confirmed; nothing was deleted", current.Project.Name).
			with("project_id", payload.ProjectID).with("impact", current.Impact)
	}
	return nil
}

func (b *Bridge) commitProject(mutate func() (engine.ProjectChange, error)) (projectResult, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return projectResult{}, err
	}
	before, err := currentRevisions(runtime)
	if err != nil {
		return projectResult{}, err
	}
	change, err := mutate()
	if err != nil {
		return projectResult{}, err
	}
	effect, err := b.reportMany(manyEffect{transactionID: change.TransactionID}, before)
	if err != nil {
		return projectResult{}, err
	}
	project := change.Project
	if stored := runtime.State().GetProject(project.ID); stored != nil {
		project = stored
	}
	return projectResult{Project: viewProject(runtime.State(), project), Committed: effect.Committed}, nil
}
