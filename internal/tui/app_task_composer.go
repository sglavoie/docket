package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

const taskComposerOverlayKey = "task-composer"

func (m *AppModel) openTaskComposer(preset taskform.TaskComposerPreset) tea.Cmd {
	composer := taskform.NewTaskComposerModel(preset, m.runtime)
	composer.SetCaptureCoordinator(m.features.Capture)
	composer.SetTaskFormCoordinator(m.features.TaskForm)
	composer.SetSize(m.viewNav.viewSize())
	m.taskComposer = composer
	m.viewNav.views.Register(presentation.ViewTaskComposer, composer)
	m.pushTypedOverlay(route.OverlayComposer, taskComposerOverlayKey)
	m.setAccentOnView(composer)
	return composer.Init()
}

func (m *AppModel) activeTaskComposer() bool {
	if m.taskComposer == nil {
		return false
	}
	overlay, ok := m.viewNav.navigation.Overlays.Current()
	if !ok || overlay.Kind() != route.OverlayComposer || overlay.Key() != taskComposerOverlayKey {
		return false
	}
	_, hasPresentation := overlay.Presentation()
	return !hasPresentation
}

func (m *AppModel) captureComposerPreset(
	project *models.Project,
	allProjects []*models.Project,
	suggestedTag string,
) (taskform.TaskComposerPreset, bool) {
	contextual := project != nil
	projects := m.availableProjects()
	if len(projects) == 0 {
		projects = append([]*models.Project(nil), allProjects...)
	}

	if project == nil {
		project = m.projectByID(m.runtime.Catalog().SystemUnfiledProjectID)
	}
	if project == nil {
		return taskform.TaskComposerPreset{}, false
	}
	parts := make([]string, 0, 2)
	if suggestedTag != "" {
		parts = append(parts, "#"+suggestedTag)
	}
	kind := taskform.ComposerProject
	if !contextual {
		kind = taskform.ComposerGlobal
	}
	return taskform.TaskComposerPreset{
		Kind:               kind,
		HeaderLabel:        "Capture → " + project.Name,
		Project:            project,
		AllProjects:        projects,
		InitialText:        strings.Join(parts, " "),
		AllowProjectSwitch: true,
		Shared:             taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, true
}

func (m *AppModel) captureComposerMessage(project *models.Project, allProjects []*models.Project) tea.Msg {
	preset, ok := m.captureComposerPreset(project, allProjects, "")
	if !ok {
		return nil
	}
	return taskform.ShowTaskComposerMsg{Preset: preset}
}

func (m *AppModel) createComposerPreset(project *models.Project, parent *models.Task) taskform.TaskComposerPreset {
	preset := taskform.TaskComposerPreset{
		Kind:        taskform.ComposerProject,
		Project:     project,
		AllProjects: m.availableProjects(),
		Shared:      taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}
	if parent != nil {
		parentID := parent.ID
		preset.Kind = taskform.ComposerChild
		preset.Shared.ParentID = &parentID
		preset.Shared.Destination = app.CaptureDestinationBacklog
	}
	return preset
}

// focusChildComposerPreset names the focused task in the composer header, so
// capture inside a Focus is never mistaken for capture into the project root.
func (m *AppModel) focusChildComposerPreset(project *models.Project, parent *models.Task) taskform.TaskComposerPreset {
	preset := m.createComposerPreset(project, parent)
	preset.HeaderLabel = "Capture → child of " + parent.Title
	return preset
}
