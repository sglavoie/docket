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
	// TaskOperationStepDueDate walks every target's deadline one calendar place
	// along the deadline sequence. It is not a spelling of SetDueDate: one step
	// resolves a different destination per target — a date, today with its Today
	// planning, or no deadline at all — so it owns its own single-snapshot
	// calculation and reports what each target's deadline actually became.
	TaskOperationStepDueDate     TaskOperation = "step_due_date"
	TaskOperationSetDefer        TaskOperation = "set_defer"
	TaskOperationSetTags         TaskOperation = "set_tags"
	TaskOperationToggleTag       TaskOperation = "toggle_tag"
	TaskOperationSetPinned       TaskOperation = "set_pinned"
	TaskOperationToggleReview    TaskOperation = "toggle_review"
	TaskOperationSetDependencies TaskOperation = "set_dependencies"
	// TaskOperationRenameTaskName renames or merges one tag or context across the
	// whole workspace. It is the one operation here that carries no targets: its
	// scope is every unarchived project, resolved by a strict census rather than
	// by a presentation's selection.
	TaskOperationRenameTaskName TaskOperation = "rename_task_name"
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
	// DueStep carries the deadline-stepping intent of TaskOperationStepDueDate.
	// Its zero value names no direction, so every other operation leaves it alone
	// and a step request that forgot to set one is refused rather than defaulted.
	DueStep DueStepRequest
	Tags    []string
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
	// RenameName carries the whole-workspace tag or context rename. It is the one
	// value here that ignores Targets entirely.
	RenameName RenameTaskNameRequest
}

// TaskClipboardProjection is application-owned clipboard text. The application
// layer deliberately does not write to the system clipboard; adapters retain
// ownership of that platform side effect. TaskCount is how many tasks the text
// describes after deduplication, so an adapter reports the size of what it
// wrote rather than the size of what it asked for.
type TaskClipboardProjection struct {
	Operation TaskOperation
	Text      string
	TaskCount int
}

// TaskOperationResult is the common outcome for single-task and bulk daily
// operations. Mutations populate Tasks and TouchedProjectIDs; clipboard
// operations populate Clipboard and commit nothing.
type TaskOperationResult struct {
	Tasks             []TaskResult
	TouchedProjectIDs []string
	TransactionID     string
	Clipboard         *TaskClipboardProjection
	// DueSteps is populated by TaskOperationStepDueDate alone, one entry per
	// resolved target in resolution order. It is committed evidence: a caller
	// updates its own remembered state from the deadlines this transaction
	// replaced and wrote, never by re-reading a row the commit may have moved.
	DueSteps []DueStepOutcome
	// DueEdits names every task this transaction gave an explicit deadline
	// instruction — set or cleared — and is populated by TaskOperationSetDueDate
	// alone. It is deliberately separate from DueSteps: a step is the sequence
	// walking one place and reports where it landed, while an edit is the user
	// naming a deadline outright and overrides whatever the caller remembered.
	DueEdits []TaskLocator
	// Moves carries the committed source-to-destination address map of an
	// operation that relocated tasks between projects. Every operation that moves
	// nothing leaves it nil.
	Moves []TaskAddressChange
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
		if err := validateOperationTargets(req.Targets); err != nil {
			return TaskOperationResult{}, err
		}
		projection, err := s.projectTaskClipboard(req.Targets, req.Operation)
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
		operationResult, err := bulkTaskOperationResult(result, err)
		if err != nil {
			return TaskOperationResult{}, err
		}
		// Naming a deadline outright is the explicit edit, so every committed
		// target is reported as one — including the clear of a deadline that was
		// already absent, which changed no field but still overrode whatever the
		// caller was holding for that task.
		operationResult.DueEdits = taskResultLocators(operationResult.Tasks)
		return operationResult, nil

	case TaskOperationStepDueDate:
		return s.stepDueDates(req.Targets, req.DueStep)

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

	case TaskOperationToggleReview:
		result, err := s.bulkToggleReview(req.Targets)
		return bulkTaskOperationResult(result, err)

	case TaskOperationRenameTaskName:
		result, err := s.RenameTaskName(req.RenameName)
		if err != nil {
			return TaskOperationResult{}, err
		}
		return TaskOperationResult{
			Tasks:             result.Tasks,
			TouchedProjectIDs: result.TouchedProjectIDs,
			TransactionID:     result.TransactionID,
		}, nil

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
		Moves:             result.Moves,
	}, nil
}

// projectTaskClipboard renders one or many tasks as a single clipboard value.
// Every target resolves against one snapshot, so the text describes one
// consistent workspace state, and the whole text is assembled before it is
// returned: a target the snapshot cannot resolve fails the entire projection
// rather than handing the adapter the part that did resolve. Order is the
// caller's — the presentation owns what the copied text reads like — and a task
// named twice is copied once, because resolveRequestedTasks deduplicates by
// project-qualified locator while preserving first-seen order.
func (s *Service) projectTaskClipboard(locators []TaskLocator, operation TaskOperation) (TaskClipboardProjection, error) {
	block, separator, err := taskClipboardFormat(operation)
	if err != nil {
		return TaskClipboardProjection{}, err
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskClipboardProjection{}, err
	}
	refs, err := resolveRequestedTasks(snap, nil, locators)
	if err != nil {
		return TaskClipboardProjection{}, err
	}
	blocks := make([]string, 0, len(refs))
	for _, ref := range refs {
		blocks = append(blocks, block(ref.Task))
	}
	return TaskClipboardProjection{
		Operation: operation,
		// Cloned because strings.Join returns its one element unchanged, which
		// would otherwise alias the snapshot's task field.
		Text:      strings.Clone(strings.Join(blocks, separator)),
		TaskCount: len(refs),
	}, nil
}

// taskClipboardFormat pairs each projection's per-task block with the separator
// that joins a selection of them. One switch owns both halves so a projection
// cannot declare what one task looks like and leave how several read undecided.
// Every block is exactly what a single task copied on its own has always
// produced, the title-and-description separator included.
func taskClipboardFormat(operation TaskOperation) (func(*models.Task) string, string, error) {
	switch operation {
	case TaskOperationCopyTitle:
		return func(task *models.Task) string { return task.Title }, "\n", nil
	case TaskOperationCopyDescription:
		return func(task *models.Task) string { return task.Description }, "\n\n", nil
	case TaskOperationCopyTitleDescription:
		return func(task *models.Task) string {
			return task.Title + "\n\n" + task.Description
		}, "\n\n", nil
	default:
		return nil, "", invalidInput("operation", fmt.Sprintf("%q is not a clipboard projection", operation))
	}
}
