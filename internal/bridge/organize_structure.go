package bridge

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

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
