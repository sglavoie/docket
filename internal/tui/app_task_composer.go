package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

const taskComposerOverlayKey = "task-composer"

func (m *AppModel) openTaskComposer(preset TaskComposerPreset, presentation PresentationID) tea.Cmd {
	composer := NewTaskComposerModel(preset, m.runtime)
	composer.presentation = presentation
	composer.SetCaptureCoordinator(m.features.Capture)
	composer.SetTaskFormCoordinator(m.features.TaskForm)
	composer.SetSize(m.width, m.height-2)
	m.taskComposer = composer
	// Register the renderer identity used by nested form pickers while the typed
	// composer overlay remains the authoritative navigation state.
	m.views.Register(presentation, composer)
	m.pushTypedOverlay(OverlayComposer, taskComposerOverlayKey)
	m.setAccentOnView(composer)
	return composer.Init()
}

func (m *AppModel) activeTaskComposer() bool {
	if !m.typedNavigation || m.taskComposer == nil {
		return false
	}
	overlay, ok := m.navigation.Overlays.Current()
	if !ok || overlay.Kind() != OverlayComposer || overlay.Key() != taskComposerOverlayKey {
		return false
	}
	_, hasPresentation := overlay.Presentation()
	return !hasPresentation
}

func (m *AppModel) captureComposerPreset(msg ShowQuickCaptureMsg) (TaskComposerPreset, bool) {
	project := msg.Project
	allProjects := m.activeProjects()
	if len(allProjects) == 0 {
		allProjects = msg.AllProjects
	}

	if project == nil {
		project = m.projectByID(m.runtime.Catalog().SystemInboxProjectID)
	}
	if project == nil {
		return TaskComposerPreset{}, false
	}
	parts := make([]string, 0, 2)
	if msg.SuggestedTag != "" {
		parts = append(parts, "#"+msg.SuggestedTag)
	}
	if msg.SuggestedSection != "" {
		parts = append(parts, "/"+msg.SuggestedSection)
	}
	kind := ComposerProject
	if msg.Project == nil {
		kind = ComposerGlobal
	}
	return TaskComposerPreset{
		Kind:               kind,
		Project:            project,
		AllProjects:        allProjects,
		InitialText:        strings.Join(parts, " "),
		AllowProjectSwitch: true,
		Shared:             TaskComposerSharedAttributes{Section: models.SectionInbox},
	}, true
}

func (m *AppModel) createComposerPreset(project *models.Project, parent *models.Task) TaskComposerPreset {
	preset := TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: m.activeProjects(),
		Shared:      TaskComposerSharedAttributes{Section: models.SectionNext},
	}
	if parent != nil {
		parentID := parent.ID
		preset.Kind = ComposerChild
		preset.Parent = parent
		preset.Shared.ParentID = &parentID
		preset.Shared.Section = taskSectionBucket(parent)
	}
	return preset
}

func linkedComposerPreset(project *models.Project, source *models.Task, linkType LinkType) TaskComposerPreset {
	section := ""
	if source != nil {
		section = taskSectionBucket(source)
	}
	return TaskComposerPreset{
		Kind:       ComposerLinked,
		Project:    project,
		SourceTask: source,
		LinkType:   linkType,
		Shared:     TaskComposerSharedAttributes{Section: section},
	}
}

func templateComposerPreset(project *models.Project, parent *models.Task, tmpl *template.TaskTemplate) TaskComposerPreset {
	return TaskComposerPreset{Kind: ComposerTemplate, Project: project, Parent: parent, Template: tmpl}
}
