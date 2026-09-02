package tui

import (
	"fmt"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const (
	operationTaskSetState    OperationID = "tui.task.set-state"
	operationTaskSetPriority OperationID = "tui.task.set-priority"
	operationTaskSetDueDate  OperationID = "tui.task.set-due-date"
	operationTaskSetTags     OperationID = "tui.task.set-tags"
	operationTaskSetParent   OperationID = "tui.task.set-parent"
	operationTaskIndent      OperationID = "tui.task.indent"
	operationTaskOutdent     OperationID = "tui.task.outdent"

	// operationTaskAdoptChildren is the `<child` sigil's key. It is a separate
	// operation from set-parent because the sigil is repeatable and additive,
	// which set-parent's single-valued replace semantics cannot express.
	operationTaskAdoptChildren OperationID = "tui.task.adopt-children"

	// operationTaskMoveToProject is the `%project` sigil's key. It travels the
	// same cross-project edit path the sigil does rather than adding a second
	// relocation route.
	operationTaskMoveToProject OperationID = "tui.task.move-to-project"

	// operationTaskReferences is the `~task` sigil's key. It is a separate
	// operation from every other relationship above because the sigil it answers
	// is unlike theirs on three counts: it is repeatable, so no single-valued
	// replace can express it; it crosses a project boundary, so the candidate
	// population is not one project's tree; and it is soft, so nothing about
	// readiness, ordering, ranking, or completion may read what it records.
	operationTaskReferences OperationID = "tui.task.references"
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

type operationValueMembership uint8

const (
	operationValueNone operationValueMembership = iota
	operationValueSome
	operationValueAll
)

// OperationValueSelectedMsg resumes a pending operation with the chosen value.
type OperationValueSelectedMsg struct {
	Request OperationExecutionRequest
	Value   any
}

// OperationValueCanceledMsg closes the value picker and returns to its action
// sheet without executing the operation.
type OperationValueCanceledMsg struct{}

// OperationValuePickerModel is the shared ListSurface value-picker control.
type OperationValuePickerModel struct {
	surface   *ListSurfaceModel
	request   OperationExecutionRequest
	provider  ValueProviderID
	title     string
	subtitle  string
	multi     bool
	searching bool
	// membershipInitial and membershipCurrent are the tri-state editor's two
	// halves: what the selection agreed on when the picker opened, and what it
	// says now. Both the tag picker and the reference picker read them, because
	// both edit a per-task set over a selection and both must emit a difference
	// rather than a replacement.
	membershipInitial map[ListItemID]operationValueMembership
	membershipCurrent map[ListItemID]operationValueMembership
	tagAdder          TagAdder
	dateAdder         DateAdder
	// dateDeferred records which section the inline date mode was opened from,
	// so the typed value writes the same field the row under the cursor names.
	dateDeferred bool
	// contextEntry records that the inline add mode was opened from a context
	// row, so the typed name mints a context instead of a tag.
	contextEntry bool
	// contextInitial is the context row every target already agreed on. An empty
	// ID means the targets disagreed, so any chosen row is a change.
	contextInitial ListItemID
}

func newOperationValuePickerModel(
	request OperationExecutionRequest,
	provider ValueProviderID,
	title string,
	subtitle string,
	source ListSource,
	multi bool,
	selected []ListItemID,
) (*OperationValuePickerModel, error) {
	mode := ListSelectionSingle
	if multi {
		mode = ListSelectionMulti
	}
	surface, err := source.NewModel(ListSurfaceConfig{
		SelectionMode: mode,
		EmptyState: ListEmptyState{
			Title: "No values available.",
		},
		NoMatchState: ListEmptyState{
			Title: "No matching values.",
		},
	})
	if err != nil {
		return nil, err
	}
	if len(selected) > 0 {
		if multi {
			if err := surface.SetSelectedIDs(selected); err != nil {
				return nil, err
			}
		} else {
			surface.SetCursorToID(selected[0])
		}
	}
	model := &OperationValuePickerModel{
		surface:  surface,
		request:  request,
		provider: provider,
		title:    title,
		subtitle: subtitle,
		multi:    multi,
	}
	if provider.tagEntry() {
		model.tagAdder = NewTagAdder()
	}
	if provider.dateEntry() {
		model.dateAdder = NewDateAdder()
	}
	return model, nil
}

// dateSectionUnderCursor reports whether the cursor sits on a defer row, and
// refuses when that row is disabled so the inline mode cannot type past a
// refusal the preset rows already carry.
func (model *OperationValuePickerModel) dateSectionUnderCursor() (bool, error) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return false, ErrListItemNotFound
	}
	if !item.Enabled() {
		return false, fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	value, ok := item.Data.(OperationDateValue)
	if !ok {
		return false, ErrListItemNotFound
	}
	return value.Deferred, nil
}

//nolint:gocyclo // Centralized key dispatch preserves picker, freeform, and tag-input precedence.
func (model *OperationValuePickerModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, OperationValueCanceledMsg{}
	}
	if model.provider.tagEntry() && model.tagAdder.AddTagMode {
		handled, cmd := model.tagAdder.handleAddTagInputKey(msg, model.addOrSelectTag)
		if handled {
			return cmd, false, nil
		}
	}
	if model.provider.dateEntry() && model.dateAdder.AddDateMode {
		var chosen *time.Time
		cmd := model.dateAdder.handleAddDateInputKey(msg, func(date time.Time) error {
			chosen = &date
			return nil
		})
		// A typed date carries the same payload the preset rows carry, so the
		// executor, the single handler, and the bulk handler stay untouched.
		if chosen != nil {
			return nil, true, OperationValueSelectedMsg{
				Request: model.request,
				Value:   OperationDateValue{Date: chosen, Deferred: model.dateDeferred},
			}
		}
		return cmd, false, nil
	}
	key := msg.String()
	if model.searching {
		switch key {
		case "up":
			model.surface.MoveCursor(-1)
			return nil, false, nil
		case "down":
			model.surface.MoveCursor(1)
			return nil, false, nil
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
				return nil, false, nil
			}
			model.searching = false
			return nil, false, nil
		case "enter":
			model.searching = false
			return nil, false, nil
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, OperationValueCanceledMsg{}
	case "/":
		model.searching = true
	case "a":
		if model.provider.tagEntry() {
			model.contextEntry = model.contextRowUnderCursor()
			return model.tagAdder.EnterAddMode(), false, nil
		}
		if model.provider.dateEntry() {
			deferred, err := model.dateSectionUnderCursor()
			if err != nil {
				return nil, false, StatusMsg(err.Error())
			}
			model.dateDeferred = deferred
			if deferred {
				model.dateAdder = NewInstantDateAdder()
			} else {
				model.dateAdder = NewDateAdder()
			}
			return model.dateAdder.EnterAddMode(), false, nil
		}
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "home", "g":
		model.surface.MoveCursorToStart()
	case "end", "G":
		model.surface.MoveCursorToEnd()
	case "pgup":
		model.surface.PageCursor(-1)
	case "pgdown":
		model.surface.PageCursor(1)
	case "v":
		// The project tag facet picker hands Space to the vocabulary sheet, so
		// selection moves to the key the project picker already uses for it.
		if model.provider == ValueProviderFilterProjectTags {
			if err := model.toggleCurrentMultiValue(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	case "space":
		if model.provider == ValueProviderFilterProjectTags {
			item, ok := model.surface.CurrentItem()
			if !ok {
				return nil, false, nil
			}
			tag, ok := projectTagVocabularySelectableTag(item)
			if !ok {
				return nil, false, StatusMsg("Untagged is a facet, not a tag")
			}
			return nil, true, ProjectTagVocabularyActionsMsg{Tag: tag}
		}
		if model.multi {
			if err := model.toggleCurrentMultiValue(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	case "enter":
		value, err := model.selectedValue()
		if err != nil {
			return nil, false, StatusMsg(err.Error())
		}
		return nil, true, OperationValueSelectedMsg{
			Request: model.request,
			Value:   value,
		}
	case "w", "s", "t", "c":
		if semanticTaskTagProvider(model.provider) {
			return nil, false, model.jumpToTagCategory(key)
		}
	case "#":
		// The context is single-valued, so its rows are a radio group rather than
		// the checkboxes the tag rows beside them are.
		if model.provider == ValueProviderTags {
			if err := model.selectContextUnderCursor(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	default:
		if model.provider == ValueProviderPriority && len(key) == 1 && key[0] >= '0' && key[0] <= '3' {
			if model.surface.SetCursorToID(ListItemID("priority:" + key)) {
				value, err := model.selectedValue()
				if err == nil {
					return nil, true, OperationValueSelectedMsg{
						Request: model.request, Value: value,
					}
				}
			}
		}
	}
	return nil, false, nil
}
