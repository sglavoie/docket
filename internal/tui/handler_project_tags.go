package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
)

// requestProjectTags applies a replacement project tag set, prompting first only
// when the edit newly parks the project. Parking is the one project tag whose
// effect the user cannot see before it happens — rows disappear from the
// Outliner — so it is the one tag that confirms.
func (m *AppModel) requestProjectTags(project *models.Project, tags []string) tea.Cmd {
	if project == nil {
		return nil
	}
	normalized, err := models.NormalizeProjectTags(tags)
	if err != nil {
		return status.ErrorCmd("%v", err)
	}
	if !models.ProjectTagsParked(normalized) || models.ProjectTagsParked(project.Tags) {
		return m.commitProjectTags(project, normalized)
	}
	if m.runtime.IsSystemUnfiledProject(project.ID) {
		return status.ErrorCmd("cannot park the system Unfiled project")
	}
	m.pendingProjectTags = project
	m.pendingProjectTagsSet = normalized
	m.projectParkConfirm.Show(projectParkConfirmAction, projectParkPrompt(project.Name, project))
	return nil
}

// requestProjectEditorPark holds a whole project-editor save behind the same
// impact confirmation the tag picker raises. The editor resolved the edit but
// wrote nothing, so a canceled prompt leaves every field of the project as it
// was rather than committing the rename and description alone.
func (m *AppModel) requestProjectEditorPark(save *projectEditorSave) tea.Cmd {
	if save == nil || save.project == nil {
		return nil
	}
	if m.runtime.IsSystemUnfiledProject(save.project.ID) {
		return status.ErrorCmd("cannot park the system Unfiled project")
	}
	m.pendingProjectEditorSave = save
	m.projectParkConfirm.Show(projectParkConfirmAction, projectParkPrompt(save.name, save.project))
	return nil
}

// projectParkPrompt is the one park prompt every route shows, so the sentence
// cannot drift between the picker, the bulk actions, and the editor. name is
// passed separately because an editor save may be renaming the project it parks.
func projectParkPrompt(name string, project *models.Project) string {
	return fmt.Sprintf("Park %s? %s Active Projects.", name, projectParkImpactClause(project))
}

// clearPendingProjectTags is the single cancel path for the shared park
// confirmation, so it releases whichever pending edit raised the prompt.
func (m *AppModel) clearPendingProjectTags() {
	m.pendingProjectTags = nil
	m.pendingProjectTagsSet = nil
	m.pendingProjectBulkTags = nil
	m.pendingProjectEditorSave = nil
	m.projectParkConfirm.Cancel()
}

// commitProjectTags persists the replacement tag set through the ordinary
// catalog transaction, so the workspace undo ledger covers it. Planning
// memberships are deliberately left alone: a parked project keeps its Today and
// Next references and they are filtered at read time, so unparking restores the
// plan untouched.
func (m *AppModel) commitProjectTags(project *models.Project, tags []string) tea.Cmd {
	if project == nil {
		return nil
	}
	parking := models.ProjectTagsParked(tags) && !models.ProjectTagsParked(project.Tags)
	unparking := !models.ProjectTagsParked(tags) && models.ProjectTagsParked(project.Tags)

	updated := *project
	updated.Tags = tags
	action := projectTagUndoAction(parking, unparking)
	transactionID, err := m.commitCatalogProjectMutation(action, updated.Name, []string{updated.ID}, func(catalog *config.WorkspaceCatalog) error {
		return catalog.UpsertProject(&updated)
	})
	if err != nil {
		m.err = err
		return nil
	}
	if parking {
		m.anchorOutlinerCursorAfterProjectPark(updated.ID)
	}
	copyCatalogMetadata(project, &updated)
	m.rebuildPlanningFromRuntime()
	feedbackCmd := m.setRecoveryFeedback(projectTagStatus(&updated, parking, unparking), transactionID, false)
	if parking {
		m.appendRecoveryFeedbackDetail(projectTaskCountPhrase(&updated) + " hidden · Fp to show")
	}
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
	)
}

func projectTagUndoAction(parking, unparking bool) string {
	switch {
	case parking:
		return "Park Project"
	case unparking:
		return "Unpark Project"
	default:
		return "Set Project Tags"
	}
}

// projectTagStatus names both recovery routes after a park, because the rows the
// user was just looking at are gone and neither route is discoverable from what
// remains on screen.
func projectTagStatus(project *models.Project, parking, unparking bool) string {
	switch {
	case parking:
		return fmt.Sprintf(
			"Parked %s · %s hidden · Fp to show",
			project.Name,
			projectTaskCountPhrase(project),
		)
	case unparking:
		return fmt.Sprintf(
			"Unparked %s · %s back in Active Projects",
			project.Name,
			projectTaskCountPhrase(project),
		)
	default:
		return fmt.Sprintf("Updated %s tags", project.Name)
	}
}

// projectTaskCountPhrase renders "14 tasks" for a loaded project and degrades to
// "its tasks" when the count is unknown, so no sentence ever claims a number the
// shell did not actually read.
func projectTaskCountPhrase(project *models.Project) string {
	if !project.IsLoaded() {
		return "its tasks"
	}
	if count := project.CountTasks(); count != 1 {
		return fmt.Sprintf("%d tasks", count)
	}
	return "1 task"
}

// projectParkImpactClause names what is about to leave the base task workspace.
// It degrades to an unnumbered phrase when the project's tasks were never read,
// so the prompt never claims a count the shell did not actually see.
func projectParkImpactClause(project *models.Project) string {
	if !project.IsLoaded() {
		return "Its tasks leave"
	}
	if count := project.CountTasks(); count != 1 {
		return fmt.Sprintf("Its %d tasks leave", count)
	}
	return "Its 1 task leaves"
}
