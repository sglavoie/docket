package grammar

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const QuickEditSuggestionLimit = 5

type QuickEditTransition uint8

const (
	QuickEditContinue QuickEditTransition = iota
	QuickEditCommit
	QuickEditCancel
)

// QuickEditComponent encapsulates quick edit state and functionality.
type QuickEditComponent struct {
	mode        bool             // True when in quick edit mode
	rename      bool             // True if rename-only mode (no modifier parsing)
	Input       textinput.Model  // Text input for editing
	task        *models.Task     // Task being edited
	preview     *QuickEditResult // Live preview of parsed changes
	original    *QuickEditResult // Original task state for comparison
	previewErr  string
	AC          AutocompleteState
	Triggers    []ACTrigger
	project     *models.Project
	projects    []*models.Project
	triggerText string
	RemoveTag   bool
	Pending     bool
}

// NewQuickEditComponent creates a new quick edit component.
func NewQuickEditComponent() *QuickEditComponent {
	quickEditInput := textinput.New()
	quickEditInput.Placeholder = "Edit task..."
	quickEditInput.Prompt = ""
	quickEditInput.CharLimit = 200
	quickEditInput.SetWidth(60)
	theme.ApplyThemedTextInput(&quickEditInput)

	return &QuickEditComponent{
		mode:     false,
		rename:   false,
		Input:    quickEditInput,
		task:     nil,
		preview:  nil,
		original: nil,
		Triggers: quickEditTriggers(),
	}
}

func quickEditTriggers() []ACTrigger {
	definitions := textgrammar.Definitions(textgrammar.SurfaceEdit)
	triggers := make([]ACTrigger, 0, len(definitions))
	for _, definition := range definitions {
		mode := AutocompleteModeForField(definition.Field)
		if mode == ACModeNone || len(definition.Trigger) != 1 {
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
	return q.Input
}

// GetTask returns the task being edited.
func (q *QuickEditComponent) GetTask() *models.Task {
	return q.task
}

// GetProject returns the project the edit is resolved against. It is the
// project SetContext last supplied, so the host reads back exactly the scope it
// handed in rather than re-deriving it from a view that may have moved.
func (q *QuickEditComponent) GetProject() *models.Project {
	return q.project
}

// GetPreview returns the live preview of parsed changes.
func (q *QuickEditComponent) GetPreview() *QuickEditResult {
	return q.preview
}

// GetOriginal returns the original task state.
func (q *QuickEditComponent) GetOriginal() *QuickEditResult {
	return q.original
}

// SetPreview updates the preview of parsed changes.
func (q *QuickEditComponent) SetPreview(preview *QuickEditResult) {
	q.preview = preview
}

func (q *QuickEditComponent) GetPreviewError() string { return q.previewErr }

func (q *QuickEditComponent) GetAutocomplete() *AutocompleteState { return &q.AC }

func (q *QuickEditComponent) SetInputWidth(width int) {
	width = max(width, 1)
	if q.Input.Width() == width {
		return
	}
	q.Input.SetWidth(width)
	// bubbles recomputes the horizontal scroll window from SetValue, SetCursor and
	// Update only, so a width change would otherwise keep drawing the old window.
	q.Input.SetCursor(q.Input.Position())
}

// RenderSelectedInput keeps the inline editor visually inside the selected row
// without letting the row-level ANSI normalization erase the input cursor.
// Quick edit deliberately uses a static virtual cursor: the insertion point is
// an essential affordance here, so it must remain visible between keystrokes.
func (q *QuickEditComponent) RenderSelectedInput() string {
	styles := theme.ThemedTextInputStyles
	selected := lipgloss.NewStyle().
		Foreground(theme.ColorSelectedFg).
		Background(theme.ColorSelectedBg).
		Bold(true)
	styles.Focused.Text = selected
	styles.Focused.Prompt = selected
	styles.Focused.Placeholder = selected
	styles.Focused.Suggestion = selected
	styles.Cursor.Color = theme.ColorSelectedFg
	styles.Cursor.Blink = false
	q.Input.SetStyles(styles)
	return q.Input.View()
}

func (q *QuickEditComponent) SetContext(project *models.Project, projects []*models.Project) {
	changed := q.project != project
	q.project = project
	q.projects = flattenProjectSets(project, [][]*models.Project{projects})
	if q.mode && changed {
		q.RefreshDraft()
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
	q.Input.SetValue(task.Title)
	q.Input.Focus()
	q.Input.CursorEnd()
	q.AC.Clear()
	q.previewErr = ""
	q.Pending = false

	q.original = QuickEditSnapshot(task)
	q.preview = QuickEditSnapshot(task)
	if project != nil {
		q.RefreshDraft()
	}

	return textinput.Blink
}

func QuickEditSnapshot(task *models.Task) *QuickEditResult {
	if task == nil {
		return nil
	}
	preview := taskref.CloneTaskDetachingDependencies(task)
	preview.Tags = append([]string(nil), task.Tags...)
	return &QuickEditResult{
		Title:    task.Title,
		Priority: task.Priority, PrioritySet: true,
		DueDate: task.DueDate, DueDateSet: task.DueDate != nil,
		State: task.State(), StateSet: true,
		Preview: preview,
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
	q.Pending = false
	q.AC.Clear()
	q.project = nil
	q.projects = nil
	q.triggerText = ""
	q.RemoveTag = false
	q.Input.SetValue("")
	q.Input.Blur()
}

// Update updates the quick edit component with a key message.
func (q *QuickEditComponent) Update(msg tea.KeyPressMsg) (textinput.Model, tea.Cmd) {
	return q.Input.Update(msg)
}

// Handle is the single state-machine funnel for every row-editor message. It
// owns autocomplete acceptance, strict commit validation, immediate cancel,
// ordinary cursor/text editing, and bracketed-paste normalization.
func (q *QuickEditComponent) Handle(msg tea.Msg) (QuickEditTransition, *QuickEditResult, tea.Cmd) {
	if !q.mode {
		return QuickEditContinue, nil, nil
	}
	if q.Pending {
		return QuickEditContinue, nil, nil
	}

	switch msg := msg.(type) {
	case tea.PasteMsg:
		msg.Content = normalizeQuickEditPaste(msg.Content)
		var cmd tea.Cmd
		q.Input, cmd = q.Input.Update(msg)
		q.RefreshDraft()
		return QuickEditContinue, nil, cmd
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "esc" {
			q.Exit()
			return QuickEditCancel, nil, nil
		}
		if q.AC.HandleKey(key, q.AcceptSuggestion) {
			q.RefreshDraft()
			return QuickEditContinue, nil, nil
		}
		if key == "tab" {
			return QuickEditContinue, nil, nil
		}
		if key == "enter" {
			result, err := q.commitResult()
			if err != nil {
				q.previewErr = err.Error()
				return QuickEditContinue, nil, nil
			}
			q.preview = result
			q.previewErr = ""
			return QuickEditCommit, result, nil
		}
		var cmd tea.Cmd
		q.Input, cmd = q.Input.Update(msg)
		q.RefreshDraft()
		return QuickEditContinue, nil, cmd
	default:
		return QuickEditContinue, nil, nil
	}
}

func normalizeQuickEditPaste(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}

func (q *QuickEditComponent) commitResult() (*QuickEditResult, error) {
	if q.rename {
		title := strings.TrimSpace(q.Input.Value())
		if err := models.ValidateTaskTitle(title); err != nil {
			return nil, err
		}
		result := QuickEditSnapshot(q.task)
		result.Title = title
		result.Preview.Title = title
		return result, nil
	}
	return ParseQuickEdit(q.Input.Value(), q.task, q.project, q.projects)
}

func (q *QuickEditComponent) RefreshDraft() {
	if q.task == nil {
		q.preview, q.previewErr = nil, "task is required for inline edit"
		q.AC.Clear()
		return
	}
	if q.rename {
		q.preview = QuickEditSnapshot(q.task)
		q.preview.Title = strings.TrimSpace(q.Input.Value())
		q.preview.Preview.Title = q.preview.Title
		if err := models.ValidateTaskTitle(q.preview.Title); err != nil {
			q.previewErr = err.Error()
		} else {
			q.previewErr = ""
		}
		q.AC.Clear()
		return
	}
	q.preview, q.previewErr = parseQuickEditForPreview(q.Input.Value(), q.task, q.project, q.projects)
	q.RefreshAutocomplete()
}

func (q *QuickEditComponent) SuggestionLineCount(budget int) int {
	if !q.AC.IsActive() {
		return 0
	}
	return min(len(q.AC.Items), max(budget, 1))
}

func (q *QuickEditComponent) RenderSuggestions(indent string, budget int) string {
	return q.AC.RenderDropdownWithLimit(indent, min(max(budget, 1), QuickEditSuggestionLimit))
}

func (q *QuickEditComponent) SetError(message string) { q.previewErr = message }

func (q *QuickEditComponent) BeginCommit() { q.Pending = true }

func (q *QuickEditComponent) RejectCommit(message string) {
	q.Pending = false
	q.previewErr = message
}

// UpdateInput updates the input model directly.
func (q *QuickEditComponent) UpdateInput(input textinput.Model) {
	q.Input = input
}

// HasTextInput returns true when quick edit mode is active.
func (q *QuickEditComponent) HasTextInput() bool {
	return q.mode
}

// ShouldCaptureEsc returns true when quick edit needs esc for internal handling.
func (q *QuickEditComponent) ShouldCaptureEsc() bool {
	return q.mode
}
