package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// TaskPickerPurpose identifies what action the GlobalTaskPicker is serving.
type TaskPickerPurpose int

const (
	TaskPickerPurposeStatusUpdate TaskPickerPurpose = iota
	TaskPickerPurposeTaskMove
	TaskPickerPurposeLinkedTask
	TaskPickerPurposePriorityUpdate
	TaskPickerPurposeDueDate
	TaskPickerPurposeSectionChange
)

// ShowGlobalTaskPickerMsg opens the global task picker overlay for the given project.
type ShowGlobalTaskPickerMsg struct {
	Project *models.Project
	Purpose TaskPickerPurpose
	BackMsg tea.Msg // emitted on Esc to go back one step
}

// GlobalTaskPickerResultMsg is emitted when the user selects a task.
type GlobalTaskPickerResultMsg struct {
	Task    *models.Task
	Project *models.Project
	Purpose TaskPickerPurpose
}

// GlobalTaskPicker is an overlay for selecting a task from a project.
// Used in multi-step global workflows (status update, task move, linked task).
type GlobalTaskPicker struct {
	project       *models.Project
	tasks         []*models.Task // active tasks only (excludes done/canceled)
	filtered      []*models.Task
	cursor        int
	input         textinput.Model
	purpose       TaskPickerPurpose
	backMsg       tea.Msg // emitted on Esc to go back one step
	breadcrumb    []string
	breadcrumbIdx int
}

// taskPickerBreadcrumb returns flow step labels and the 0-based index for a task picker purpose.
func taskPickerBreadcrumb(purpose TaskPickerPurpose) (steps []string, stepIdx int) {
	switch purpose {
	case TaskPickerPurposeStatusUpdate:
		return []string{"Select Project", "Select Task", "Choose Status"}, 1
	case TaskPickerPurposeTaskMove:
		return []string{"Source Project", "Select Task", "Destination", "Section"}, 1
	case TaskPickerPurposeLinkedTask:
		return []string{"Select Project", "Select Task", "Link Type", "Link Mode"}, 1
	case TaskPickerPurposePriorityUpdate:
		return []string{"Select Project", "Select Task", "Set Priority"}, 1
	case TaskPickerPurposeDueDate:
		return []string{"Select Project", "Select Task", "Set Due Date"}, 1
	case TaskPickerPurposeSectionChange:
		return []string{"Select Project", "Select Task", "Choose Section"}, 1
	default:
		return nil, 0
	}
}

const globalTaskPickerMaxVisible = 12

// newGlobalTaskPicker creates a task picker for the given project and purpose.
// backMsg is emitted on Esc to allow the caller to go back one step.
func newGlobalTaskPicker(project *models.Project, purpose TaskPickerPurpose, backMsg tea.Msg) *GlobalTaskPicker {
	ti := textinput.New()
	ti.Placeholder = "Search tasks..."
	ti.CharLimit = 100
	ti.Focus()
	applyThemedTextInput(&ti)

	// Collect non-completed tasks
	var tasks []*models.Task
	for _, t := range project.Tasks {
		if !t.Status.IsComplete() {
			tasks = append(tasks, t)
		}
	}

	steps, stepIdx := taskPickerBreadcrumb(purpose)
	return &GlobalTaskPicker{
		project:       project,
		tasks:         tasks,
		filtered:      tasks,
		cursor:        0,
		input:         ti,
		purpose:       purpose,
		backMsg:       backMsg,
		breadcrumb:    steps,
		breadcrumbIdx: stepIdx,
	}
}

func (p *GlobalTaskPicker) initCmd() tea.Cmd {
	return textinput.Blink
}

// handleKey processes a key press.
// Returns (cmd, dismiss, resultMsg).
func (p *GlobalTaskPicker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, p.backMsg

	case "enter":
		if p.cursor < len(p.filtered) {
			selected := p.filtered[p.cursor]
			return nil, true, GlobalTaskPickerResultMsg{
				Task:    selected,
				Project: p.project,
				Purpose: p.purpose,
			}
		}
		return nil, false, nil

	case "up", "ctrl+k", "ctrl+p":
		p.moveCursor(-1)
		return nil, false, nil

	case "down", "ctrl+j", "ctrl+n":
		p.moveCursor(1)
		return nil, false, nil

	case "ctrl+u":
		p.moveCursor(-globalTaskPickerMaxVisible)
		return nil, false, nil

	case "ctrl+d":
		p.moveCursor(globalTaskPickerMaxVisible)
		return nil, false, nil

	default:
		prev := p.input.Value()
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		if p.input.Value() != prev {
			p.fuzzyFilter()
		}
		return cmd, false, nil
	}
}

func (p *GlobalTaskPicker) moveCursor(delta int) {
	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	maxCursor := max(len(p.filtered)-1, 0)
	if p.cursor > maxCursor {
		p.cursor = maxCursor
	}
}

func (p *GlobalTaskPicker) fuzzyFilter() {
	query := strings.TrimSpace(p.input.Value())

	if query == "" {
		p.filtered = p.tasks
		p.cursor = 0
		return
	}

	type scored struct {
		task  *models.Task
		score int
	}
	var results []scored

	for _, t := range p.tasks {
		s := fuzzy.ScoreTerms(t.Title, query)
		if s > 0 {
			results = append(results, scored{task: t, score: s})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	p.filtered = make([]*models.Task, len(results))
	for i, r := range results {
		p.filtered[i] = r.task
	}

	maxCursor := max(len(p.filtered)-1, 0)
	if p.cursor > maxCursor {
		p.cursor = maxCursor
	}
}

func (p *GlobalTaskPicker) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Select Task")
	footerText := HelpStyle.Render("Enter: select · ↑↓/Ctrl+j/k: navigate · Esc: back")

	// Box sizing: 70% of screen, min 50
	maxWidth := min(max(width*7/10, 50), width-4)

	titleMinInner := lipgloss.Width(title) + 6 - 2
	footerMinInner := lipgloss.Width(footerText) + 6 - 2
	innerWidth := max(maxWidth, max(titleMinInner, footerMinInner))

	p.input.SetWidth(max(innerWidth-2, 10))

	var content strings.Builder
	if bc := renderStepBreadcrumb(p.breadcrumb, p.breadcrumbIdx); bc != "" {
		content.WriteString(bc)
		content.WriteString("\n")
	}
	content.WriteString(p.input.View())
	content.WriteString("\n")

	if len(p.filtered) == 0 {
		content.WriteString(MutedStyle.Render("No matching tasks."))
	} else {
		visibleCount := min(len(p.filtered), globalTaskPickerMaxVisible)
		start := 0
		if p.cursor >= visibleCount {
			start = p.cursor - visibleCount + 1
		}
		end := min(start+visibleCount, len(p.filtered))

		for i := start; i < end; i++ {
			t := p.filtered[i]
			statusIcon := t.Status.Icon()
			priorityStr := ""
			if t.Priority > 0 {
				priorityStr = fmt.Sprintf(" !%d", t.Priority)
			}
			prefix := fmt.Sprintf(" %s ", statusIcon)
			priorityRendered := MutedStyle.Render(priorityStr)
			maxTitleLen := max(innerWidth-2-lipgloss.Width(prefix)-lipgloss.Width(priorityRendered), 1)
			taskTitle := truncateToVisualWidth(t.Title, maxTitleLen)
			titleRendered := TaskTitleStyleFor(t, TaskTitleStyleOptions{
				CursorSelected: i == p.cursor,
			}).Render(taskTitle)
			line := prefix + titleRendered + priorityRendered

			lineWidth := lipgloss.Width(line)
			if lineWidth < innerWidth-2 {
				line += strings.Repeat(" ", innerWidth-2-lineWidth)
			}

			if i == p.cursor {
				line = SelectedStyle.Render(line)
			}

			content.WriteString(line)
			if i < end-1 {
				content.WriteString("\n")
			}
		}

		if len(p.filtered) > visibleCount {
			indicator := MutedStyle.Render(fmt.Sprintf(" %d/%d", p.cursor+1, len(p.filtered)))
			content.WriteString("\n")
			content.WriteString(indicator)
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
