package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// noteTaskItem wraps a task for use with BasePicker.
type noteTaskItem struct {
	task *models.Task
}

func (i noteTaskItem) FilterText() string { return i.task.Title }
func (i noteTaskItem) DisplayText() string {
	return fmt.Sprintf("#%d %s", i.task.ShortID, i.task.Title)
}
func (i noteTaskItem) CountValue() int { return 0 }
func (i noteTaskItem) ItemID() string  { return i.task.ID }

// NoteTaskLinkerModel is a task picker for linking notes to tasks.
type NoteTaskLinkerModel struct {
	picker        *BasePicker[noteTaskItem]
	project       *models.Project
	note          *models.Note
	changed       bool
	lastToggled   string // ID of the last toggled task
	statusMessage string // error message shown on save failure
	width         int
	height        int
}

// NewNoteTaskLinkerModel creates a new note-to-task linker.
func NewNoteTaskLinkerModel(project *models.Project, note *models.Note) *NoteTaskLinkerModel {
	var items []noteTaskItem
	for _, t := range project.Tasks {
		items = append(items, noteTaskItem{task: t})
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Link Note to Task",
		Subtitle:       note.Title,
		EmptyMessage:   "No tasks in project.",
		NoMatchMessage: "No matching tasks.",
	})

	return &NoteTaskLinkerModel{
		picker:  picker,
		project: project,
		note:    note,
	}
}

// setStatus sets the status message.
func (m *NoteTaskLinkerModel) setStatus(msg string) {
	m.statusMessage = msg
}

// clearStatus clears the status message.
func (m *NoteTaskLinkerModel) clearStatus() {
	m.statusMessage = ""
}

func (m *NoteTaskLinkerModel) Init() tea.Cmd {
	return nil
}

func (m *NoteTaskLinkerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.picker.SetSize(width, height)
}

func (m *NoteTaskLinkerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Let picker handle search mode first
		handled, cmd := m.picker.HandleKeyMsg(msg)
		if handled {
			return m, cmd
		}

		switch msg.String() {
		case "q", "esc":
			return m, m.close()
		case "enter":
			return m, m.close()
		case " ":
			selected := m.picker.SelectedItem()
			if selected.task != nil {
				m.toggleLink(selected.task)
			}
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}
	return m, nil
}

func (m *NoteTaskLinkerModel) toggleLink(task *models.Task) {
	if m.note.HasLink(task.ID) {
		m.note.RemoveLink(task.ID)
	} else {
		m.note.AddLink(task.ID)
	}
	if err := storage.SaveNote(m.project, m.note); err != nil {
		m.setStatus(fmt.Sprintf("Failed to save note: %v", err))
		return
	}
	m.clearStatus()
	m.changed = true
	m.lastToggled = task.ID
}

func (m *NoteTaskLinkerModel) close() tea.Cmd {
	if m.changed {
		taskID := m.lastToggled
		return func() tea.Msg { return NoteTaskLinkedMsg{TaskID: taskID} }
	}
	return func() tea.Msg { return CloseNoteTaskLinkerMsg{} }
}

func (m *NoteTaskLinkerModel) View() string {
	// Already-linked task IDs for display
	linkedSet := map[string]bool{}
	for _, id := range m.note.LinkedTasks {
		linkedSet[id] = true
	}

	subtitle := m.picker.Config.Subtitle
	if len(m.note.LinkedTasks) > 0 {
		subtitle += fmt.Sprintf(" (%d linked)", len(m.note.LinkedTasks))
	}

	// Show linked indicator on items
	var content strings.Builder
	content.WriteString(m.picker.RenderSearchBar())

	maxVisible := 15
	if m.height > 0 {
		maxVisible = m.height - 14
	}

	// Custom render to show linked status
	if len(m.picker.Filtered) == 0 {
		if len(m.picker.Items) == 0 {
			content.WriteString(MutedStyle.Render(m.picker.Config.EmptyMessage))
		} else {
			content.WriteString(MutedStyle.Render(m.picker.Config.NoMatchMessage))
		}
		content.WriteString("\n")
	} else {
		start := 0
		if m.picker.Cursor >= maxVisible {
			start = m.picker.Cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.picker.Filtered))

		for i := start; i < end; i++ {
			item := m.picker.Filtered[i]
			selected := i == m.picker.Cursor

			isLinked := linkedSet[item.ItemID()]
			display := item.DisplayText()
			if isLinked {
				display = PurpleStyle.Render(display + " ✓")
			}

			line := RenderSelectableRow(selected, false, false, false, display, 55)
			content.WriteString(line)
			content.WriteString("\n")
		}

		if remaining := len(m.picker.Filtered) - end; remaining > 0 {
			content.WriteString(MutedStyle.Render(fmt.Sprintf("\n... and %d more (use j/k to scroll)", remaining)))
			content.WriteString("\n")
		}
	}

	var helpText string
	if m.picker.IsSearchMode() {
		helpText = "enter done · esc clear"
	} else {
		helpText = "space toggle · / search · enter close"
	}

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Link Note to Task", subtitle))
	out.WriteString("\n\n")
	out.WriteString(content.String())
	out.WriteString("\n")

	statusLeft := HelpStyle.Render(helpText)
	if m.statusMessage != "" {
		statusLeft = ValidationErrorStyle.Render(m.statusMessage)
	}
	out.WriteString(renderStatusBarFooter(statusLeft, "", m.width))

	return out.String()
}

func (m *NoteTaskLinkerModel) StatusBarLabel() string { return "Link to Task" }
