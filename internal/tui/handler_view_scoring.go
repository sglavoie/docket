package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// handleScoringMessages handles scoring-related view messages.
// Returns true if the message was handled.
//
//nolint:gocyclo // routes a flat list of scoring view messages; splitting would obscure the dispatch
func (m *AppModel) handleScoringMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowScoringProfilePickerMsg:
		auto := m.runtime.Automation()
		picker := NewScoringProfilePickerModel(auto.GetProjectScoringProfile(msg.ProjectID), auto.ScoringProfiles, m.configManager, msg.ProjectID)
		m.showView(ViewScoringProfilePicker, picker)
		return true, nil

	case ScoringProfilePickerDoneMsg:
		m.popView()
		if !msg.Canceled {
			if err := m.commitAutomationMutation(func(a *config.AutomationConfig) {
				if msg.ProfileID == "" {
					a.UnsetProjectScoringProfile(msg.ProjectID)
				} else {
					a.SetProjectScoringProfile(msg.ProjectID, msg.ProfileID)
				}
			}); err != nil {
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
			editor := NewScoreEditorModel(msg.Task, msg.Project, m.runtime.Automation().GetProjectScoringProfile(msg.Project.ID), m.secondaryFeatures.TaskItem)
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
		m.showView(ViewScoreProfileManager, NewScoreProfileManagerModel(m.runtime, projectID))
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
			mgr := NewScoreProfileManagerModel(m.runtime, projectID)
			mgr.SetSize(m.width, m.height-2)
			m.views.Register(ViewScoreProfileManager, mgr)
		}
		return true, nil

	case ScoreEditorDoneMsg:
		popCmd := m.popView()
		if msg.Saved {
			// The dimensions committed through the application layer against a
			// fresh snapshot, so reload the owning project's in-memory copy before
			// rebuilding the score-bearing views.
			if msg.Project != nil {
				m.reloadProjectByID(msg.Project.ID)
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
