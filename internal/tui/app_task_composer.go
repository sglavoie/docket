package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

const taskComposerOverlayKey = "task-composer"

func (m *AppModel) openTaskComposer(preset taskform.TaskComposerPreset) tea.Cmd {
	composer := taskform.NewTaskComposerModel(preset, m.domain.runtime)
	composer.SetCaptureCoordinator(m.features.Capture)
	composer.SetTaskFormCoordinator(m.features.TaskForm)
	composer.SetSize(m.viewNav.viewSize())
	composer.EnableDraftRecovery()
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
) (taskform.TaskComposerPreset, bool) {
	contextual := project != nil
	projects := m.availableProjects()
	if len(projects) == 0 {
		projects = append([]*models.Project(nil), allProjects...)
	}

	if project == nil {
		project = m.projectByID(m.domain.runtime.Catalog().SystemUnfiledProjectID)
	}
	if project == nil {
		return taskform.TaskComposerPreset{}, false
	}
	kind := taskform.ComposerProject
	if !contextual {
		kind = taskform.ComposerGlobal
	}
	return taskform.TaskComposerPreset{
		Kind:               kind,
		HeaderLabel:        "Capture",
		Project:            project,
		AllProjects:        projects,
		AllowProjectSwitch: true,
		Shared:             taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, true
}

func (m *AppModel) captureComposerMessage(project *models.Project, allProjects []*models.Project) tea.Msg {
	preset, ok := m.captureComposerPreset(project, allProjects)
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

// repeatComposerPreset prepares the draft a repetition starts from and wraps it
// in a composer preset that opens straight into the expanded form. The draft is
// read through the application layer rather than off the row's task pointer, so
// the values are the stored ones at this instant and the preparation refuses
// exactly where the application layer would.
//
// Once this returns, the preset owns everything: the seed holds copied values,
// so a later edit to the source task, or its deletion, cannot reach the open
// draft. Only Project stays a live pointer, because the composer needs the
// destination project itself, and the save re-resolves it against current state
// anyway.
func (m *AppModel) repeatComposerPreset(project *models.Project, task *models.Task) (taskform.TaskComposerPreset, error) {
	draft, err := m.DomainService().PrepareTaskRepeatDraft(app.TaskLocator{ProjectID: project.ID, TaskID: task.ID})
	if err != nil {
		return taskform.TaskComposerPreset{}, err
	}
	return taskform.TaskComposerPreset{
		Kind:        taskform.ComposerProject,
		Project:     project,
		AllProjects: m.availableProjects(),
		Shared:      taskform.TaskComposerSharedAttributes{Destination: draft.Destination},
		Seed: &taskform.TaskComposerSeed{
			HostTitle:      "Repeat Task",
			Title:          draft.Title,
			Description:    draft.Description,
			Priority:       draft.Priority,
			Tags:           draft.Tags,
			Context:        draft.Context,
			ChecklistItems: draft.ChecklistItems,
			Destination:    draft.Destination,
		},
	}, nil
}

// focusChildComposerPreset names the focused task in the composer header, so
// capture inside a Focus is never mistaken for capture into the project root.
func (m *AppModel) focusChildComposerPreset(project *models.Project, parent *models.Task) taskform.TaskComposerPreset {
	preset := m.createComposerPreset(project, parent)
	preset.HeaderLabel = "Capture → child of " + parent.Title
	return preset
}

// parseTaskContent extracts the title and description from edited content.
// The editor template format is:
//
//	# Task Title
//	<!-- Edit the title above and description below. -->
//	Description text here...
//
// The function uses a state machine: it looks for the title heading before
// the instruction comment, then treats everything after the comment as
// description (preserving any HTML comments the user may have in their text).
func parseTaskContent(content string, task *models.Task) (title, description string) {
	lines := strings.Split(content, "\n")
	var descLines []string
	title = task.Title // Default to original title
	inDescription := false
	foundComment := false
	foundTitle := false

	// Collect lines that come after the title but before the comment marker.
	// If no comment marker is found, these become the description (fallback).
	var preCommentLines []string

	for _, line := range lines {
		// Already in description section — keep everything
		if inDescription {
			descLines = append(descLines, line)
			continue
		}

		// Look for title line (first # line, only before the comment marker)
		if !foundComment && !foundTitle && len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			title = strings.TrimSpace(line[2:])
			foundTitle = true
			continue
		}

		// Look for the instruction comment marker (only the first <!-- line)
		if !foundComment && len(line) >= 4 && line[:4] == "<!--" {
			foundComment = true
			continue
		}

		// After comment, skip empty lines until description starts
		if foundComment && line == "" {
			continue
		}

		// Start of description content
		if foundComment {
			inDescription = true
			descLines = append(descLines, line)
			continue
		}

		// Before comment marker: accumulate lines in case comment is missing
		if foundTitle {
			preCommentLines = append(preCommentLines, line)
		}
	}

	// If no comment marker was found, use pre-comment lines as description
	// so that accidentally deleting the marker doesn't silently drop content.
	if !foundComment && len(preCommentLines) > 0 {
		descLines = preCommentLines
	}

	// Trim trailing empty lines
	for len(descLines) > 0 && descLines[len(descLines)-1] == "" {
		descLines = descLines[:len(descLines)-1]
	}

	// Trim leading empty lines
	for len(descLines) > 0 && descLines[0] == "" {
		descLines = descLines[1:]
	}

	// If title was cleared, keep the original
	if title == "" {
		title = task.Title
	}

	return title, strings.Join(descLines, "\n")
}

// clearBulkTaskSelectionsForTaskIDs is the compatibility entry point for async
// results that lack project-qualified locators. It clears identity-keyed state
// only when each matching task ID belongs to one selected project; ambiguous
// duplicate-ID selections are left intact.
func (m *AppModel) clearBulkTaskSelectionsForTaskIDs(ids []string) {
	m.clearBulkTaskSelectionsForTaskIDSet(taskIDSliceToSet(ids))
}

func (m *AppModel) clearBulkTaskSelectionsForLocators(locators []app.TaskLocator) {
	if len(locators) == 0 {
		return
	}
	set := make(map[taskref.TaskIdentity]bool, len(locators))
	for _, locator := range locators {
		identity := taskref.FromLocator(locator)
		if identity.Valid() {
			set[identity] = true
		}
	}
	clearOutliner := func(outlinerView *outliner.OutlinerModel) {
		if outlinerView == nil {
			return
		}
		for identity := range set {
			delete(outlinerView.Selected, identity)
		}
		if outlinerView.SelectedCount() == 0 {
			outlinerView.ExitMultiSelect()
		}
	}
	clearOutliner(m.crossOutliner)
	if m.viewNav.views != nil {
		if view := m.viewNav.views.Get(presentation.ViewCrossProjectOutliner); view != nil {
			if outlinerView, ok := view.(*outliner.OutlinerModel); ok {
				clearOutliner(outlinerView)
			}
		}
	}
}

func (m *AppModel) clearBulkTaskSelectionsForTaskIDSet(selectedIDs map[string]bool) {
	if len(selectedIDs) == 0 {
		return
	}

	if m.crossOutliner != nil && multiSelectStateHasTaskID(&m.crossOutliner.MultiSelectState, selectedIDs) {
		m.crossOutliner.ExitMultiSelect()
	}

	m.clearViewBulkTaskSelectionForRefs(presentation.ViewCrossProjectOutliner, selectedIDs)
}

func (m *AppModel) clearViewBulkTaskSelectionForRefs(vt presentation.PresentationID, selectedIDs map[string]bool) {
	if m.viewNav.views == nil {
		return
	}
	view := m.viewNav.views.Get(vt)
	switch v := view.(type) {
	case *outliner.OutlinerModel:
		if multiSelectStateHasTaskID(&v.MultiSelectState, selectedIDs) {
			v.ExitMultiSelect()
		}
	}
}

func taskIDSliceToSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = true
		}
	}
	return set
}

func multiSelectStateHasTaskID(state *listsurface.MultiSelectState, selectedIDs map[string]bool) bool {
	if state == nil {
		return false
	}
	matchedProjects := make(map[string]string, len(selectedIDs))
	for identity := range state.Selected {
		if !selectedIDs[identity.TaskID] {
			continue
		}
		if projectID, matched := matchedProjects[identity.TaskID]; matched && projectID != identity.ProjectID {
			// An ID-only result cannot identify which duplicate selection committed.
			// Leave the selection intact instead of clearing both projects.
			return false
		}
		matchedProjects[identity.TaskID] = identity.ProjectID
	}
	return len(matchedProjects) > 0
}
