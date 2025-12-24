package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// ProjectListModel represents the project list view.
type ProjectListModel struct {
	config       *config.Config
	projects     []*models.Project
	cursor       int
	showArchived bool
	width        int
	height       int
}

// NewProjectListModel creates a new project list model.
func NewProjectListModel(cfg *config.Config) *ProjectListModel {
	m := &ProjectListModel{
		config:       cfg,
		cursor:       0,
		showArchived: false,
	}
	m.loadProjects()
	return m
}

func (m *ProjectListModel) loadProjects() {
	if m.showArchived {
		m.projects = m.config.GetArchivedProjects()
	} else {
		m.projects = m.config.GetActiveProjects()
	}

	for _, p := range m.projects {
		storage.LoadProject(p)
	}
}

// SetSize sets the view dimensions.
func (m *ProjectListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ProjectListModel) Init() tea.Cmd {
	return nil
}

func (m *ProjectListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.projects)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.projects) > 0 && m.cursor < len(m.projects) {
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: m.projects[m.cursor]}
				}
			}
		case "a":
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
		case "r":
			m.loadProjects()
		}
	}

	return m, nil
}

func (m *ProjectListModel) View() string {
	var b strings.Builder

	// Title
	title := "Projects"
	if m.showArchived {
		title = "Archived Projects"
	}
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	if len(m.projects) == 0 {
		if m.showArchived {
			b.WriteString(MutedStyle.Render("No archived projects."))
		} else {
			b.WriteString(MutedStyle.Render("No projects. Create one with: tt project new <name>"))
		}
		b.WriteString("\n")
	} else {
		// Header
		header := fmt.Sprintf("  %-20s %-10s %8s %8s %8s", "Name", "Context", "Tasks", "Active", "Done")
		b.WriteString(MutedStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 60)))
		b.WriteString("\n")

		for i, p := range m.projects {
			cursor := "  "
			style := NormalStyle
			if i == m.cursor {
				cursor = "> "
				style = BoldStyle.Foreground(ColorPrimary)
			}

			nameStr := util.Truncate(p.Name, 20)
			if p.ID == m.config.CurrentProject {
				nameStr += " *"
			}
			name := style.Render(fmt.Sprintf("%-20s", nameStr))
			context := MutedStyle.Render(fmt.Sprintf("%-10s", p.Context))
			total := fmt.Sprintf("%8d", p.CountTasks())
			active := fmt.Sprintf("%8d", p.CountActiveTasks())
			done := fmt.Sprintf("%8d", p.CountCompletedTasks())

			line := fmt.Sprintf("%s%s %s %s %s %s", cursor, name, context, total, active, done)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	toggleText := "a: show archived"
	if m.showArchived {
		toggleText = "a: show active"
	}
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Enter: select | %s | r: refresh | esc: back", toggleText)))

	return b.String()
}
