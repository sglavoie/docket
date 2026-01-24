package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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
		_ = storage.LoadProject(p)
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
		// Let search mode handle all input
		if m.IsSearchMode() {
			_, cmd := m.HandleKeyMsg(msg)
			return m, cmd
		}

		switch msg.String() {
		case "esc":
			// If there's an active filter, clear it first
			if m.SearchQuery() != "" {
				m.ClearSearch()
				m.Filter("")
				return m, nil
			}
			return m, func() tea.Msg {
				return CloseProjectCycleManagerMsg{Changed: false}
			}

		case "q":
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

		case "g":
			// Go to the highlighted project
			item := m.SelectedItem()
			if item.Project != nil {
				return m, func() tea.Msg {
					return SelectProjectMsg{Project: item.Project}
				}
			}
			return m, nil

		case "p":
			// Toggle pin status on highlighted project
			item := m.SelectedItem()
			if item.Project != nil {
				item.Project.TogglePinned()
				if err := m.configManager.Save(m.config); err != nil {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
					}
				}
				// Refresh list (re-sorts with pinned first)
				currentID := item.Project.ID
				m.SetItems(buildProjectInfoList(m.config))
				m.SetCursorToID(currentID)
			}
			return m, nil

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

	// Build subtitle from selection count
	selectedCount := len(m.Selected)
	var subtitle string
	if selectedCount == 0 {
		subtitle = "No projects selected"
	} else {
		subtitle = fmt.Sprintf("%d %s selected", selectedCount, pluralize(selectedCount, "project", "projects"))
	}

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Project Cycle",
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}

// ShouldCaptureEsc returns true when in search mode or has active filter.
func (m *ProjectCycleManagerModel) ShouldCaptureEsc() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when in search mode.
func (m *ProjectCycleManagerModel) HasTextInput() bool {
	return m.IsSearchMode()
}

// RenderFooter renders context-appropriate keyboard hints (overrides BasePicker).
func (m *ProjectCycleManagerModel) RenderFooter() string {
	if m.IsSearchMode() {
		return MutedStyle.Render("enter done | esc clear")
	}

	if len(m.Filtered) == 0 {
		return MutedStyle.Render("esc cancel")
	}

	return MutedStyle.Render("space toggle | / search | c clear\np pin | g go to project | enter apply | esc cancel")
}
