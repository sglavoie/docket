package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

const taskComposerOverlayKey = "task-composer"

func (m *AppModel) openTaskComposer(preset TaskComposerPreset) tea.Cmd {
	composer := NewTaskComposerModel(preset, m.runtime)
	composer.SetCaptureCoordinator(m.features.Capture)
	composer.SetTaskFormCoordinator(m.features.TaskForm)
	composer.SetSize(m.width, m.height-2)
	m.taskComposer = composer
	m.views.Register(ViewTaskComposer, composer)
	m.pushTypedOverlay(OverlayComposer, taskComposerOverlayKey)
	m.setAccentOnView(composer)
	return composer.Init()
}

func (m *AppModel) activeTaskComposer() bool {
	if m.taskComposer == nil {
		return false
	}
	overlay, ok := m.navigation.Overlays.Current()
	if !ok || overlay.Kind() != OverlayComposer || overlay.Key() != taskComposerOverlayKey {
		return false
	}
	_, hasPresentation := overlay.Presentation()
	return !hasPresentation
}

func (m *AppModel) captureComposerPreset(
	project *models.Project,
	allProjects []*models.Project,
	suggestedTag string,
) (TaskComposerPreset, bool) {
	contextual := project != nil
	projects := m.availableProjects()
	if len(projects) == 0 {
		projects = append([]*models.Project(nil), allProjects...)
	}

	if project == nil {
		project = m.projectByID(m.runtime.Catalog().SystemUnfiledProjectID)
	}
	if project == nil {
		return TaskComposerPreset{}, false
	}
	parts := make([]string, 0, 2)
	if suggestedTag != "" {
		parts = append(parts, "#"+suggestedTag)
	}
	kind := ComposerProject
	if !contextual {
		kind = ComposerGlobal
	}
	return TaskComposerPreset{
		Kind:               kind,
		Project:            project,
		AllProjects:        projects,
		InitialText:        strings.Join(parts, " "),
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, true
}

func (m *AppModel) captureComposerMessage(project *models.Project, allProjects []*models.Project) tea.Msg {
	preset, ok := m.captureComposerPreset(project, allProjects, "")
	if !ok {
		return nil
	}
	return ShowTaskComposerMsg{Preset: preset}
}

func (m *AppModel) createComposerPreset(project *models.Project, parent *models.Task) TaskComposerPreset {
	preset := TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: m.availableProjects(),
		Shared:      TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}
	if parent != nil {
		parentID := parent.ID
		preset.Kind = ComposerChild
		preset.Shared.ParentID = &parentID
		preset.Shared.Destination = app.CaptureDestinationBacklog
	}
	return preset
}
