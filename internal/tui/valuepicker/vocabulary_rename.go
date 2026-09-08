package valuepicker

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// Renaming a stored task name is a nested workflow inside the picker that
// already lists it: `e` on a row names it, an explicit confirmation states what
// the change would do, and the outcome is retained on the very list the rename
// rewrote. The picker owns the steps and none of the effects — it holds no
// service, no workspace, and no frozen preview — so every sentence it shows was
// computed by the shell and handed back through the three setters below. That
// split is what lets the browse state underneath survive: a name step and a
// confirmation are stages of this picker rather than overlays above it, so the
// query, the cursor, and the pending multi-select are simply never touched.

// vocabularyRenameNameLimit bounds the typed destination at the same fifty
// characters the inline tag add mode accepts, so a name that can be minted is a
// name that can be renamed onto.
const vocabularyRenameNameLimit = 50

// vocabularyRenameReportLines is how many outcome lines are on screen at once.
// A rename can name every affected lens, and the overlay has to stay inside the
// terminal, so the report is a window with a scroll rather than a list that
// grows until it is cut off.
const vocabularyRenameReportLines = 8

// VocabularyRenameRequestedMsg asks the shell to freeze a plan for one rename.
// The picker cannot compute one: the affected counts come from a workspace
// census and the affected lenses from the saved collection, and neither is the
// picker's to read.
type VocabularyRenameRequestedMsg struct {
	Provider operation.ValueProviderID
	From     string
	To       string
}

// VocabularyRenameConfirmedMsg accepts the plan the shell is holding.
type VocabularyRenameConfirmedMsg struct {
	Provider operation.ValueProviderID
}

// VocabularyRenameAbandonedMsg releases the plan the shell froze for a
// confirmation the user walked back from. A frozen preview carries a whole
// workspace census, so leaving one behind on every abandoned confirmation would
// pin the workspace in memory for the rest of the session.
type VocabularyRenameAbandonedMsg struct{}

// VocabularyRenamePlan is the shell's frozen answer to one rename request: what
// the act is called, and the lines that say what it would change.
type VocabularyRenamePlan struct {
	Heading string
	Lines   []string
}

// VocabularyRenameOutcome is what a committed rename left behind, including the
// saved lenses whose predicates may now select differently.
type VocabularyRenameOutcome struct {
	Heading string
	Lines   []string
}

type vocabularyRenameStage uint8

const (
	vocabularyRenameOff vocabularyRenameStage = iota
	vocabularyRenameNaming
	vocabularyRenameConfirming
	vocabularyRenameReporting
)

// vocabularyRenameState is the whole nested workflow. Its zero value is the
// browse grammar, which is what makes the rename absent rather than disabled on
// every picker that is not a task-name vocabulary.
type vocabularyRenameState struct {
	stage   vocabularyRenameStage
	from    string
	input   textinput.Model
	failure string
	plan    VocabularyRenamePlan
	outcome VocabularyRenameOutcome
	offset  int
}

// vocabularyRenameProvider reports whether a picker lists stored task names a
// rename can act on. The two filter facets are the whole set: they are the only
// surfaces that enumerate every spelling in use across the scope, which is what
// makes a rename discoverable from the row it would rewrite.
func vocabularyRenameProvider(provider operation.ValueProviderID) bool {
	return provider == operation.ValueProviderFilterTags ||
		provider == operation.ValueProviderFilterContexts
}

// vocabularyRenameSourceUnderCursor reads the stored name the cursor row would
// rename. The context picker's leading row is a facet over the tasks the
// vocabulary does not cover rather than a name anything stores, so it is
// refused here by the same reasoning the project tag sheet refuses Untagged.
func (model *OperationValuePickerModel) vocabularyRenameSourceUnderCursor() (string, error) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return "", listsurface.ErrListItemNotFound
	}
	name, ok := item.Data.(string)
	if !ok || name == "" {
		return "", fmt.Errorf("this row is not a stored name")
	}
	if name == query.NoContextFilterKey {
		return "", fmt.Errorf("No context is a facet, not a context")
	}
	return name, nil
}

// beginVocabularyRename opens the name step prefilled with the highlighted
// name, so the common edit is a suffix and the common merge is a retype.
func (model *OperationValuePickerModel) beginVocabularyRename() (tea.Cmd, tea.Msg) {
	name, err := model.vocabularyRenameSourceUnderCursor()
	if err != nil {
		return nil, status.StatusMsg(err.Error())
	}
	input := textinput.New()
	input.CharLimit = vocabularyRenameNameLimit
	theme.ApplyThemedTextInput(&input)
	input.SetValue(name)
	input.CursorEnd()
	input.Focus()
	model.rename = vocabularyRenameState{stage: vocabularyRenameNaming, from: name, input: input}
	return textinput.Blink, nil
}

// handleVocabularyRenameKey is the nested grammar. It runs after the inline add
// modes and before the browse switch, so an open name step takes every
// printable key as literal input exactly as a search or an add mode does.
func (model *OperationValuePickerModel) handleVocabularyRenameKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch model.rename.stage {
	case vocabularyRenameNaming:
		return model.handleVocabularyRenameNamingKey(msg)
	case vocabularyRenameConfirming:
		return model.handleVocabularyRenameConfirmKey(msg)
	case vocabularyRenameReporting:
		return model.handleVocabularyRenameReportKey(msg)
	case vocabularyRenameOff:
	}
	return nil, false, nil
}

func (model *OperationValuePickerModel) handleVocabularyRenameNamingKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		model.rename = vocabularyRenameState{}
		return nil, false, nil
	case "enter":
		destination := strings.TrimSpace(model.rename.input.Value())
		if destination == "" {
			model.rename.failure = "Enter a destination name"
			return nil, false, nil
		}
		model.rename.failure = ""
		return nil, false, VocabularyRenameRequestedMsg{
			Provider: model.provider,
			From:     model.rename.from,
			To:       destination,
		}
	default:
		var cmd tea.Cmd
		model.rename.input, cmd = model.rename.input.Update(msg)
		return cmd, false, nil
	}
}

func (model *OperationValuePickerModel) handleVocabularyRenameConfirmKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "y", "enter":
		return nil, false, VocabularyRenameConfirmedMsg{Provider: model.provider}
	case "n", "esc":
		// Back to the name rather than out of the workflow: a confirmation read
		// and declined is nearly always a destination the reader wants to change.
		model.rename.stage = vocabularyRenameNaming
		model.rename.plan = VocabularyRenamePlan{}
		model.rename.failure = ""
		return nil, false, VocabularyRenameAbandonedMsg{}
	}
	return nil, false, nil
}

func (model *OperationValuePickerModel) handleVocabularyRenameReportKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc", "enter":
		model.rename = vocabularyRenameState{}
	case "up", "k":
		if model.rename.offset > 0 {
			model.rename.offset--
		}
	case "down", "j":
		if model.rename.offset < model.vocabularyRenameMaxOffset() {
			model.rename.offset++
		}
	}
	return nil, false, nil
}

func (model *OperationValuePickerModel) vocabularyRenameMaxOffset() int {
	return max(len(model.rename.outcome.Lines)-vocabularyRenameReportLines, 0)
}

// ShowVocabularyRenamePlan mounts the shell's frozen plan as the confirmation
// step. It is ignored unless the name step is still open, so a plan answering a
// request the reader has already walked away from cannot reopen the workflow.
func (model *OperationValuePickerModel) ShowVocabularyRenamePlan(plan VocabularyRenamePlan) {
	if model == nil || model.rename.stage != vocabularyRenameNaming {
		return
	}
	model.rename.stage = vocabularyRenameConfirming
	model.rename.plan = plan
	model.rename.failure = ""
}

// RefuseVocabularyRename retains a refusal on the name step with the typed
// value intact, which is what makes a rejected name — reserved, unchanged, or
// unspellable — one correction away from a second attempt.
func (model *OperationValuePickerModel) RefuseVocabularyRename(message string) {
	if model == nil || model.rename.stage == vocabularyRenameOff {
		return
	}
	model.rename.stage = vocabularyRenameNaming
	model.rename.plan = VocabularyRenamePlan{}
	model.rename.failure = message
}

// AdoptVocabularyRenameOutcome retains a committed rename's details on the
// picker rebuilt from the renamed workspace. The report survives on the new
// list because the rename is what produced it: dismissing it returns to a
// browse grammar over the destination row, not to the vocabulary that is gone.
func (model *OperationValuePickerModel) AdoptVocabularyRenameOutcome(outcome VocabularyRenameOutcome) {
	if model == nil {
		return
	}
	model.rename = vocabularyRenameState{stage: vocabularyRenameReporting, outcome: outcome}
}

// VocabularyRenameSource is the stored name the open workflow started from.
func (model *OperationValuePickerModel) VocabularyRenameSource() string {
	if model == nil {
		return ""
	}
	return model.rename.from
}

// VocabularyRenameDestination is the name typed into the open workflow.
func (model *OperationValuePickerModel) VocabularyRenameDestination() string {
	if model == nil {
		return ""
	}
	return strings.TrimSpace(model.rename.input.Value())
}

// VocabularyRenameConfirming reports whether a frozen plan is on screen waiting
// to be accepted, which is the one state the shell may apply a preview from.
func (model *OperationValuePickerModel) VocabularyRenameConfirming() bool {
	return model != nil && model.rename.stage == vocabularyRenameConfirming
}

// VocabularyRenameReporting reports whether a committed rename's details are
// still retained on the picker.
func (model *OperationValuePickerModel) VocabularyRenameReporting() bool {
	return model != nil && model.rename.stage == vocabularyRenameReporting
}

// VocabularyRenameFailure is the refusal retained on the name step, if any.
func (model *OperationValuePickerModel) VocabularyRenameFailure() string {
	if model == nil {
		return ""
	}
	return model.rename.failure
}

// vocabularyRenameNoun is what this picker's rows are called, which is the
// difference between "Rename Tag" and "Rename Context" and is read by the step
// titles and the guidance alike.
func (model *OperationValuePickerModel) vocabularyRenameNoun() string {
	if model.provider == operation.ValueProviderFilterContexts {
		return "context"
	}
	return "tag"
}
