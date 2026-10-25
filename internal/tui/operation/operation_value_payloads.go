package operation

import (
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/query"
)

// OperationParentValue is the picker-neutral reparent payload. A nil ParentID
// is the project root, which is what the `>0` edit sigil expresses.
type OperationParentValue struct {
	ParentID *string
}

// OperationDateValue is the picker-neutral due/defer payload. Nil clears the
// field. Deferred names which of the two fields the value writes, because one
// picker answers both.
type OperationDateValue struct {
	Date     *time.Time
	Deferred bool
}

// OperationCheckInEditorAction is the typed row action exposed by the combined
// scheduling picker. It is deliberately not a date value: choosing the row
// opens the reusable two-field editor and can therefore never be mistaken for
// a due-date or deferral mutation by the resumed deadline operation.
type OperationCheckInEditorAction struct{}

// OperationCheckInValue is the editor-neutral check-in payload. Clear records
// the explicit choice to remove a schedule; an absent date without Clear is not
// an instruction and is refused by the editor. The input spellings travel only
// so a failed asynchronous save can restore the exact draft the user submitted.
type OperationCheckInValue struct {
	Schedule    models.CheckInSchedule
	Clear       bool
	DateInput   string
	RepeatInput string
}

// OperationTagsValue is the tri-state bulk tag edit emitted by the value
// picker. Add and Remove are uniform patches; tags in neither set preserve
// their existing per-task membership. ContextSet carries the singleton task
// context in the same payload so both land in one transaction; an empty Context
// clears the field.
type OperationTagsValue struct {
	Add        []string
	Remove     []string
	ContextSet bool
	Context    string
}

// OperationTaskReferenceValue is the soft-reference edit emitted by the `~`
// picker. It is a difference rather than the final checked set: Add names the
// rows the user checked that the picker had not preselected, Remove the
// preselected rows the user unchecked, and a reference in neither half is one
// the user never touched and the commit must leave alone.
type OperationTaskReferenceValue struct {
	Add    []string
	Remove []string
}

// AcceptsPresetValue reports whether a key binding may deliver value directly
// instead of opening the provider's picker. It mirrors the type assertions the
// task handlers below make, so a mistyped preset is rejected when the input
// strategy is registered rather than when the user presses the chord.
func (id ValueProviderID) AcceptsPresetValue(value any) bool {
	switch id {
	case ValueProviderState:
		state, ok := value.(models.WorkflowState)
		return ok && state.IsValid()
	case ValueProviderPriority, ValueProviderFilterPriority:
		_, ok := value.(int)
		return ok
	case ValueProviderFilterDue:
		// A due preset is the token one row writes, so a binding that names a
		// spelling no row offers is rejected at registration.
		token, ok := value.(string)
		if !ok {
			return false
		}
		_, known := query.DueFilterShortcutForToken(token)
		return known
	case ValueProviderFilterDependency:
		// The f~ rows are their own canonical tokens too, so the same registration
		// check keeps a binding from naming a spelling no row offers.
		token, ok := value.(string)
		if !ok {
			return false
		}
		_, known := query.DependencyFilterShortcutForToken(token)
		return known
	case ValueProviderFilterCompletionAge:
		_, ok := value.(query.CompletionAgeFilter)
		return ok
	case ValueProviderFilterStates,
		ValueProviderFilterTags,
		ValueProviderFilterContexts,
		ValueProviderFilterProjects,
		ValueProviderFilterProjectTags:
		_, ok := value.([]string)
		return ok
	case ValueProviderProjectTagDestination, ValueProviderProject:
		_, ok := value.(string)
		return ok
	case ValueProviderDate, ValueProviderDueDate:
		_, ok := value.(OperationDateValue)
		return ok
	case ValueProviderParent:
		_, ok := value.(OperationParentValue)
		return ok
	case ValueProviderChild:
		_, ok := value.([]string)
		return ok
	case ValueProviderTags:
		if _, ok := value.([]string); ok {
			return true
		}
		_, ok := value.(OperationTagsValue)
		return ok
	case ValueProviderNoteTitle, ValueProviderTagName:
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}
