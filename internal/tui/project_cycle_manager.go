package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// ProjectCycleManagerModel allows selecting projects for quick cycling with ` and ~.
type ProjectCycleManagerModel struct {
	*BasePicker[ProjectInfo]
	config        *config.Config
	configManager *config.Manager
}

// ShowProjectCycleManagerMsg is sent to open the project cycle manager.
type ShowProjectCycleManagerMsg struct{}

// CloseProjectCycleManagerMsg is sent when the project cycle manager is done.
type CloseProjectCycleManagerMsg struct {
	Changed bool
}

// NewProjectCycleManagerModel creates a new project cycle manager.
func NewProjectCycleManagerModel(cfg *config.Config, manager *config.Manager) *ProjectCycleManagerModel {
	projects := buildProjectInfoList(cfg)

	picker := NewBasePicker(projects, PickerConfig{
		Title:          "Project Cycle Manager",
		Subtitle:       "Select projects for ` / ~ cycling",
		EmptyMessage:   "No projects available",
		NoMatchMessage: "No matching projects",
		MultiSelect:    true,
		ShowCounts:     true,
		BoxWidth:       60,
	})

	// Pre-select currently configured cycle projects
	for _, id := range cfg.CycleProjects {
		picker.PreSelectByID(id)
	}

	return &ProjectCycleManagerModel{
		BasePicker:    picker,
		config:        cfg,
		configManager: manager,
	}
}

// buildProjectInfoList gathers all active projects with task counts.
func buildProjectInfoList(cfg *config.Config) []ProjectInfo {
	activeProjects := cfg.GetActiveProjects()
	projects := make([]ProjectInfo, 0, len(activeProjects))

	for _, p := range activeProjects {
		storage.LoadProject(p)
		projects = append(projects, ProjectInfo{
			Project:   p,
			TaskCount: p.CountTasks(),
		})
	}

	return projects
}

// Init initializes the model.
func (m *ProjectCycleManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *ProjectCycleManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return CloseProjectCycleManagerMsg{Changed: false}
			}

		case "enter":
			// Save selected projects to config, preserving selection order
			ids := m.getSelectedIDsInOrder()
			m.config.SetCycleProjects(ids)
			if err := m.configManager.Save(m.config); err != nil {
				return m, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
				}
			}
			return m, func() tea.Msg {
				return CloseProjectCycleManagerMsg{Changed: true}
			}

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

// getSelectedIDsInOrder returns selected project IDs in the order they appear in the list.
func (m *ProjectCycleManagerModel) getSelectedIDsInOrder() []string {
	var ids []string
	for _, item := range m.Items {
		if m.Selected[item.ItemID()] {
			ids = append(ids, item.ItemID())
		}
	}
	return ids
}

// View renders the project cycle manager.
func (m *ProjectCycleManagerModel) View() string {
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Project Cycle Manager"))
	content.WriteString("\n")

	// Selection count
	selectedCount := len(m.Selected)
	if selectedCount == 0 {
		content.WriteString(MutedStyle.Render("No projects selected for cycling"))
	} else {
		content.WriteString(MutedStyle.Render(fmt.Sprintf("Selected: %d %s", selectedCount, pluralize(selectedCount, "project", "projects"))))
	}
	content.WriteString("\n\n")

	// Search bar
	content.WriteString(m.RenderSearchBar())

	// List
	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 14
	}
	if maxVisible < 5 {
		maxVisible = 5
	}
	content.WriteString(m.RenderList(maxVisible))

	// Footer
	content.WriteString("\n")
	content.WriteString(m.RenderFooter())

	return RenderCenteredBox(content.String(), boxWidth, m.Width(), m.Height(), TagManagerBoxStyle)
}
