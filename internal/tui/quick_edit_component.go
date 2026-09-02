package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

const quickEditSuggestionLimit = 5

type quickEditTransition uint8

const (
	quickEditContinue quickEditTransition = iota
	quickEditCommit
	quickEditCancel
)

// QuickEditComponent encapsulates quick edit state and functionality.
type QuickEditComponent struct {
	mode        bool             // True when in quick edit mode
	rename      bool             // True if rename-only mode (no modifier parsing)
	input       textinput.Model  // Text input for editing
	task        *models.Task     // Task being edited
	preview     *quickEditResult // Live preview of parsed changes
	original    *quickEditResult // Original task state for comparison
	previewErr  string
	ac          AutocompleteState
	triggers    []ACTrigger
	project     *models.Project
	projects    []*models.Project
	triggerText string
	removeTag   bool
	pending     bool
}

// NewQuickEditComponent creates a new quick edit component.
func NewQuickEditComponent() *QuickEditComponent {
	quickEditInput := textinput.New()
	quickEditInput.Placeholder = "Edit task..."
	quickEditInput.Prompt = ""
	quickEditInput.CharLimit = 200
	quickEditInput.SetWidth(60)
	applyThemedTextInput(&quickEditInput)

	return &QuickEditComponent{
		mode:     false,
		rename:   false,
		input:    quickEditInput,
		task:     nil,
		preview:  nil,
		original: nil,
		triggers: quickEditTriggers(),
	}
}

func quickEditTriggers() []ACTrigger {
	definitions := textgrammar.Definitions(textgrammar.SurfaceEdit)
	triggers := make([]ACTrigger, 0, len(definitions))
	for _, definition := range definitions {
		mode := autocompleteModeForField(definition.Field)
		if mode == acModeNone || len(definition.Trigger) != 1 {
			continue
		}
		triggers = append(triggers, ACTrigger{Text: definition.Trigger, Mode: mode})
	}
	return triggers
}

// IsActive returns whether quick edit mode is active.
func (q *QuickEditComponent) IsActive() bool {
	return q.mode
}

// IsRenameOnly returns whether quick edit is in rename-only mode.
func (q *QuickEditComponent) IsRenameOnly() bool {
	return q.rename
}

// GetInput returns the text input model.
func (q *QuickEditComponent) GetInput() textinput.Model {
	return q.input
}

// GetTask returns the task being edited.
func (q *QuickEditComponent) GetTask() *models.Task {
	return q.task
}

// GetPreview returns the live preview of parsed changes.
func (q *QuickEditComponent) GetPreview() *quickEditResult {
	return q.preview
}

// GetOriginal returns the original task state.
func (q *QuickEditComponent) GetOriginal() *quickEditResult {
	return q.original
}

// SetPreview updates the preview of parsed changes.
func (q *QuickEditComponent) SetPreview(preview *quickEditResult) {
	q.preview = preview
}

func (q *QuickEditComponent) GetPreviewError() string { return q.previewErr }

func (q *QuickEditComponent) GetAutocomplete() *AutocompleteState { return &q.ac }

func (q *QuickEditComponent) SetInputWidth(width int) {
	width = max(width, 1)
	if q.input.Width() == width {
		return
	}
	q.input.SetWidth(width)
	// bubbles recomputes the horizontal scroll window from SetValue, SetCursor and
	// Update only, so a width change would otherwise keep drawing the old window.
	q.input.SetCursor(q.input.Position())
}

// renderSelectedInput keeps the inline editor visually inside the selected row
// without letting the row-level ANSI normalization erase the input cursor.
// Quick edit deliberately uses a static virtual cursor: the insertion point is
// an essential affordance here, so it must remain visible between keystrokes.
func (q *QuickEditComponent) renderSelectedInput() string {
	styles := ThemedTextInputStyles
	selected := lipgloss.NewStyle().
		Foreground(ColorSelectedFg).
		Background(ColorSelectedBg).
		Bold(true)
	styles.Focused.Text = selected
	styles.Focused.Prompt = selected
	styles.Focused.Placeholder = selected
	styles.Focused.Suggestion = selected
	styles.Cursor.Color = ColorSelectedFg
	styles.Cursor.Blink = false
	q.input.SetStyles(styles)
	return q.input.View()
}

func (q *QuickEditComponent) SetContext(project *models.Project, projects []*models.Project) {
	changed := q.project != project
	q.project = project
	q.projects = flattenProjectSets(project, [][]*models.Project{projects})
	if q.mode && changed {
		q.refreshDraft()
	}
}

// Enter enters quick edit mode for the given task.
func (q *QuickEditComponent) Enter(task *models.Task, renameOnly bool) tea.Cmd {
	return q.EnterWithContext(task, nil, nil, renameOnly)
}

// EnterWithContext starts the independently testable row editor with enough
// workspace context for structural validation and dynamic completion.
func (q *QuickEditComponent) EnterWithContext(task *models.Task, project *models.Project, projects []*models.Project, renameOnly bool) tea.Cmd {
	q.mode = true
	q.rename = renameOnly
	q.task = task
	q.SetContext(project, projects)
	q.input.SetValue(task.Title)
	q.input.Focus()
	q.input.CursorEnd()
	q.ac.Clear()
	q.previewErr = ""
	q.pending = false

	q.original = quickEditSnapshot(task)
	q.preview = quickEditSnapshot(task)
	if project != nil {
		q.refreshDraft()
	}

	return textinput.Blink
}

func quickEditSnapshot(task *models.Task) *quickEditResult {
	if task == nil {
		return nil
	}
	preview := cloneTaskDetachingDependencies(task)
	preview.Tags = append([]string(nil), task.Tags...)
	return &quickEditResult{
		title:    task.Title,
		priority: task.Priority, prioritySet: true,
		dueDate: task.DueDate, dueDateSet: task.DueDate != nil,
		state: task.State(), stateSet: true,
		preview: preview,
	}
}

// Exit exits quick edit mode.
func (q *QuickEditComponent) Exit() {
	q.mode = false
	q.rename = false
	q.task = nil
	q.preview = nil
	q.original = nil
	q.previewErr = ""
	q.pending = false
	q.ac.Clear()
	q.project = nil
	q.projects = nil
	q.triggerText = ""
	q.removeTag = false
	q.input.SetValue("")
	q.input.Blur()
}

// Update updates the quick edit component with a key message.
func (q *QuickEditComponent) Update(msg tea.KeyPressMsg) (textinput.Model, tea.Cmd) {
	return q.input.Update(msg)
}

// Handle is the single state-machine funnel for every row-editor message. It
// owns autocomplete acceptance, strict commit validation, immediate cancel,
// ordinary cursor/text editing, and bracketed-paste normalization.
func (q *QuickEditComponent) Handle(msg tea.Msg) (quickEditTransition, *quickEditResult, tea.Cmd) {
	if !q.mode {
		return quickEditContinue, nil, nil
	}
	if q.pending {
		return quickEditContinue, nil, nil
	}

	switch msg := msg.(type) {
	case tea.PasteMsg:
		msg.Content = normalizeQuickEditPaste(msg.Content)
		var cmd tea.Cmd
		q.input, cmd = q.input.Update(msg)
		q.refreshDraft()
		return quickEditContinue, nil, cmd
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "esc" {
			q.Exit()
			return quickEditCancel, nil, nil
		}
		if q.ac.HandleKey(key, q.acceptSuggestion) {
			q.refreshDraft()
			return quickEditContinue, nil, nil
		}
		if key == "tab" {
			return quickEditContinue, nil, nil
		}
		if key == "enter" {
			result, err := q.commitResult()
			if err != nil {
				q.previewErr = err.Error()
				return quickEditContinue, nil, nil
			}
			q.preview = result
			q.previewErr = ""
			return quickEditCommit, result, nil
		}
		var cmd tea.Cmd
		q.input, cmd = q.input.Update(msg)
		q.refreshDraft()
		return quickEditContinue, nil, cmd
	default:
		return quickEditContinue, nil, nil
	}
}

func normalizeQuickEditPaste(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}

func (q *QuickEditComponent) commitResult() (*quickEditResult, error) {
	if q.rename {
		title := strings.TrimSpace(q.input.Value())
		if err := models.ValidateTaskTitle(title); err != nil {
			return nil, err
		}
		result := quickEditSnapshot(q.task)
		result.title = title
		result.preview.Title = title
		return result, nil
	}
	return parseQuickEdit(q.input.Value(), q.task, q.project, q.projects)
}

func (q *QuickEditComponent) refreshDraft() {
	if q.task == nil {
		q.preview, q.previewErr = nil, "task is required for inline edit"
		q.ac.Clear()
		return
	}
	if q.rename {
		q.preview = quickEditSnapshot(q.task)
		q.preview.title = strings.TrimSpace(q.input.Value())
		q.preview.preview.Title = q.preview.title
		if err := models.ValidateTaskTitle(q.preview.title); err != nil {
			q.previewErr = err.Error()
		} else {
			q.previewErr = ""
		}
		q.ac.Clear()
		return
	}
	q.preview, q.previewErr = parseQuickEditForPreview(q.input.Value(), q.task, q.project, q.projects)
	q.refreshAutocomplete()
}

func (q *QuickEditComponent) SuggestionLineCount(budget int) int {
	if !q.ac.IsActive() {
		return 0
	}
	return min(len(q.ac.Items), max(budget, 1))
}

func (q *QuickEditComponent) RenderSuggestions(indent string, budget int) string {
	return q.ac.RenderDropdownWithLimit(indent, min(max(budget, 1), quickEditSuggestionLimit))
}

func (q *QuickEditComponent) SetError(message string) { q.previewErr = message }

func (q *QuickEditComponent) BeginCommit() { q.pending = true }

func (q *QuickEditComponent) RejectCommit(message string) {
	q.pending = false
	q.previewErr = message
}

// UpdateInput updates the input model directly.
func (q *QuickEditComponent) UpdateInput(input textinput.Model) {
	q.input = input
}

// HasTextInput returns true when quick edit mode is active.
func (q *QuickEditComponent) HasTextInput() bool {
	return q.mode
}

// ShouldCaptureEsc returns true when quick edit needs esc for internal handling.
func (q *QuickEditComponent) ShouldCaptureEsc() bool {
	return q.mode
}
