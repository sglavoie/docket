package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/storage"
)

// handleScoringMessages handles scoring-related view messages.
// Returns true if the message was handled.
func (m *AppModel) handleScoringMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowScoringProfilePickerMsg:
		picker := NewScoringProfilePickerModel(m.config, m.configManager, msg.ProjectID)
		m.showView(ViewScoringProfilePicker, picker)
		return true, nil

	case ScoringProfilePickerDoneMsg:
		m.popView()
		if !msg.Canceled {
			if msg.ProfileID == "" {
				m.config.UnsetProjectScoringProfile(msg.ProjectID)
			} else {
				m.config.SetProjectScoringProfile(msg.ProjectID, msg.ProfileID)
			}
			_ = m.configManager.Save(m.config)
			// Refresh the preview viewport content if it's visible
			if v := m.views.Get(ViewProjectPreview); v != nil {
				if preview, ok := v.(*ProjectPreviewModel); ok {
					preview.refreshContent()
				}
			}
		}
		return true, nil

	case ShowScoreEditorMsg:
		if msg.Task != nil && msg.Project != nil {
			editor := NewScoreEditorModel(msg.Task, msg.Project, m.config)
			if editor != nil {
				m.showView(ViewScoreEditor, editor)
			} else {
				m.statusMessage = "No scoring profile assigned — opening profile manager"
				return true, func() tea.Msg {
					return ShowScoreProfileManagerMsg{}
				}
			}
		}
		return true, nil

	case ShowScoreProfileManagerMsg:
		projectID := ""
		if m.currentProject != nil {
			projectID = m.currentProject.ID
		}
		m.showView(ViewScoreProfileManager, NewScoreProfileManagerModel(m.config, m.configManager, projectID))
		return true, nil

	case CloseScoreProfileManagerMsg:
		m.popView()
		return true, nil

	case ScoreProfileChangedMsg:
		if m.currentView() == ViewScoreProfileManager {
			projectID := ""
			if m.currentProject != nil {
				projectID = m.currentProject.ID
			}
			mgr := NewScoreProfileManagerModel(m.config, m.configManager, projectID)
			mgr.SetSize(m.width, m.height-2)
			m.views.Register(ViewScoreProfileManager, mgr)
		}
		return true, nil

	case ScoreEditorDoneMsg:
		m.popView()
		if msg.Saved {
			// Refresh the project tree and data after score changes
			if m.projectTree != nil {
				_ = storage.LoadProject(m.projectTree.project)
				m.projectTree.buildTree()
			}
			// Refresh kanban if active
			if m.kanban != nil {
				_ = storage.LoadProject(m.kanban.Project)
				m.kanban.loadTasks()
			}
			// Refresh all-tasks view if open
			if m.allTasksView != nil {
				m.allTasksView.loadAllTasks()
			}
			// Refresh outliner if open
			if m.outliner != nil {
				_ = storage.LoadProject(m.outliner.project)
				m.outliner.rebuildRows()
			}
			m.statusMessage = fmt.Sprintf("Score dimensions updated for task #%d", msg.Task.ShortID)
		}
		return true, nil
	}
	return false, nil
}
