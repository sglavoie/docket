package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
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
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
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
				m.setInfoStatus("No scoring profile assigned — opening profile manager")
				return true, func() tea.Msg {
					return ShowScoreProfileManagerMsg{ProjectID: msg.Project.ID}
				}
			}
		}
		return true, nil

	case ShowScoreProfileManagerMsg:
		projectID := msg.ProjectID
		if projectID == "" && m.currentProject != nil {
			projectID = m.currentProject.ID
		}
		m.showView(ViewScoreProfileManager, NewScoreProfileManagerModel(m.config, m.configManager, projectID))
		return true, nil

	case ScoreProfileChangedMsg:
		if m.currentView() == ViewScoreProfileManager {
			projectID := ""
			if v := m.views.Get(ViewScoreProfileManager); v != nil {
				if existing, ok := v.(*ScoreProfileManagerModel); ok {
					projectID = existing.currentProjectID
				}
			}
			if projectID == "" && m.currentProject != nil {
				projectID = m.currentProject.ID
			}
			mgr := NewScoreProfileManagerModel(m.config, m.configManager, projectID)
			mgr.SetSize(m.width, m.height-2)
			m.views.Register(ViewScoreProfileManager, mgr)
		}
		return true, nil

	case ScoreEditorDoneMsg:
		popCmd := m.popView()
		if msg.Saved {
			if m.projectTree != nil {
				m.projectTree.buildTree()
			}
			if m.allTasksView != nil {
				m.allTasksView.loadAllTasks()
			}
			if m.outliner != nil {
				if m.outliner.project == nil {
					m.outliner.crossForceRebuild = true
				}
				m.outliner.rebuildRows()
			}
			if cross := m.activeCrossProjectOutliner(); cross != nil && cross != m.outliner {
				cross.crossForceRebuild = true
				cross.rebuildRows()
				cross.clampCursor()
			}
			for _, vt := range []ViewType{ViewKanban, ViewCrossProjectKanban} {
				if v := m.views.Get(vt); v != nil {
					if kb, ok := v.(*KanbanModel); ok {
						kb.rebuildCols()
					}
				}
			}
			m.setSuccessStatus(fmt.Sprintf("Score dimensions updated for task #%d", msg.Task.ShortID))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		}
		return true, popCmd
	}
	return false, nil
}
