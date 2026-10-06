package taskform

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// The inspection is a read-only snapshot inside capture. It never navigates
// the host or replaces the textarea, so returning preserves the entire draft.
func (m *TaskComposerModel) openMatchingTaskPreview() {
	m.UpdatePreview()
	task, _, _ := m.MatchingCaptureTask()
	if task == nil {
		return
	}
	lines := []string{m.DuplicateCaptureHint(), "", task.Title,
		"Status: " + models.WorkflowStateGroupForState(task.WorkflowState).Label()}
	if task.Priority != models.PriorityUnset {
		lines = append(lines, "Priority: "+models.PriorityName(task.Priority))
	}
	if task.DueDate != nil {
		date := task.DueDate.Format("Mon, Jan 2 2006")
		if clock := chrome.FormatDueClock(task); clock != "" {
			date += " " + clock
		}
		lines = append(lines, "Due: "+date)
	}
	if len(task.Tags) > 0 {
		lines = append(lines, "Tags: "+strings.Join(task.Tags, ", "))
	}
	if task.Context != "" {
		lines = append(lines, "Context: "+task.Context)
	}
	if task.ResumeNote != "" {
		lines = append(lines, "", "Resume: "+task.ResumeNote)
	}
	if task.Description != "" {
		lines = append(lines, "", task.Description)
	}
	for _, item := range task.Checklist {
		mark := " "
		if item.Done {
			mark = "x"
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", mark, item.Text))
	}
	m.matchingTaskText = strings.Join(lines, "\n")
	vp := viewport.New()
	m.matchingTaskPreview = &vp
	m.resizeMatchingTaskPreview()
}

func (m *TaskComposerModel) resizeMatchingTaskPreview() {
	vp := m.matchingTaskPreview
	vp.SetWidth(max(m.width, 1))
	vp.SetHeight(max(m.height-m.shellFrame.ExcessRows()-1, 1))
	vp.SetContent(strings.Join(textfit.WrapText(m.matchingTaskText, max(m.width, 1)), "\n"))
}

func (m *TaskComposerModel) updateMatchingTaskPreview(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc", "ctrl+o":
			m.matchingTaskPreview = nil
			m.matchingTaskText = ""
			return nil
		}
	}
	var cmd tea.Cmd
	*m.matchingTaskPreview, cmd = m.matchingTaskPreview.Update(msg)
	return cmd
}
