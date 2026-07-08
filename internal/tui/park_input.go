package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ParkInputModel handles parking a task with a one-line resume note.
type ParkInputModel struct {
	task         *models.Task
	project      *models.Project
	input        textinput.Model
	err          error
	width        int
	height       int
	initialValue string
	promptMode   bool
	discardConfirmState
}

// NewParkInputModel creates a new park input model.
func NewParkInputModel(task *models.Task, project *models.Project) *ParkInputModel {
	input := textinput.New()
	input.Placeholder = "state + next step"
	input.Focus()
	input.CharLimit = 240
	input.SetWidth(60)
	applyThemedTextInput(&input)

	if task != nil && strings.TrimSpace(task.ParkedNote) != "" {
		input.SetValue(task.ParkedNote)
	}

	return &ParkInputModel{
		task:         task,
		project:      project,
		input:        input,
		initialValue: input.Value(),
	}
}

// SetSize sets the view dimensions.
func (m *ParkInputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(max(20, min(width-20, 60)))
}

func (m *ParkInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *ParkInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.promptMode {
				return m, parkNoteDoneCmd(m.task, m.project, "", false, true)
			}
			if m.input.Value() != m.initialValue {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }

		case "enter":
			note := strings.TrimSpace(m.input.Value())
			unparked := false
			if !m.project.MutateTask(m.task.ID, func(t *models.Task) {
				if note == "" && t.IsParked() {
					t.Unpark(false)
					unparked = true
					return
				}
				t.Park(note)
			}) {
				m.err = fmt.Errorf("task no longer exists in project")
				return m, func() tea.Msg { return BackMsg{} }
			}
			if fresh := m.project.GetTaskByID(m.task.ID); fresh != nil {
				m.task = fresh
			}
			return m, parkNoteDoneCmd(m.task, m.project, note, unparked, false)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func parkNoteDoneCmd(task *models.Task, project *models.Project, note string, unparked, skipped bool) tea.Cmd {
	return func() tea.Msg {
		return ParkNoteDoneMsg{
			Task:     task,
			Project:  project,
			Note:     note,
			Unparked: unparked,
			Skipped:  skipped,
		}
	}
}

func (m *ParkInputModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderTaskScopedFullscreenTitle(
		fmt.Sprintf("Park Task #%d  ", m.task.ShortID),
		m.task,
		"",
		m.width))
	b.WriteString("\n\n")

	if m.task.IsParked() {
		b.WriteString(FieldLabelStyle.Render("Current: "))
		if strings.TrimSpace(m.task.ParkedNote) != "" {
			b.WriteString(m.task.ParkedNote)
		} else {
			b.WriteString("(parked without note)")
		}
		if m.task.ParkedAt != nil {
			b.WriteString("  ")
			b.WriteString(MutedStyle.Render(m.task.ParkedAt.Format(util.DateTimeFormat)))
		}
		b.WriteString("\n\n")
	}

	b.WriteString(FieldLabelFocusedStyle.Render("Resume note: "))
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	bodyLines := []string{
		BoldStyle.Render("Controls"),
		"  Enter          Save",
	}
	if m.promptMode {
		bodyLines = append(bodyLines, "  Esc            Skip")
	} else {
		bodyLines = append(bodyLines, "  Esc            Cancel")
	}
	if m.task.IsParked() {
		bodyLines = append(bodyLines, "  Empty Enter    Unpark")
	}
	if help := clampHelpBoxToHeight(bodyLines, m.height, b.String()); help != "" {
		b.WriteString(help)
	} else {
		return tea.NewView(strings.TrimRight(b.String(), "\n"))
	}

	return tea.NewView(b.String())
}

// HasTextInput returns true because the park input is always active.
func (m *ParkInputModel) HasTextInput() bool { return true }

// ShouldCaptureEsc returns true so esc is handled by the view.
func (m *ParkInputModel) ShouldCaptureEsc() bool { return true }

func (m *ParkInputModel) IsFullHeight() bool { return false }

func (m *ParkInputModel) StatusBarLabel() string { return "Park" }
