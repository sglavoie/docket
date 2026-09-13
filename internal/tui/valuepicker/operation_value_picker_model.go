package valuepicker

import (
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/status"
)

type OperationValueMembership uint8

const (
	OperationValueNone OperationValueMembership = iota
	OperationValueSome
	OperationValueAll
)

// OperationValuePickerModel is the shared ListSurface value-picker control.
type OperationValuePickerModel struct {
	surface   *listsurface.ListSurfaceModel
	request   operation.OperationExecutionRequest
	provider  operation.ValueProviderID
	title     string
	subtitle  string
	multi     bool
	searching bool
	// membershipInitial and membershipCurrent are the tri-state editor's two
	// halves: what the selection agreed on when the picker opened, and what it
	// says now. Both the tag picker and the reference picker read them, because
	// both edit a per-task set over a selection and both must emit a difference
	// rather than a replacement.
	membershipInitial map[listsurface.ListItemID]OperationValueMembership
	membershipCurrent map[listsurface.ListItemID]OperationValueMembership
	tagAdder          listsurface.TagAdder
	dateAdder         chrome.DateAdder
	// dateDeferred records which section the inline date mode was opened from,
	// so the typed value writes the same field the row under the cursor names.
	dateDeferred bool
	// contextEntry records that the inline add mode was opened from a context
	// row, so the typed name mints a context instead of a tag.
	contextEntry bool
	// ContextInitial is the context row every target already agreed on. An empty
	// ID means the targets disagreed, so any chosen row is a change.
	ContextInitial listsurface.ListItemID
	// rename is the nested rename/merge workflow the two filter vocabularies
	// offer. Its zero value is the browse grammar. See vocabulary_rename.go.
	rename vocabularyRenameState
}

func NewOperationValuePickerModel(
	request operation.OperationExecutionRequest,
	provider operation.ValueProviderID,
	title string,
	subtitle string,
	source listsurface.ListSource,
	multi bool,
	selected []listsurface.ListItemID,
) (*OperationValuePickerModel, error) {
	mode := listsurface.ListSelectionSingle
	if multi {
		mode = listsurface.ListSelectionMulti
	}
	surface, err := source.NewModel(listsurface.ListSurfaceConfig{
		SelectionMode: mode,
		EmptyState: listsurface.ListEmptyState{
			Title: "No values available.",
		},
		NoMatchState: listsurface.ListEmptyState{
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
	if provider.TagEntry() {
		model.tagAdder = listsurface.NewTagAdder()
	}
	if provider.DateEntry() {
		model.dateAdder = chrome.NewDateAdder()
	}
	return model, nil
}

//nolint:gocyclo // Centralized key dispatch preserves picker, freeform, and tag-input precedence.
func (model *OperationValuePickerModel) HandleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, operation.OperationValueCanceledMsg{}
	}
	if model.provider.TagEntry() && model.tagAdder.AddTagMode {
		handled, cmd := model.tagAdder.HandleAddTagInputKey(msg, model.AddOrSelectTag)
		if handled {
			return cmd, false, nil
		}
	}
	if model.provider.DateEntry() && model.dateAdder.AddDateMode {
		var chosen *time.Time
		cmd := model.dateAdder.HandleAddDateInputKey(msg, func(date time.Time) error {
			chosen = &date
			return nil
		})
		// A typed date carries the same payload the preset rows carry, so the
		// executor, the single handler, and the bulk handler stay untouched.
		if chosen != nil {
			return nil, true, operation.OperationValueSelectedMsg{
				Request: model.request,
				Value:   operation.OperationDateValue{Date: chosen, Deferred: model.dateDeferred},
			}
		}
		return cmd, false, nil
	}
	if model.rename.stage != vocabularyRenameOff {
		// The nested rename runs above the browse grammar for the same reason the
		// add modes do: its name step is text entry, so every key it does not name
		// itself is a character rather than a command.
		return model.handleVocabularyRenameKey(msg)
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

	if model.provider.DateEntry() {
		// The preset keys are resolved ahead of the browse grammar rather than in
		// its default arm, because two of them are letters that arm already spells
		// for another picker: `t` and `w` are tag-category jumps below, and falling
		// through to them would leave `@t` and `@w` doing nothing here. Search
		// editing and the typed-date input have already claimed their keys above, so
		// a preset key can only ever reach this while the list is being browsed.
		if result, dismiss, handled := model.handleDatePresetShortcut(key); handled {
			return nil, dismiss, result
		}
	}

	switch key {
	case "esc":
		return nil, true, operation.OperationValueCanceledMsg{}
	case "/":
		model.searching = true
	case "a":
		if model.provider.TagEntry() {
			model.contextEntry = model.contextRowUnderCursor()
			return model.tagAdder.EnterAddMode(), false, nil
		}
		if model.provider.DateEntry() {
			section, err := model.typedDateSectionUnderCursor()
			if err != nil {
				return nil, false, status.StatusMsg(err.Error())
			}
			if section == datePickerSectionCheckIn {
				return nil, false, nil
			}
			deferred := section == datePickerSectionDefer
			model.dateDeferred = deferred
			if deferred {
				model.dateAdder = chrome.NewInstantDateAdder()
			} else {
				model.dateAdder = chrome.NewDateAdder()
			}
			return model.dateAdder.EnterAddMode(), false, nil
		}
	case "e":
		// `e` means edit or rename everywhere else, and a vocabulary row is the one
		// thing on a picker that has a name of its own to rename.
		if vocabularyRenameProvider(model.provider) {
			cmd, result := model.beginVocabularyRename()
			return cmd, false, result
		}
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "tab", "shift+tab":
		// The key is consumed either way: a picker this does not apply to must not
		// leak Tab back to the surface underneath, where it would change the
		// Outliner's mode out from under the overlay.
		if model.sectionedDatePicker() {
			direction := 1
			if key == "shift+tab" {
				direction = -1
			}
			return nil, false, model.switchDateSection(direction)
		}
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
		if model.provider == operation.ValueProviderFilterProjectTags {
			if err := model.ToggleCurrentMultiValue(); err != nil {
				return nil, false, status.StatusMsg(err.Error())
			}
		}
	case "space":
		if model.provider == operation.ValueProviderFilterProjectTags {
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
			if err := model.ToggleCurrentMultiValue(); err != nil {
				return nil, false, status.StatusMsg(err.Error())
			}
		}
	case "enter":
		value, err := model.SelectedValue()
		if err != nil {
			return nil, false, status.StatusMsg(err.Error())
		}
		if _, openCheckIn := value.(operation.OperationCheckInEditorAction); openCheckIn {
			return nil, true, operation.OperationValueRequestMsg{
				Request: operation.OperationExecutionRequest{
					ID:      operation.OperationTaskSetCheckIn,
					Context: operation.CloneCommandContext(model.request.Context),
				},
				Provider: operation.ValueProviderCheckIn,
			}
		}
		return nil, true, operation.OperationValueSelectedMsg{Request: model.request, Value: value}
	case "w", "s", "t", "c":
		if semanticTaskTagProvider(model.provider) {
			return nil, false, model.jumpToTagCategory(key)
		}
	case "#":
		// The context is single-valued, so its rows are a radio group rather than
		// the checkboxes the tag rows beside them are.
		if model.provider == operation.ValueProviderTags {
			if err := model.SelectContextUnderCursor(); err != nil {
				return nil, false, status.StatusMsg(err.Error())
			}
		}
	default:
		if model.provider == operation.ValueProviderPriority && len(key) == 1 && key[0] >= '0' && key[0] <= '3' {
			if model.surface.SetCursorToID(listsurface.ListItemID("priority:" + key)) {
				value, err := model.SelectedValue()
				if err == nil {
					return nil, true, operation.OperationValueSelectedMsg{
						Request: model.request, Value: value,
					}
				}
			}
		}
	}
	return nil, false, nil
}
