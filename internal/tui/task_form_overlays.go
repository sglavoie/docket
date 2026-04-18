package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// formOverlayKind tracks which inline overlay is active in TaskFormModel.
type formOverlayKind int

const (
	formOverlayNone          formOverlayKind = iota
	formOverlayType                          // formTypePicker
	formOverlayPriority                      // formPriorityPicker
	formOverlayFlags                         // formFlagsPicker
	formOverlayDue                           // formDueDatePicker
	formOverlayAttr                          // formAttrPicker (generic type-attr picker)
	formOverlayChecklistEdit                 // inline textinput for editing a checklist item
	formOverlaySubtaskEdit                   // inline overlay for editing a subtask draft
)

// ── Inline priority picker ────────────────────────────────────────────────────

// TaskFormPriorityPickedMsg is emitted from the inline priority picker.
type TaskFormPriorityPickedMsg struct {
	Priority int
}

var formPriorityEntries = []struct {
	priority int
	key      string
	label    string
}{
	{1, "1", "critical"},
	{2, "2", "high"},
	{3, "3", "normal"},
	{4, "4", "low"},
	{5, "5", "someday"},
}

type formPriorityPicker struct {
	cursor int
}

func newFormPriorityPicker(current int) *formPriorityPicker {
	cursor := 2 // default: normal
	for i, e := range formPriorityEntries {
		if e.priority == current {
			cursor = i
			break
		}
	}
	return &formPriorityPicker{cursor: cursor}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formPriorityPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, TaskFormPriorityPickedMsg{Priority: formPriorityEntries[p.cursor].priority}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < len(formPriorityEntries)-1 {
			p.cursor++
		}
		return nil, false, nil
	default:
		for _, e := range formPriorityEntries {
			if msg.String() == e.key {
				return nil, true, TaskFormPriorityPickedMsg{Priority: e.priority}
			}
		}
		return nil, false, nil
	}
}

func (p *formPriorityPicker) render(width, current int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Set Priority")
	footerText := HelpStyle.Render("j/k · 1-5: pick · Enter: select · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	var content strings.Builder
	for i, e := range formPriorityEntries {
		icon := GetPriorityStyle(e.priority).Render(PriorityIcon(e.priority))
		isCurrent := e.priority == current
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		line := fmt.Sprintf(" %s%s %s %s", currentMark, icon, e.label, keyHint)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(formPriorityEntries)-1 {
			content.WriteString("\n")
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content.String())
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}
	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)
	return strings.Join(lines, "\n")
}

// ── Inline due-date preset picker ─────────────────────────────────────────────

// TaskFormDueDatePickedMsg is emitted from the inline due-date preset picker.
// Date is nil when the user picks "clear".
type TaskFormDueDatePickedMsg struct {
	Date *time.Time
}

type formDueDatePicker struct {
	cursor int
}

func newFormDueDatePicker(current *time.Time) *formDueDatePicker {
	cursor := 0
	if current == nil {
		// Pre-select "clear"
		for i, e := range dueDatePickerEntries {
			if e.isClear {
				cursor = i
				break
			}
		}
	} else {
		today := util.StartOfDay(time.Now())
		for i, e := range dueDatePickerEntries {
			if e.isClear {
				continue
			}
			if current.Equal(today.AddDate(0, 0, e.daysFromNow)) {
				cursor = i
				break
			}
		}
	}
	return &formDueDatePicker{cursor: cursor}
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg). dismiss=true closes the overlay.
func (p *formDueDatePicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	toMsg := func(e dueDatePickerEntry) tea.Msg {
		if e.isClear {
			return TaskFormDueDatePickedMsg{Date: nil}
		}
		t := util.StartOfDay(time.Now()).AddDate(0, 0, e.daysFromNow)
		return TaskFormDueDatePickedMsg{Date: &t}
	}
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "enter":
		return nil, true, toMsg(dueDatePickerEntries[p.cursor])
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, false, nil
	case "down", "j":
		if p.cursor < len(dueDatePickerEntries)-1 {
			p.cursor++
		}
		return nil, false, nil
	default:
		key := msg.String()
		for _, e := range dueDatePickerEntries {
			if key == e.key {
				return nil, true, toMsg(e)
			}
		}
		return nil, false, nil
	}
}

func (p *formDueDatePicker) render(width int, current *time.Time) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	var titleText string
	if current != nil {
		titleText = fmt.Sprintf("Set Due Date: %s", current.Format("Mon Jan 2, 2006"))
	} else {
		titleText = "Set Due Date"
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	footerText := HelpStyle.Render("j/k · key: pick · Enter: select · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(width*2/5, 36), width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	today := util.StartOfDay(time.Now())
	currentEntryIdx := -1
	if current == nil {
		for i, e := range dueDatePickerEntries {
			if e.isClear {
				currentEntryIdx = i
				break
			}
		}
	} else {
		for i, e := range dueDatePickerEntries {
			if e.isClear {
				continue
			}
			if current.Equal(today.AddDate(0, 0, e.daysFromNow)) {
				currentEntryIdx = i
				break
			}
		}
	}

	var content strings.Builder
	for i, e := range dueDatePickerEntries {
		isCurrent := i == currentEntryIdx
		keyHint := MutedStyle.Render(fmt.Sprintf("[%s]", e.key))
		currentMark := "  "
		if isCurrent {
			currentMark = "● "
		}
		line := fmt.Sprintf(" %s%s %s", currentMark, e.label, keyHint)
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth-2 {
			line += strings.Repeat(" ", innerWidth-2-lineWidth)
		}
		if i == p.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(dueDatePickerEntries)-1 {
			content.WriteString("\n")
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content.String())
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}
	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)
	return strings.Join(lines, "\n")
}

// ── Overlay rendering dispatch ────────────────────────────────────────────────

// renderFormOverlay renders the active inline overlay.
// Returns empty string if no overlay is active.
func (m *TaskFormModel) renderFormOverlay() string {
	switch m.activeOverlay {
	case formOverlayType:
		if m.typePicker != nil {
			return m.typePicker.render(m.width, m.draft.taskType)
		}
	case formOverlayPriority:
		if m.priorityPicker != nil {
			return m.priorityPicker.render(m.width, m.draft.priority)
		}
	case formOverlayFlags:
		if m.flagsPicker != nil {
			return m.flagsPicker.render(m.width)
		}
	case formOverlayDue:
		if m.duePicker != nil {
			return m.duePicker.render(m.width, m.draft.dueDate)
		}
	case formOverlayAttr:
		if m.attrPicker != nil {
			return m.attrPicker.render(m.width)
		}
	case formOverlayChecklistEdit:
		return m.renderChecklistEditOverlay()
	case formOverlaySubtaskEdit:
		return m.renderSubtaskEditOverlay()
	}
	return ""
}

// handleFormOverlayKey routes key events to the active inline overlay.
// Returns consumed=true when the key was handled by the overlay.
func (m *TaskFormModel) handleFormOverlayKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	var (
		cmd     tea.Cmd
		dismiss bool
		result  tea.Msg
	)
	switch m.activeOverlay {
	case formOverlayType:
		if m.typePicker != nil {
			cmd, dismiss, result = m.typePicker.handleKey(msg)
		}
	case formOverlayPriority:
		if m.priorityPicker != nil {
			cmd, dismiss, result = m.priorityPicker.handleKey(msg)
		}
	case formOverlayFlags:
		if m.flagsPicker != nil {
			cmd, dismiss, result = m.flagsPicker.handleKey(msg)
		}
	case formOverlayDue:
		if m.duePicker != nil {
			cmd, dismiss, result = m.duePicker.handleKey(msg)
		}
	case formOverlayAttr:
		if m.attrPicker != nil {
			cmd, dismiss, result = m.attrPicker.handleKey(msg)
		}
	case formOverlayChecklistEdit:
		cmd, dismiss, result = m.handleChecklistEditKey(msg)
	case formOverlaySubtaskEdit:
		cmd, dismiss, result = m.handleSubtaskEditKey(msg)
	default:
		return nil, false
	}

	if dismiss {
		// When cancelling a brand-new item, remove it from the draft.
		if result == nil && m.childrenAddingNew {
			switch m.activeOverlay {
			case formOverlayChecklistEdit:
				if m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.checklist) {
					m.draft.checklist = append(
						m.draft.checklist[:m.childrenEditIdx],
						m.draft.checklist[m.childrenEditIdx+1:]...,
					)
				}
			case formOverlaySubtaskEdit:
				if m.childrenEditIdx >= 0 && m.childrenEditIdx < len(m.draft.subtasks) {
					m.draft.subtasks = append(
						m.draft.subtasks[:m.childrenEditIdx],
						m.draft.subtasks[m.childrenEditIdx+1:]...,
					)
				}
			}
		}
		m.childrenAddingNew = false
		m.closeInlineOverlay()
	}
	if result != nil {
		m.applyFormOverlayResult(result)
	}
	return cmd, true
}

// closeInlineOverlay resets the inline overlay state.
func (m *TaskFormModel) closeInlineOverlay() {
	m.activeOverlay = formOverlayNone
	m.typePicker = nil
	m.priorityPicker = nil
	m.flagsPicker = nil
	m.duePicker = nil
	m.attrPicker = nil
}

// applyTypeChange applies a task type change with all required side effects:
// clears incompatible parent, clamps focusIndex, marks dirty.
// No-op if newType == current type.
func (m *TaskFormModel) applyTypeChange(newType models.TaskType) {
	if newType == m.draft.taskType {
		return
	}
	m.draft.taskType = newType
	if m.draft.parentID != nil {
		if p := formFindTask(m.project.Tasks, *m.draft.parentID); p == nil || !p.CanAddChild(newType) {
			m.draft.parentID = nil
		}
	}
	// Epics cannot have parents — clear parentID unconditionally.
	if newType == models.TaskTypeEpic {
		m.draft.parentID = nil
	}
	maxIdx := formBasicsFieldCount + formAttrFieldCount(newType) - 1
	if m.focusIndex > maxIdx {
		m.focusIndex = maxIdx
	}
	if m.isFieldDisabled(m.focusIndex) {
		m.focusIndex++
	}
	m.hasChanges = true
	m.err = nil
}

// applyFormOverlayResult merges an overlay result into the draft and marks dirty.
func (m *TaskFormModel) applyFormOverlayResult(msg tea.Msg) {
	switch msg := msg.(type) {
	case TaskFormTypePickedMsg:
		m.applyTypeChange(msg.Type)
	case TaskFormPriorityPickedMsg:
		m.draft.priority = msg.Priority
		m.hasChanges = true
		m.err = nil
	case TaskFormFlagsPickedMsg:
		m.draft.pinned = msg.Pinned
		m.draft.active = msg.Active
		m.draft.dimmed = msg.Dimmed
		m.hasChanges = true
		m.err = nil
	case TaskFormDueDatePickedMsg:
		m.draft.dueDate = msg.Date
		m.draft.hasTime = false // Preset picker always sets date-only.
		if msg.Date == nil {
			m.draft.dueInput = ""
		} else {
			m.draft.dueInput = msg.Date.Format("2006-01-02")
		}
		// Sync the textinput so it shows the updated value if still on the Due row.
		m.dueTextInput.SetValue(m.draft.dueInput)
		m.hasChanges = true
		m.err = nil

	case TaskFormAttrPickedMsg:
		m.applyAttrPickedField(msg.Field, msg.Value)
		m.hasChanges = true
		m.err = nil

	case TaskFormChecklistItemEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.checklist) {
			m.draft.checklist[msg.Idx].Text = msg.Text
			m.hasChanges = true
			m.err = nil
		}
		m.childrenAddingNew = false

	case TaskFormSubtaskEditedMsg:
		if msg.Idx >= 0 && msg.Idx < len(m.draft.subtasks) {
			m.draft.subtasks[msg.Idx].title = msg.Title
			m.draft.subtasks[msg.Idx].taskType = msg.TaskType
			m.hasChanges = true
			m.err = nil
		}
		m.childrenAddingNew = false
	}
}

// applyAttrPickedField merges a single attr field result into the draft.
func (m *TaskFormModel) applyAttrPickedField(field, value string) {
	switch field {
	case "bugReproducible":
		m.draft.bugAttrs.Reproducible = models.BugReproducible(value)
	case "bugSeverity":
		m.draft.bugAttrs.Severity = models.BugSeverity(value)
	case "ideaCategory":
		m.draft.ideaAttrs.Category = models.IdeaCategory(value)
	case "ideaEffort":
		m.draft.ideaAttrs.Effort = models.EffortEstimate(value)
	case "ideaValue":
		m.draft.ideaAttrs.Value = models.IdeaValue(value)
	case "ideaValidated":
		m.draft.ideaAttrs.Validated = value == "true"
	case "taskEffort":
		m.draft.taskAttrs.Effort = models.EffortEstimate(value)
	case "taskWorkType":
		m.draft.taskAttrs.WorkType = models.WorkType(value)
	case "storyPoints":
		if value == "" {
			m.draft.storyAttrs.StoryPoints = 0
		} else {
			v, _ := strconv.Atoi(value)
			m.draft.storyAttrs.StoryPoints = v
		}
	case "epicScope":
		m.draft.epicAttrs.Scope = models.EpicScope(value)
	}
}

// formFindTask returns the task with the given ID from a flat list, or nil.
func formFindTask(tasks []*models.Task, id string) *models.Task {
	for _, t := range tasks {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// ── Checklist item edit overlay ───────────────────────────────────────────────

// TaskFormChecklistItemEditedMsg is emitted when a checklist item edit is confirmed.
type TaskFormChecklistItemEditedMsg struct {
	Idx  int
	Text string
}

// handleChecklistEditKey handles key events for the checklist item edit overlay.
func (m *TaskFormModel) handleChecklistEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.checklistItemInput.Value())
		return nil, true, TaskFormChecklistItemEditedMsg{Idx: m.childrenEditIdx, Text: text}
	case "esc":
		return nil, true, nil // cancel
	default:
		var cmd tea.Cmd
		m.checklistItemInput, cmd = m.checklistItemInput.Update(msg)
		return cmd, false, nil
	}
}

// renderChecklistEditOverlay renders the inline checklist item edit overlay.
func (m *TaskFormModel) renderChecklistEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Edit Checklist Item")
	footerText := HelpStyle.Render("Enter: save · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	m.checklistItemInput.SetWidth(innerWidth - 4)
	applyThemedTextInput(&m.checklistItemInput)
	content := " " + m.checklistItemInput.View()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content)
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}
	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)
	return strings.Join(lines, "\n")
}

// ── Subtask edit overlay ──────────────────────────────────────────────────────

// TaskFormSubtaskEditedMsg is emitted when a subtask draft edit is confirmed.
type TaskFormSubtaskEditedMsg struct {
	Idx      int
	Title    string
	TaskType models.TaskType
}

// handleSubtaskEditKey handles key events for the subtask edit overlay.
func (m *TaskFormModel) handleSubtaskEditKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "enter":
		title := strings.TrimSpace(m.subtaskItemInput.Value())
		t := formSubtaskTypes[m.subtaskTypeCursor]
		return nil, true, TaskFormSubtaskEditedMsg{Idx: m.childrenEditIdx, Title: title, TaskType: t}
	case "esc":
		return nil, true, nil // cancel
	case "up", "k", "shift+tab":
		if m.subtaskTypeCursor > 0 {
			m.subtaskTypeCursor--
		}
		return nil, false, nil
	case "down", "j", "tab":
		if m.subtaskTypeCursor < len(formSubtaskTypes)-1 {
			m.subtaskTypeCursor++
		}
		return nil, false, nil
	default:
		var cmd tea.Cmd
		m.subtaskItemInput, cmd = m.subtaskItemInput.Update(msg)
		return cmd, false, nil
	}
}

// renderSubtaskEditOverlay renders the inline subtask edit overlay (title + type).
func (m *TaskFormModel) renderSubtaskEditOverlay() string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Edit Subtask")
	footerText := HelpStyle.Render("j/k: type · Enter: save · Esc: cancel")

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	maxWidth := min(max(m.width*2/3, 40), m.width-4)
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	m.subtaskItemInput.SetWidth(innerWidth - 6)
	applyThemedTextInput(&m.subtaskItemInput)

	var content strings.Builder
	content.WriteString(" " + m.subtaskItemInput.View() + "\n")
	content.WriteString(" " + MutedStyle.Render("Type:") + "  ")
	for i, st := range formSubtaskTypes {
		badge := GetTypeBadge(string(st), st.Icon())
		if i == m.subtaskTypeCursor {
			content.WriteString(SelectorOptionSelectedStyle.Render(" " + badge + " " + string(st) + " "))
		} else {
			content.WriteString(MutedStyle.Render(" " + badge + " " + string(st) + " "))
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(innerWidth)

	rendered := boxStyle.Render(content.String())
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		return rendered
	}
	actualWidth := lipgloss.Width(lines[0])
	lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
	lines[len(lines)-1] = renderTitledBottomBorder(footerText, actualWidth, borderStyle)
	return strings.Join(lines, "\n")
}
