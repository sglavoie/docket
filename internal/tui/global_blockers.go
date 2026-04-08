package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// GlobalBlockerEntry pairs a blocker with its source project.
type GlobalBlockerEntry struct {
	Blocker *models.Blocker
	Project *models.Project
}

// GlobalBlockersModel displays and manages blockers across all active projects.
type GlobalBlockersModel struct {
	config  *config.Config
	entries []GlobalBlockerEntry
	state   blockerViewState
}

// NewGlobalBlockersModel loads blockers from all active projects.
func NewGlobalBlockersModel(cfg *config.Config) *GlobalBlockersModel {
	m := &GlobalBlockersModel{
		config: cfg,
		state:  newBlockerViewState(cfg.Editor),
	}
	m.loadAllBlockers()
	return m
}

func (m *GlobalBlockersModel) loadAllBlockers() {
	m.entries = nil
	for _, project := range m.config.GetActiveProjects() {
		blockers, err := storage.LoadBlockers(project)
		if err != nil {
			continue
		}
		for _, b := range blockers {
			m.entries = append(m.entries, GlobalBlockerEntry{
				Blocker: b,
				Project: project,
			})
		}
	}
}

func (m *GlobalBlockersModel) GetConfirmDialog() *ConfirmDialog { return m.state.GetConfirmDialog() }

// getTaskRef returns nil for both values — global blockers have no task reference.
func (m *GlobalBlockersModel) getTaskRef() (*models.Task, *models.Project) { return nil, nil }

func (m *GlobalBlockersModel) Init() tea.Cmd { return nil }

func (m *GlobalBlockersModel) SetSize(width, height int) {
	m.state.setSize(width, height)
}

func (m *GlobalBlockersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd, handled := blockerUpdateShared(&m.state, m, msg)
	if handled {
		return m, cmd
	}
	return m, nil
}

func (m *GlobalBlockersModel) View() tea.View {
	return tea.NewView(blockerViewShared(&m.state, m))
}

func (m *GlobalBlockersModel) HasTextInput() bool {
	return m.state.hasTextInput()
}

func (m *GlobalBlockersModel) ShouldCaptureEsc() bool {
	return m.state.shouldCaptureEsc()
}

// blockerDataSource implementation for global blockers.

func (m *GlobalBlockersModel) getVisibleItems(tab int) []blockerItem {
	var result []blockerItem
	switch tab {
	case blockersTabResolved:
		for _, e := range m.entries {
			if e.Blocker.IsResolved() {
				result = append(result, blockerItem(e))
			}
		}
	case blockersTabAll:
		for _, e := range m.entries {
			result = append(result, blockerItem(e))
		}
	default:
		for _, e := range m.entries {
			if !e.Blocker.IsResolved() {
				result = append(result, blockerItem(e))
			}
		}
	}
	return result
}

func (m *GlobalBlockersModel) activeCount() int {
	count := 0
	for _, e := range m.entries {
		if !e.Blocker.IsResolved() {
			count++
		}
	}
	return count
}

func (m *GlobalBlockersModel) resolvedCount() int {
	count := 0
	for _, e := range m.entries {
		if e.Blocker.IsResolved() {
			count++
		}
	}
	return count
}

func (m *GlobalBlockersModel) findItemIndex(blocker *models.Blocker) int {
	for i, e := range m.entries {
		if e.Blocker.ID == blocker.ID {
			return i
		}
	}
	return -1
}

// getProjectBlockers returns all blockers for a given project from the entries slice.
func (m *GlobalBlockersModel) getProjectBlockers(project *models.Project) []*models.Blocker {
	var blockers []*models.Blocker
	for _, e := range m.entries {
		if e.Project == project {
			blockers = append(blockers, e.Blocker)
		}
	}
	return blockers
}

func (m *GlobalBlockersModel) saveBlockerData(_ *models.Blocker, project *models.Project) error {
	return storage.SaveBlockers(project, m.getProjectBlockers(project))
}

func (m *GlobalBlockersModel) deleteBlockerData(idx int) (*models.Project, error) {
	project := m.entries[idx].Project
	m.entries = append(m.entries[:idx], m.entries[idx+1:]...)
	return project, storage.SaveBlockers(project, m.getProjectBlockers(project))
}

func (m *GlobalBlockersModel) updateBlockerData(idx int) (*models.Project, error) {
	project := m.entries[idx].Project
	return project, storage.SaveBlockers(project, m.getProjectBlockers(project))
}

func (m *GlobalBlockersModel) getAddProject() *models.Project {
	visible := m.getVisibleItems(m.state.tabs.Active)
	if len(visible) > 0 && m.state.cursor < len(visible) {
		return visible[m.state.cursor].Project
	}
	projects := m.config.GetActiveProjects()
	if len(projects) > 0 {
		return projects[0]
	}
	return nil
}

func (m *GlobalBlockersModel) getItemBlocker(idx int) *models.Blocker {
	return m.entries[idx].Blocker
}

func (m *GlobalBlockersModel) addBlocker(blocker *models.Blocker, project *models.Project) {
	m.entries = append(m.entries, GlobalBlockerEntry{
		Blocker: blocker,
		Project: project,
	})
}

func (m *GlobalBlockersModel) titleText() string {
	return "Blockers Across All Projects"
}

func (m *GlobalBlockersModel) showProjectName() bool {
	return true
}

func (m *GlobalBlockersModel) closeMsg() tea.Msg {
	return CloseGlobalBlockersMsg{}
}

// ShowGlobalBlockersMsg requests opening the global blockers view.
type ShowGlobalBlockersMsg struct{}

// CloseGlobalBlockersMsg requests closing the global blockers view.
type CloseGlobalBlockersMsg struct{}

func (m *GlobalBlockersModel) StatusBarLabel() string { return "Global Blockers" }
