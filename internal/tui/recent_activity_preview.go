package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// RecentActivityPreviewModel shows a scrollable task preview overlay.
type RecentActivityPreviewModel struct {
	task     CrossProjectTask
	project  *models.Project
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

// NewRecentActivityPreviewModel creates a new preview model.
func NewRecentActivityPreviewModel(task CrossProjectTask, project *models.Project) *RecentActivityPreviewModel {
	return &RecentActivityPreviewModel{
		task:    task,
		project: project,
	}
}

// SetSize sets the view dimensions.
func (m *RecentActivityPreviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	headerHeight := 3
	footerHeight := 2

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(width), viewport.WithHeight(height-headerHeight-footerHeight))
		m.viewport.YPosition = headerHeight
		m.ready = true
	} else {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(height - headerHeight - footerHeight)
	}

	m.viewport.SetContent(m.renderContent())
}

func (m *RecentActivityPreviewModel) Init() tea.Cmd {
	return nil
}

func (m *RecentActivityPreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "p", "esc":
			return m, func() tea.Msg {
				return CloseRecentActivityPreviewMsg{}
			}
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "pgup", "ctrl+u":
			m.viewport.HalfPageUp()
		case "pgdown", "ctrl+d":
			m.viewport.HalfPageDown()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *RecentActivityPreviewModel) renderContent() string {
	task := m.task.Task
	if task == nil {
		return MutedStyle.Render("No task selected.")
	}

	var b strings.Builder

	// Project name
	if m.task.Project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(m.task.Project.Name))
		b.WriteString("\n\n")
	}

	// Type and title header
	typeStyle := GetTypeStyle(string(task.Type))
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title))
	styledHeader := TaskJumperPreviewHeaderStyle.Render(header)
	b.WriteString(lipgloss.NewStyle().Width(max(m.width-2, 1)).Render(styledHeader))
	b.WriteString("\n\n")

	// Status
	statusStyle := GetStatusStyle(string(task.Status))
	fmt.Fprintf(&b, "%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status)))

	// Priority
	priorityStyle := GetPriorityStyle(task.Priority)
	fmt.Fprintf(&b, "%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority)))

	// Tags
	if len(task.Tags) > 0 {
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = TagInlineStyle.Render(tag)
		}
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", "))
	}

	// Section
	if task.Section != "" {
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Section:"),
			task.Section)
	}

	// Due date
	if task.DueDate != nil {
		dueStyle := DueDateStyle
		now := time.Now()
		today := util.StartOfDay(now)
		dueDay := util.StartOfDay(*task.DueDate)
		if dueDay.Before(today) {
			dueStyle = OverdueStyle
		} else if dueDay.Equal(today) {
			dueStyle = DueTodayStyle
		}
		dateStr := task.DueDate.Format(util.DateFormat)
		dateStr = AppendTimeIfHasTime(task, dateStr, "15:04")
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Due:"),
			dueStyle.Render(dateStr))
	}

	// Parent
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		fmt.Fprintf(&b, "%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(parentRef))
	}

	// Subtasks
	if len(task.Children) > 0 {
		completed := 0
		for _, child := range task.Children {
			if child.Status.IsComplete() {
				completed++
			}
		}
		fmt.Fprintf(&b, "%s %d/%d done\n",
			MutedStyle.Render("Subtasks:"),
			completed, len(task.Children))
	}

	// Dependencies
	if m.task.Project != nil {
		blockers := m.task.Project.GetBlockingTasks(task)
		if len(blockers) > 0 {
			fmt.Fprintf(&b, "%s ", MutedStyle.Render("Blocked by:"))
			maxShow := min(len(blockers), 3)
			var refs []string
			for i := range maxShow {
				blocker := blockers[i]
				ss := GetStatusStyle(string(blocker.Status))
				refs = append(refs, fmt.Sprintf("%s #%d", ss.Render(blocker.Status.Icon()), blocker.ShortID))
			}
			b.WriteString(strings.Join(refs, ", "))
			if len(blockers) > maxShow {
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d more", len(blockers)-maxShow)))
			}
			b.WriteString("\n")
		}

		blocked := m.task.Project.GetBlockedByTasks(task)
		if len(blocked) > 0 {
			fmt.Fprintf(&b, "%s ", MutedStyle.Render("Blocking:"))
			maxShow := min(len(blocked), 3)
			var refs []string
			for i := range maxShow {
				dep := blocked[i]
				ss := GetStatusStyle(string(dep.Status))
				refs = append(refs, fmt.Sprintf("%s #%d", ss.Render(dep.Status.Icon()), dep.ShortID))
			}
			b.WriteString(strings.Join(refs, ", "))
			if len(blocked) > maxShow {
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" +%d more", len(blocked)-maxShow)))
			}
			b.WriteString("\n")
		}
	}

	// Description
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(renderMarkdown(task.Description, m.width-2))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *RecentActivityPreviewModel) View() tea.View {
	task := m.task.Task
	if task == nil {
		return tea.NewView(MutedStyle.Render("No task selected."))
	}

	var b strings.Builder

	// Title
	title := fmt.Sprintf("Task: #%d %s",
		task.ShortID,
		TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title))
	b.WriteString(lipgloss.NewStyle().Width(m.width).Render(HelpViewTitleStyle.Render(title)))
	b.WriteString("\n\n")

	// Viewport
	if m.ready {
		b.WriteString(m.viewport.View())
	}

	// Footer
	b.WriteString("\n")
	scrollPercent := ""
	if m.ready && m.viewport.TotalLineCount() > m.viewport.Height() {
		scrollPercent = fmt.Sprintf(" %3.0f%%", m.viewport.ScrollPercent()*100)
	}
	footer := HelpStyle.Render(fmt.Sprintf("p/esc close · j/k scroll · g/G top/bottom%s", scrollPercent))
	b.WriteString(footer)

	return tea.NewView(b.String())
}

func (m *RecentActivityPreviewModel) StatusBarLabel() string { return "Recent Activity" }
