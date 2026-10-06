package taskform

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// The inspection is a read-only snapshot inside capture. It never navigates
// the host or replaces the textarea, so returning preserves the entire draft.
func (m *TaskComposerModel) openMatchingTaskPreview() {
	m.UpdatePreview()
	tasks, project := m.MatchingCaptureTasks()
	if len(tasks) == 0 {
		return
	}
	m.matchingTaskTexts = make([]string, len(tasks))
	m.matchingTaskReferences = make([]string, len(tasks))
	now := time.Now()
	for i, task := range tasks {
		m.matchingTaskTexts[i] = m.matchingTaskDocument(task, project, now)
		m.matchingTaskReferences[i] = fmt.Sprintf("%s#%d", project.Slug, task.ShortID)
	}
	m.matchingTaskIndex = 0
	vp := viewport.New()
	m.matchingTaskPreview = &vp
	m.showMatchingTask()
}

func (m *TaskComposerModel) matchingTaskDocument(task *models.Task, project *models.Project, now time.Time) string {
	identity := fmt.Sprintf("%s#%d", project.Slug, task.ShortID)
	if task.ParentID != nil {
		if parent := project.GetTaskByID(*task.ParentID); parent != nil {
			identity += fmt.Sprintf(" · under #%d %s", parent.ShortID, parent.Title)
		}
	}
	lines := []string{identity, "", task.Title}
	if m.runtime != nil {
		locator := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
		planning := m.runtime.Planning()
		switch {
		case slices.Contains(planning.GetToday(), locator):
			lines = append(lines, chrome.PlanFieldLabel+": Today")
		case slices.Contains(planning.GetNext(), locator):
			lines = append(lines, chrome.PlanFieldLabel+": Next")
		}
	}
	lines = append(lines, chrome.StatusFieldLabel+": "+models.WorkflowStateGroupForState(task.WorkflowState).Label())
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
	if task.IsDeferredAt(now) {
		lines = append(lines, "Defer until: "+chrome.FormatDeferDate(*task.DeferredUntil, now))
	}
	if task.NextCheckIn != nil {
		lines = append(lines, "Check-in: "+task.NextCheckIn.Format("Mon, Jan 2 2006"))
		repeat := "off"
		if task.CheckInRepeatDays > models.CheckInRepeatOff {
			repeat = fmt.Sprintf("every %d days", task.CheckInRepeatDays)
		}
		lines = append(lines, "Repeat: "+repeat)
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
	if len(task.Attachments) > 0 {
		lines = append(lines, "", fmt.Sprintf("Attachments: %d", len(task.Attachments)))
		for _, attachment := range task.Attachments {
			lines = append(lines, listsurface.AttachmentDisplayName(attachment))
		}
	}
	for _, comment := range task.Comments {
		if comment == nil {
			continue
		}
		label := "Comment"
		if !comment.CreatedAt.IsZero() {
			label += " " + comment.CreatedAt.Local().Format("2006-01-02 15:04")
		}
		lines = append(lines, "", label+":", comment.Text)
	}
	return strings.Join(lines, "\n")
}

func (m *TaskComposerModel) showMatchingTask() {
	m.matchingTaskText = m.matchingTaskTexts[m.matchingTaskIndex]
	m.resizeMatchingTaskPreview()
	m.matchingTaskPreview.GotoTop()
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
		case "y":
			reference := m.matchingTaskReferences[m.matchingTaskIndex]
			return func() tea.Msg {
				if err := external.CopyText(reference); err != nil {
					return status.ErrorMsg{Err: fmt.Errorf("couldn't copy task reference: %w", err)}
				}
				return status.InfoStatusMsg("Copied task reference: " + reference)
			}
		case "left", "right":
			next := m.matchingTaskIndex
			if key.String() == "left" {
				next--
			} else {
				next++
			}
			if next >= 0 && next < len(m.matchingTaskTexts) {
				m.matchingTaskIndex = next
				m.showMatchingTask()
			}
			return nil
		case "esc", "ctrl+o":
			m.matchingTaskPreview = nil
			m.matchingTaskText = ""
			m.matchingTaskTexts = nil
			m.matchingTaskReferences = nil
			m.matchingTaskIndex = 0
			return nil
		}
	}
	var cmd tea.Cmd
	*m.matchingTaskPreview, cmd = m.matchingTaskPreview.Update(msg)
	return cmd
}
