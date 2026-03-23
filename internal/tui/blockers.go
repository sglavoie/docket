package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// BlockersModel displays and manages blockers for a project.
type BlockersModel struct {
	project  *models.Project
	blockers []*models.Blocker
	state    blockerViewState
}

// NewBlockersModelWithEditor creates a blockers viewer with editor config.
func NewBlockersModelWithEditor(project *models.Project, editor string) *BlockersModel {
	blockers, _ := storage.LoadBlockers(project)
	return &BlockersModel{
		project:  project,
		blockers: blockers,
		state:    newBlockerViewState(editor),
	}
}

func (m *BlockersModel) GetConfirmDialog() *ConfirmDialog { return m.state.GetConfirmDialog() }

func (m *BlockersModel) Init() tea.Cmd { return nil }

func (m *BlockersModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewBlockers)
}

func (m *BlockersModel) SetSize(width, height int) {
	m.state.setSize(width, height)
}

func (m *BlockersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd, handled := blockerUpdateShared(&m.state, m, msg)
	if handled {
		return m, cmd
	}
	return m, nil
}

func (m *BlockersModel) View() tea.View {
	return tea.NewView(blockerViewShared(&m.state, m))
}

func (m *BlockersModel) HasTextInput() bool {
	return m.state.hasTextInput()
}

func (m *BlockersModel) ShouldCaptureEsc() bool {
	return m.state.shouldCaptureEsc()
}

// blockerDataSource implementation for project-local blockers.

func (m *BlockersModel) getVisibleItems(tab int) []blockerItem {
	var source []*models.Blocker
	switch tab {
	case blockersTabResolved:
		for _, b := range m.blockers {
			if b.IsResolved() {
				source = append(source, b)
			}
		}
	case blockersTabAll:
		source = append(source, m.blockers...)
	default:
		for _, b := range m.blockers {
			if !b.IsResolved() {
				source = append(source, b)
			}
		}
	}
	items := make([]blockerItem, len(source))
	for i, b := range source {
		items[i] = blockerItem{Blocker: b, Project: m.project}
	}
	return items
}

func (m *BlockersModel) activeCount() int {
	count := 0
	for _, b := range m.blockers {
		if !b.IsResolved() {
			count++
		}
	}
	return count
}

func (m *BlockersModel) resolvedCount() int {
	count := 0
	for _, b := range m.blockers {
		if b.IsResolved() {
			count++
		}
	}
	return count
}

func (m *BlockersModel) findItemIndex(blocker *models.Blocker) int {
	for i, b := range m.blockers {
		if b.ID == blocker.ID {
			return i
		}
	}
	return -1
}

func (m *BlockersModel) saveBlockerData(_ *models.Blocker, _ *models.Project) error {
	return storage.SaveBlockers(m.project, m.blockers)
}

func (m *BlockersModel) deleteBlockerData(idx int) (*models.Project, error) {
	m.blockers = append(m.blockers[:idx], m.blockers[idx+1:]...)
	return m.project, storage.SaveBlockers(m.project, m.blockers)
}

func (m *BlockersModel) updateBlockerData(_ int) (*models.Project, error) {
	return m.project, storage.SaveBlockers(m.project, m.blockers)
}

func (m *BlockersModel) getAddProject() *models.Project {
	return m.project
}

func (m *BlockersModel) getItemBlocker(idx int) *models.Blocker {
	return m.blockers[idx]
}

func (m *BlockersModel) addBlocker(blocker *models.Blocker, _ *models.Project) {
	m.blockers = append(m.blockers, blocker)
}

func (m *BlockersModel) titleText() string {
	return "Blockers for " + m.project.Name
}

func (m *BlockersModel) showProjectName() bool {
	return false
}

func (m *BlockersModel) closeMsg() tea.Msg {
	return CloseBlockersMsg{}
}

// Blocker-related messages

// ShowBlockersMsg requests opening the blockers view for a project.
type ShowBlockersMsg struct {
	Project *models.Project
}

// CloseBlockersMsg requests closing the blockers view.
type CloseBlockersMsg struct{}

// BlockerAddedMsg signals a blocker was added.
type BlockerAddedMsg struct {
	Project *models.Project
}

// BlockerEditedMsg signals a blocker was edited.
type BlockerEditedMsg struct {
	Project *models.Project
}

// BlockerDeletedMsg signals a blocker was deleted.
type BlockerDeletedMsg struct {
	Project *models.Project
}

// BlockerGoToProjectMsg navigates to a project from the blockers view.
type BlockerGoToProjectMsg struct {
	Project *models.Project
}

func (m *BlockersModel) StatusBarLabel() string { return "Blockers" }
