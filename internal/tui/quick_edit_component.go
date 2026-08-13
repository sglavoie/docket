package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
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
		triggers = append(triggers, ACTrigger{Char: definition.Trigger[0], Mode: mode})
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
	q.input.SetWidth(max(width, 1))
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
	preview := *task
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

func (q *QuickEditComponent) cursorByteOffset() int {
	runes := []rune(q.input.Value())
	position := max(0, min(q.input.Position(), len(runes)))
	return len(string(runes[:position]))
}

func (q *QuickEditComponent) refreshAutocomplete() {
	value := q.input.Value()
	cursorByte := q.cursorByteOffset()
	mode, pos, partial, triggerText, removeTag := detectQuickEditTrigger(value[:cursorByte], q.triggers)
	if mode == acModeNone {
		q.ac.Clear()
		q.triggerText = ""
		q.removeTag = false
		return
	}
	q.triggerText = triggerText
	q.removeTag = removeTag
	parsedPrefix := textgrammar.Parse(value[:pos], textgrammar.SurfaceEdit)
	if field, singleton := autocompleteSingletonField(mode); singleton && len(parsedPrefix.TokensFor(field)) > 0 {
		q.ac.Clear()
		return
	}
	q.ac.UpdateSuggestions(mode, pos, q.suggestions(mode, pos, partial, parsedPrefix))
}

func (q *QuickEditComponent) suggestions(mode, triggerPos int, partial string, parsedPrefix textgrammar.Result) []ACSuggestion {
	switch mode {
	case acModeTag:
		project := q.effectiveProject()
		field := textgrammar.FieldTag
		values := CollectTagNamesForProject(project)
		prefix := "#"
		if q.removeTag {
			field = textgrammar.FieldRemoveTag
			values = append([]string(nil), q.task.Tags...)
			prefix = "-#"
		}
		used := parsedPrefix.TokensFor(field)
		return filterUsedTagSuggestions(quickEditTagSuggestions(values, partial, prefix), used)
	case acModeParent:
		parents, _ := quickEditParentCandidates(q.input.Value(), q.task, q.project, q.projects)
		items := quickEditClearSuggestions(textgrammar.FieldParent, partial)
		return append(items, quickEditStructuralSuggestions(parents, partial, textgrammar.FieldParent)...)
	case acModeChild:
		// The token under the cursor is removed before the draft is read, so an
		// in-progress alias cannot withhold the very task it is naming. Child
		// declares no clear operation, so no <0 row is offered.
		draft := quickEditDraftWithoutToken(q.input.Value(), triggerPos)
		children, _ := quickEditChildCandidates(draft, q.task, q.project, q.projects)
		return quickEditStructuralSuggestions(children, partial, textgrammar.FieldChild)
	case acModeProject:
		return quickEditProjectSuggestions(q.projects, partial)
	case acModePriority, acModeDate, acModeDestination:
		return quickEditStaticSuggestions(mode, partial)
	default:
		return nil
	}
}

func (q *QuickEditComponent) effectiveProject() *models.Project {
	parsed := textgrammar.Parse(q.input.Value(), textgrammar.SurfaceEdit)
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		if project := resolveProject(tokens[0].Value, q.projects); project != nil {
			return project
		}
	}
	return q.project
}

func (q *QuickEditComponent) acceptSuggestion() {
	if !q.ac.IsActive() {
		return
	}
	cursorByte := q.cursorByteOffset()
	triggerText := q.triggerText
	if triggerText == "" {
		triggerText = string(TriggerCharForMode(q.ac.Mode, q.triggers))
	}
	selected := q.ac.Selected()
	var value string
	if len(triggerText) == 1 {
		value = q.ac.BuildAcceptedInputAt(q.input.Value(), cursorByte, triggerText[0])
	} else {
		value = q.input.Value()[:q.ac.TriggerPos] + triggerText + selected.Value + " " + q.input.Value()[cursorByte:]
	}
	insertedPrefix := q.input.Value()[:q.ac.TriggerPos] + triggerText + selected.Value + " "
	q.input.SetValue(value)
	q.input.SetCursor(utf8.RuneCountInString(insertedPrefix))
	q.ac.Clear()
	q.triggerText = ""
	q.removeTag = false
}

func detectQuickEditTrigger(input string, triggers []ACTrigger) (mode, pos int, partial, triggerText string, removeTag bool) {
	tokenStart := strings.LastIndexAny(input, " \n\t") + 1
	if tokenStart < len(input) && strings.HasPrefix(input[tokenStart:], "-#") {
		return acModeTag, tokenStart, input[tokenStart+2:], "-#", true
	}
	mode, pos, partial = DetectTrigger(input, triggers)
	if mode == acModeNone {
		return mode, pos, partial, "", false
	}
	return mode, pos, partial, string(input[pos]), false
}

func quickEditTagSuggestions(values []string, partial, prefix string) []ACSuggestion {
	return scoredQuickEditSuggestions(values, partial, func(value string) ACSuggestion {
		return ACSuggestion{Display: prefix + value, Value: value}
	})
}

// quickEditStructuralSuggestions ranks candidates for a >parent or <child
// reference. Both families score one search text, so the two relationships can
// never rank the same task differently; the field decides only which sigil the
// row is rendered with, read from the registry rather than spelled here.
func quickEditStructuralSuggestions(tasks []*models.Task, partial string, field textgrammar.Field) []ACSuggestion {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	type candidate struct {
		task  *models.Task
		score int
	}
	items := make([]candidate, 0, len(tasks))
	for _, task := range tasks {
		search := fmt.Sprintf("%d %s", task.ShortID, task.Title)
		score := fuzzy.Score(search, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{task, score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, ACSuggestion{
			Display: fmt.Sprintf("%s%d %s", definition.Trigger, item.task.ShortID, item.task.Title),
			Value:   strconv.Itoa(item.task.ShortID),
		})
	}
	return result
}

// quickEditDraftWithoutToken removes the token the cursor is completing so the
// rest of the line reads as settled facts. A repeatable field needs every other
// token of its own family visible, and needs its own in-progress token hidden.
func quickEditDraftWithoutToken(value string, triggerPos int) string {
	if triggerPos < 0 || triggerPos >= len(value) {
		return value
	}
	end := strings.IndexAny(value[triggerPos:], " \t\n")
	if end < 0 {
		return value[:triggerPos]
	}
	return value[:triggerPos] + value[triggerPos+end:]
}

func quickEditProjectSuggestions(projects []*models.Project, partial string) []ACSuggestion {
	type candidate struct {
		project *models.Project
		score   int
	}
	items := make([]candidate, 0, len(projects))
	for _, project := range projects {
		// Callers provide the Outliner's already-admitted project population,
		// which includes parked destinations but excludes archived projects.
		if project == nil {
			continue
		}
		score := fuzzy.Score(project.Slug+" "+project.Name, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{project, score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, ACSuggestion{Display: "%" + item.project.Slug + " " + item.project.Name, Value: item.project.Slug})
	}
	return result
}

func quickEditStaticSuggestions(mode int, partial string) []ACSuggestion {
	field, ok := autocompleteSingletonField(mode)
	if !ok {
		return nil
	}
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	result := make([]ACSuggestion, 0, len(definition.Values))
	partial = strings.ToLower(partial)
	result = append(result, quickEditClearSuggestions(field, partial)...)
	for _, value := range definition.Values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		result = append(result, ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	return result
}

func quickEditClearSuggestions(field textgrammar.Field, partial string) []ACSuggestion {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	partial = strings.ToLower(partial)
	result := make([]ACSuggestion, 0, len(definition.Clears))
	for _, clear := range definition.Clears {
		if clear.Surfaces&textgrammar.SurfaceEdit == 0 || (partial != "" && !strings.Contains(strings.ToLower(clear.Value), partial)) {
			continue
		}
		display := definition.Trigger + clear.Value
		if clear.Description != "" {
			display += " (" + clear.Description + ")"
		}
		result = append(result, ACSuggestion{Display: display, Value: clear.Value})
	}
	return result
}

func scoredQuickEditSuggestions(values []string, partial string, render func(string) ACSuggestion) []ACSuggestion {
	type candidate struct {
		value string
		score int
	}
	items := make([]candidate, 0, len(values))
	for _, value := range values {
		score := fuzzy.Score(value, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{value, score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].value < items[j].value
		}
		return items[i].score > items[j].score
	})
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, render(item.value))
	}
	return result
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
