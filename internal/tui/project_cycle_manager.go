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

// ShowContextPickerForCycleMsg is sent to open context picker from cycle manager.
type ShowContextPickerForCycleMsg struct{}

// NewProjectCycleManagerModel creates a new project cycle manager.
func NewProjectCycleManagerModel(cfg *config.Config, manager *config.Manager) *ProjectCycleManagerModel {
	projects := buildProjectInfoListFiltered(cfg, cfg.FilterContext, cfg.FilterCustomContext)

	picker := NewBasePicker(projects, PickerConfig{
		Title:          "Project Cycle Manager",
		Subtitle:       "Select projects for ` / ~ cycling",
		EmptyMessage:   "No projects available",
		NoMatchMessage: "No matching projects",
		MultiSelect:    true,
		ShowCounts:     true,
		BoxWidth:       60,
	})

	// Pre-select currently configured cycle projects for the current context
	contextKey := getContextKey(cfg.FilterContext, cfg.FilterCustomContext)
	cycleIDs := cfg.GetCycleProjectIDsForContext(contextKey)
	for _, id := range cycleIDs {
		picker.PreSelectByID(id)
	}

	return &ProjectCycleManagerModel{
		BasePicker:    picker,
		config:        cfg,
		configManager: manager,
	}
}

// getContextKey returns the context key for cycle project storage.
// Custom contexts use "+" prefix to distinguish from regular contexts.
func getContextKey(filterContext, filterCustomContext string) string {
	if filterCustomContext != "" {
		return "+" + filterCustomContext
	}
	return filterContext
}

// buildProjectInfoListFiltered gathers active projects filtered by context.
// If filterCustomContext is set, filters by custom context membership.
// Otherwise, if filterContext is set, filters by project context.
// If both are empty, returns all active projects.
func buildProjectInfoListFiltered(cfg *config.Config, filterContext, filterCustomContext string) []ProjectInfo {
	activeProjects := cfg.GetActiveProjects()
	projects := make([]ProjectInfo, 0, len(activeProjects))

	for _, p := range activeProjects {
		// Custom context filter takes precedence
		if filterCustomContext != "" {
			if !cfg.IsProjectInCustomContext(filterCustomContext, p.ID) {
				continue
			}
		} else if filterContext != "" && p.Context != filterContext {
			continue
		}
		_ = storage.LoadProject(p)
		projects = append(projects, ProjectInfo{
			Project:   p,
			TaskCount: p.CountTasks(),
		})
	}

	return projects
}

// RefreshWithContext reloads the project list with the current context filter.
// Also reloads selections from the context-specific cycle list.
func (m *ProjectCycleManagerModel) RefreshWithContext() {
	currentID := ""
	if item := m.SelectedItem(); item.Project != nil {
		currentID = item.Project.ID
	}
	m.SetItems(buildProjectInfoListFiltered(m.config, m.config.FilterContext, m.config.FilterCustomContext))

	// Clear existing selections and reload from context-specific storage
	m.Selected = make(map[string]bool)
	contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
	cycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
	for _, id := range cycleIDs {
		m.Selected[id] = true
	}

	if currentID != "" {
		m.SetCursorToID(currentID)
	}
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
			// Use context-specific storage when a context filter is active
			ids := m.getSelectedIDsInOrder()
			contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
			m.config.SetCycleProjectsForContext(contextKey, ids)
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
				m.SetItems(buildProjectInfoListFiltered(m.config, m.config.FilterContext, m.config.FilterCustomContext))
				m.SetCursorToID(currentID)
			}
			return m, nil

		case "c":
			// Show context picker
			return m, func() tea.Msg {
				return ShowContextPickerForCycleMsg{}
			}

		case "C":
			// Clear context filter (both regular and custom)
			if m.config.FilterContext != "" || m.config.FilterCustomContext != "" {
				m.config.FilterContext = ""
				m.config.FilterCustomContext = ""
				if err := m.configManager.Save(m.config); err != nil {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
					}
				}
				m.RefreshWithContext()
			}
			return m, nil

		case "u":
			// Clear selection for visible (filtered) projects
			for _, item := range m.Filtered {
				delete(m.Selected, item.ItemID())
			}
			return m, nil

		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewProjectCycleManager,
					ViewName:   "Project Cycle",
					Items:      m.GetHelpItems(),
				}
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

	// Build subtitle: context indicator + selection count
	var subtitleParts []string

	// Context indicator (colored) - custom context takes precedence
	contextStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	if m.config.FilterCustomContext != "" {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Context: %s", contextStyle.Render("+ "+m.config.FilterCustomContext)))
	} else if m.config.FilterContext != "" {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Context: %s", contextStyle.Render(m.config.FilterContext)))
	} else {
		subtitleParts = append(subtitleParts, "Context: All")
	}

	// Selection count
	selectedCount := len(m.Selected)
	if selectedCount == 0 {
		subtitleParts = append(subtitleParts, "No projects selected")
	} else {
		subtitleParts = append(subtitleParts, fmt.Sprintf("%d %s selected", selectedCount, pluralize(selectedCount, "project", "projects")))
	}

	subtitle := strings.Join(subtitleParts, " · ")

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
		if m.config.FilterContext != "" {
			return MutedStyle.Render("c/C context set/clear | esc cancel")
		}
		return MutedStyle.Render("c context | esc cancel")
	}

	return MutedStyle.Render("space toggle | / search | c/C context | u clear\np pin | g go | enter apply | esc cancel")
}

// GetHelpItems returns the help items for the project cycle manager.
func (m *ProjectCycleManagerModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewProjectCycleManager)
}
