package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// noteTaskItem wraps a task for use with ListSurfaceAdapter.
type noteTaskItem struct {
	task *models.Task
}

func (i noteTaskItem) FilterText() string { return i.task.Title }
func (i noteTaskItem) DisplayText() string {
	return fmt.Sprintf("#%d %s", i.task.ShortID, TaskTitleStyleFor(i.task, TaskTitleStyleOptions{}).Render(i.task.Title))
}
func (i noteTaskItem) CountValue() int { return 0 }
func (i noteTaskItem) ItemID() string  { return i.task.ID }

// NoteTaskLinkerModel is a task picker for linking notes to tasks.
type NoteTaskLinkerModel struct {
	picker        *ListSurfaceAdapter[noteTaskItem]
	project       *models.Project
	note          *models.Note
	noteService   projectNoteService
	deferCommit   bool
	linkChanges   map[string]bool
	changed       bool
	lastToggled   string // ID of the last toggled task
	statusMessage string // error message shown on save failure
	width         int
	height        int
}

// NewNoteTaskLinkerModelWithService creates a linker backed by the typed note
// application API. Bulk mode defers all toggles and commits them as one batch
// when the overlay closes.
func NewNoteTaskLinkerModelWithService(project *models.Project, note *models.Note, service projectNoteService, deferCommit bool) *NoteTaskLinkerModel {
	var items []noteTaskItem
	for _, t := range project.Tasks {
		items = append(items, noteTaskItem{task: t})
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{
		Title:          "Link Note to Task",
		Subtitle:       note.Title,
		EmptyMessage:   "No tasks in project.",
		NoMatchMessage: "No matching tasks.",
	})

	return &NoteTaskLinkerModel{
		picker:      picker,
		project:     project,
		note:        cloneNoteForCommand(note),
		noteService: service,
		deferCommit: deferCommit,
		linkChanges: make(map[string]bool),
	}
}

type noteTaskLinkCommitResultMsg struct {
	Result app.NoteResult
	TaskID string
}

type noteTaskLinkCommitErrorMsg struct{ Err error }

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
	case tea.KeyPressMsg:
		// Let picker handle search mode first
		handled, cmd := m.picker.HandleKeyMsg(msg)
		if handled {
			return m, cmd
		}

		switch msg.String() {
		case "esc":
			return m, m.close()
		case "enter":
			return m, m.close()
		case "space":
			selected := m.picker.SelectedItem()
			if selected.task != nil {
				return m, m.toggleLink(selected.task)
			}
		}
	case noteTaskLinkCommitResultMsg:
		if msg.Result.Note != nil {
			m.note = cloneNoteForCommand(msg.Result.Note)
		}
		m.clearStatus()
		m.changed = true
		m.lastToggled = msg.TaskID
	case noteTaskLinkCommitErrorMsg:
		m.setStatus(fmt.Sprintf("Failed to save note: %v", msg.Err))
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}
	return m, nil
}

func (m *NoteTaskLinkerModel) toggleLink(task *models.Task) tea.Cmd {
	if task == nil || m.note == nil || m.project == nil {
		return nil
	}
	if m.deferCommit {
		if m.note.HasLink(task.ID) {
			m.note.RemoveLink(task.ID)
			m.linkChanges[task.ID] = false
		} else {
			m.note.AddLink(task.ID)
			m.linkChanges[task.ID] = true
		}
		m.changed = true
		m.lastToggled = task.ID
		m.clearStatus()
		return nil
	}

	updated := cloneNoteForCommand(m.note)
	if updated.HasLink(task.ID) {
		updated.RemoveLink(task.ID)
	} else {
		updated.AddLink(task.ID)
	}
	request := cloneNoteBatchRequest(app.NoteBatchRequest{
		Operations: []app.NoteMutationRequest{{
			Kind:     app.NoteUpdate,
			Location: app.ProjectNoteLocation(m.project.ID),
			Note:     updated,
		}},
		UndoAction:  "Link Note to Task",
		UndoContext: m.note.Title,
	})
	service := m.noteService
	taskID := task.ID
	return func() tea.Msg {
		if service == nil {
			return noteTaskLinkCommitErrorMsg{Err: fmt.Errorf("project note service is unavailable")}
		}
		result, err := service.ApplyNoteBatch(request)
		if err != nil {
			return noteTaskLinkCommitErrorMsg{Err: err}
		}
		return noteTaskLinkCommitResultMsg{Result: result.Results[0], TaskID: taskID}
	}
}

func (m *NoteTaskLinkerModel) close() tea.Cmd {
	if m.deferCommit {
		if !m.changed {
			return func() tea.Msg { return CloseNoteTaskLinkerMsg{} }
		}
		changes := make(map[string]bool, len(m.linkChanges))
		for taskID, linked := range m.linkChanges {
			changes[taskID] = linked
		}
		projectID := ""
		if m.project != nil {
			projectID = m.project.ID
		}
		return func() tea.Msg {
			return BulkNoteLinkDoneMsg{ProjectID: projectID, LinkChanges: changes}
		}
	}
	if m.changed {
		taskID := m.lastToggled
		return func() tea.Msg { return NoteTaskLinkedMsg{TaskID: taskID} }
	}
	return func() tea.Msg { return CloseNoteTaskLinkerMsg{} }
}

func (m *NoteTaskLinkerModel) View() tea.View {
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
	if len(m.picker.FilteredItems()) == 0 {
		if len(m.picker.SourceItems()) == 0 {
			content.WriteString(MutedStyle.Render(m.picker.Config.EmptyMessage))
		} else {
			content.WriteString(MutedStyle.Render(m.picker.Config.NoMatchMessage))
		}
		content.WriteString("\n")
	} else {
		start := 0
		if m.picker.CursorIndex() >= maxVisible {
			start = m.picker.CursorIndex() - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.picker.FilteredItems()))

		for i := start; i < end; i++ {
			item := m.picker.FilteredItems()[i]
			selected := i == m.picker.CursorIndex()

			isLinked := linkedSet[item.ItemID()]
			display := item.DisplayText()
			if isLinked {
				display = PurpleStyle.Render(display + " ✓")
			}

			line := RenderSelectableRow(selected, false, false, false, false, display, 55)
			content.WriteString(line)
			content.WriteString("\n")
		}

		if remaining := len(m.picker.FilteredItems()) - end; remaining > 0 {
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

	return tea.NewView(out.String())
}

func (m *NoteTaskLinkerModel) StatusBarLabel() string { return "Link to Task" }
