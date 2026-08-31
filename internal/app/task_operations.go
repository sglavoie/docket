package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TaskOperation names the small, presentation-neutral task operations used by
// the daily Outliner path. A presentation supplies project-qualified targets
// and a typed value; the service owns target resolution, validation, atomic
// mutation, and clipboard projection.
type TaskOperation string

const (
	TaskOperationEditTitle            TaskOperation = "edit_title"
	TaskOperationEditDescription      TaskOperation = "edit_description"
	TaskOperationCopyTitle            TaskOperation = "copy_title"
	TaskOperationCopyDescription      TaskOperation = "copy_description"
	TaskOperationCopyTitleDescription TaskOperation = "copy_title_description"
	TaskOperationCreateChild          TaskOperation = "create_child"
	TaskOperationSetState             TaskOperation = "set_state"
	TaskOperationSetPriority          TaskOperation = "set_priority"
	TaskOperationSetDueDate           TaskOperation = "set_due_date"
	TaskOperationSetDefer             TaskOperation = "set_defer"
	TaskOperationSetTags              TaskOperation = "set_tags"
	TaskOperationToggleTag            TaskOperation = "toggle_tag"
	TaskOperationSetPinned            TaskOperation = "set_pinned"
	TaskOperationSetDependencies      TaskOperation = "set_dependencies"
)

// TaskOperationRequest carries one operation and its value. Targets are always
// project-qualified so a cross-project presentation never depends on
// workspace-wide task-ID uniqueness. Text is used by title/description edit and
// child creation; Date is nil to clear due date or deferral.
type TaskOperationRequest struct {
	Operation TaskOperation
	Targets   []TaskLocator
	Text      string
	State     models.WorkflowState
	Priority  int
	Date      *time.Time
	// PlanForToday makes a non-nil due-date mutation carry explicit Today
	// planning intent in the same transaction. Interactive TUI due-date gestures
	// set it only when Date is today's calendar date.
	PlanForToday bool
	Tags         []string
	// TagPatch switches SetTags from replacement semantics to the tri-state
	// bulk editor's add/remove patch. Membership for tags in neither set is
	// retained independently on every target.
	TagPatch     bool
	TagsToAdd    []string
	TagsToRemove []string
	// ContextSet makes SetTags also write the singleton task context in the same
	// transaction, so a simultaneous tag and context edit is one undo entry. An
	// empty Context clears the field.
	ContextSet bool
	Context    string
	Tag        string
	Pinned     bool
	// Dependencies carries the soft-reference intent of SetDependencies. Its zero
	// value expresses nothing, so a request that never mentions a reference
	// preserves every target's stored set — the same preserve-by-default rule the
	// edit surfaces obey, applied to a selection of any size.
	Dependencies DependencyPatch
}

// TaskClipboardProjection is application-owned clipboard text. The application
// layer deliberately does not write to the system clipboard; adapters retain
// ownership of that platform side effect.
type TaskClipboardProjection struct {
	Operation TaskOperation
	Text      string
}

// TaskOperationResult is the common outcome for single-task and bulk daily
// operations. Mutations populate Tasks and TouchedProjectIDs; clipboard
// operations populate Clipboard and commit nothing.
type TaskOperationResult struct {
	Tasks             []TaskResult
	TouchedProjectIDs []string
	TransactionID     string
	Clipboard         *TaskClipboardProjection
}

// ExecuteTaskOperation is the one application path for daily Outliner task
// operations. Single-task mutations deliberately use the same bulk primitives
// as multi-selection, giving both cardinalities identical validation, rollback,
// transaction, and result semantics.
func (s *Service) ExecuteTaskOperation(req TaskOperationRequest) (TaskOperationResult, error) {
	switch req.Operation {
	case TaskOperationEditTitle:
		locator, err := singleOperationTarget(req)
		if err != nil {
			return TaskOperationResult{}, err
		}
		title := req.Text
		result, err := s.Edit(EditRequest{Locator: locator, Title: &title})
		return singleTaskOperationResult(result, err)

	case TaskOperationEditDescription:
		locator, err := singleOperationTarget(req)
		if err != nil {
			return TaskOperationResult{}, err
		}
		description := req.Text
		result, err := s.Edit(EditRequest{Locator: locator, Description: &description})
		return singleTaskOperationResult(result, err)

	case TaskOperationCopyTitle, TaskOperationCopyDescription, TaskOperationCopyTitleDescription:
		locator, err := singleOperationTarget(req)
		if err != nil {
			return TaskOperationResult{}, err
		}
		projection, err := s.projectTaskClipboard(locator, req.Operation)
		if err != nil {
			return TaskOperationResult{}, err
		}
		return TaskOperationResult{Clipboard: &projection}, nil

	case TaskOperationCreateChild:
		locator, err := singleOperationTarget(req)
		if err != nil {
			return TaskOperationResult{}, err
		}
		parentID := locator.TaskID
		result, err := s.Capture(CaptureRequest{
			ProjectID:   locator.ProjectID,
			Title:       req.Text,
			ParentID:    &parentID,
			Destination: CaptureDestinationBacklog,
		})
		return singleTaskOperationResult(result, err)

	case TaskOperationSetState:
		if err := validateOperationTargets(req.Targets); err != nil {
			return TaskOperationResult{}, err
		}
		result, err := s.BulkTransition(req.Targets, req.State)
		return bulkTaskOperationResult(result, err)

	case TaskOperationSetPriority:
		result, err := s.BulkSetPriority(req.Targets, req.Priority)
		return bulkTaskOperationResult(result, err)

	case TaskOperationSetDueDate:
		var (
			result BulkResult
			err    error
		)
		if req.Date == nil {
			result, err = s.BulkClearDueDate(req.Targets)
		} else if req.PlanForToday {
			result, err = s.BulkSetDueDateAndPlanToday(req.Targets, *req.Date)
		} else {
			result, err = s.BulkSetDueDate(req.Targets, *req.Date)
		}
		return bulkTaskOperationResult(result, err)

	case TaskOperationSetDefer:
		result, err := s.BulkDefer(req.Targets, req.Date)
		return bulkTaskOperationResult(result, err)

	case TaskOperationSetTags:
		var (
			result BulkResult
			err    error
		)
		edit := taskContextEdit{Set: req.ContextSet, Value: req.Context}
		if req.TagPatch {
			result, err = s.bulkPatchTags(req.Targets, req.TagsToAdd, req.TagsToRemove, edit)
		} else {
			result, err = s.bulkSetTags(req.Targets, req.Tags, edit)
		}
		return bulkTaskOperationResult(result, err)

	case TaskOperationToggleTag:
		result, err := s.BulkToggleTag(req.Targets, req.Tag)
		return bulkTaskOperationResult(result, err)

	case TaskOperationSetPinned:
		result, err := s.BulkSetPinned(req.Targets, req.Pinned)
		return bulkTaskOperationResult(result, err)

	case TaskOperationSetDependencies:
		result, err := s.bulkPatchDependencies(req.Targets, req.Dependencies)
		return bulkTaskOperationResult(result, err)

	default:
		return TaskOperationResult{}, invalidInput("operation", fmt.Sprintf("unknown task operation %q", req.Operation))
	}
}

func singleOperationTarget(req TaskOperationRequest) (TaskLocator, error) {
	if len(req.Targets) != 1 {
		return TaskLocator{}, invalidInput("tasks", fmt.Sprintf("%s requires exactly one task", req.Operation))
	}
	if !req.Targets[0].Valid() {
		return TaskLocator{}, invalidInput("task", "project id and task id are required")
	}
	return req.Targets[0], nil
}

func validateOperationTargets(targets []TaskLocator) error {
	if len(targets) == 0 {
		return invalidInput("tasks", "at least one task is required")
	}
	for _, locator := range targets {
		if !locator.Valid() {
			return invalidInput("task", "project id and task id are required")
		}
	}
	return nil
}

func singleTaskOperationResult(result TaskResult, err error) (TaskOperationResult, error) {
	if err != nil {
		return TaskOperationResult{}, err
	}
	return TaskOperationResult{
		Tasks:             []TaskResult{result},
		TouchedProjectIDs: []string{result.ProjectID},
		TransactionID:     result.TransactionID,
	}, nil
}

func bulkTaskOperationResult(result BulkResult, err error) (TaskOperationResult, error) {
	if err != nil {
		return TaskOperationResult{}, err
	}
	return TaskOperationResult{
		Tasks:             result.Tasks,
		TouchedProjectIDs: result.TouchedProjectIDs,
		TransactionID:     result.TransactionID,
	}, nil
}

func (s *Service) projectTaskClipboard(locator TaskLocator, operation TaskOperation) (TaskClipboardProjection, error) {
	ref, _, _, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskClipboardProjection{}, err
	}
	var text string
	switch operation {
	case TaskOperationCopyTitle:
		text = ref.Task.Title
	case TaskOperationCopyDescription:
		text = ref.Task.Description
	case TaskOperationCopyTitleDescription:
		text = ref.Task.Title + "\n\n" + ref.Task.Description
	default:
		return TaskClipboardProjection{}, invalidInput("operation", fmt.Sprintf("%q is not a clipboard projection", operation))
	}
	return TaskClipboardProjection{Operation: operation, Text: strings.Clone(text)}, nil
}
