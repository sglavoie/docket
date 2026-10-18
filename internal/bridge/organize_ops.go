package bridge

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
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
