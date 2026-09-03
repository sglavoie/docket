package tui

import (
	"fmt"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/status"
)

type operationValueMembership uint8

const (
	operationValueNone operationValueMembership = iota
	operationValueSome
	operationValueAll
)

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
				return nil, false, status.StatusMsg(err.Error())
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
				return nil, false, status.StatusMsg(err.Error())
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
				return nil, false, status.StatusMsg("Untagged is a facet, not a tag")
			}
			return nil, true, ProjectTagVocabularyActionsMsg{Tag: tag}
		}
		if model.multi {
			if err := model.toggleCurrentMultiValue(); err != nil {
				return nil, false, status.StatusMsg(err.Error())
			}
		}
	case "enter":
		value, err := model.selectedValue()
		if err != nil {
			return nil, false, status.StatusMsg(err.Error())
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
				return nil, false, status.StatusMsg(err.Error())
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
