package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// ShowProjectPreviewMsg is sent to show the project preview.
type ShowProjectPreviewMsg struct {
	Project *models.Project
}

// CloseProjectPreviewMsg is sent when the project preview should close.
type CloseProjectPreviewMsg struct{}

// ProjectPreviewModel is the model for the project preview view.
type ProjectPreviewModel struct {
	project  *models.Project
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

// NewProjectPreviewModel creates a new project preview model.
func NewProjectPreviewModel(project *models.Project) *ProjectPreviewModel {
	return &ProjectPreviewModel{
		project: project,
		ready:   false,
	}
}

// SetSize sets the view dimensions.
func (m *ProjectPreviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	headerHeight := 3 // Title + spacing
	footerHeight := 2 // Footer hint

	if !m.ready {
		m.viewport = viewport.New(width, height-headerHeight-footerHeight)
		m.viewport.YPosition = headerHeight
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height - headerHeight - footerHeight
	}

	m.viewport.SetContent(m.renderContent())
}

func (m *ProjectPreviewModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectPreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "p", "q", "esc":
			return m, func() tea.Msg {
				return CloseProjectPreviewMsg{}
			}
		case "up", "k":
			m.viewport.LineUp(1)
		case "down", "j":
			m.viewport.LineDown(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "pgup", "ctrl+u":
			m.viewport.HalfViewUp()
		case "pgdown", "ctrl+d":
			m.viewport.HalfViewDown()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)

	return m, cmd
}

func (m *ProjectPreviewModel) renderContent() string {
	if m.project == nil {
		return MutedStyle.Render("No project selected.")
	}

	var b strings.Builder
	p := m.project

	// Context badge
	b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[%s]", p.Context)))
	b.WriteString("\n\n")

	// Description
	if p.Description != "" {
		b.WriteString(BoldStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(p.Description)
		b.WriteString("\n\n")
	}

	// Timestamps
	b.WriteString(BoldStyle.Render("Timestamps"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Created: %s\n", p.CreatedAt.Format(util.DateTimeFormat)))
	b.WriteString(fmt.Sprintf("  Updated: %s\n", p.UpdatedAt.Format(util.DateTimeFormat)))

	// Archived status
	if p.Archived && p.ArchivedAt != nil {
		b.WriteString(OverdueStyle.Render(fmt.Sprintf("  Archived: %s", p.ArchivedAt.Format(util.DateTimeFormat))))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Task summary
	total := p.CountTasks()
	active := p.CountActiveTasks()
	done := p.CountCompletedTasks()

	progress := 0.0
	if total > 0 {
		progress = float64(done) / float64(total) * 100
	}

	b.WriteString(BoldStyle.Render("Task Summary"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Total: %d  Active: %d  Completed: %d\n", total, active, done))
	b.WriteString(fmt.Sprintf("  Progress: %s %.0f%%\n", RenderProgressBar(progress, 20), progress))
	b.WriteString("\n")

	// Task type breakdown
	if total > 0 {
		typeCounts := make(map[models.TaskType]int)
		statusCounts := make(map[models.TaskStatus]int)

		for _, task := range p.Tasks {
			typeCounts[task.Type]++
			statusCounts[task.Status]++
		}

		b.WriteString(BoldStyle.Render("By Type"))
		b.WriteString("\n")
		var typeStats []string
		for _, tt := range models.AllTaskTypes() {
			if count := typeCounts[tt]; count > 0 {
				style := GetTypeStyle(string(tt))
				typeStats = append(typeStats, style.Render(fmt.Sprintf("  %s %s: %d", tt.Icon(), tt, count)))
			}
		}
		b.WriteString(strings.Join(typeStats, "\n"))
		b.WriteString("\n\n")

		b.WriteString(BoldStyle.Render("By Status"))
		b.WriteString("\n")
		var statusStats []string
		for _, s := range models.AllTaskStatuses() {
			if count := statusCounts[s]; count > 0 {
				style := GetStatusStyle(string(s))
				statusStats = append(statusStats, style.Render(fmt.Sprintf("  %s %s: %d", s.Icon(), s, count)))
			}
		}
		b.WriteString(strings.Join(statusStats, "\n"))
		b.WriteString("\n\n")

		// Overdue count
		overdueCount := 0
		today := time.Now().Truncate(24 * time.Hour)
		for _, task := range p.Tasks {
			if task.DueDate != nil && !task.Status.IsComplete() && task.DueDate.Before(today) {
				overdueCount++
			}
		}
		if overdueCount > 0 {
			b.WriteString(OverdueStyle.Render(fmt.Sprintf("Overdue Tasks: %d", overdueCount)))
			b.WriteString("\n\n")
		}

		// Recent tasks
		recentTasks := storage.GetRecentTasks(p.Tasks, 5)
		if len(recentTasks) > 0 {
			b.WriteString(BoldStyle.Render("Recent Tasks"))
			b.WriteString("\n")

			for _, task := range recentTasks {
				line := fmt.Sprintf("  %s %s %s %s  %s",
					MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)),
					GetTypeBadge(string(task.Type), task.Type.Icon()),
					GetStatusBadge(string(task.Status), task.Status.Icon()),
					util.Truncate(task.Title, 40),
					MutedStyle.Render(storage.FormatRelativeTime(task.UpdatedAt)))

				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func (m *ProjectPreviewModel) View() string {
	if m.project == nil {
		return MutedStyle.Render("No project selected.")
	}

	var b strings.Builder

	// Title
	title := fmt.Sprintf("Project: %s", m.project.Name)
	b.WriteString(HelpViewTitleStyle.Render(title))
	b.WriteString("\n\n")

	// Viewport with content
	if m.ready {
		b.WriteString(m.viewport.View())
	}

	// Footer
	b.WriteString("\n")
	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	footer := HelpStyle.Render(fmt.Sprintf("p/q/esc close | j/k scroll | g/G top/bottom%s", scrollPercent))
	b.WriteString(footer)

	return b.String()
}
