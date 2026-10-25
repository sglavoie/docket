package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/filterops"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/projectops"
	"github.com/sglavoie/docket/internal/tui/status"
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
	if m.domain.runtime.IsSystemUnfiledProject(project.ID) {
		return status.ErrorCmd("cannot park the system Unfiled project")
	}
	m.project.ArmPark(
		projectops.ParkPending{Project: project, Tags: normalized},
		projectParkPrompt(project.Name, project),
	)
	return nil
}

// requestProjectEditorPark holds a whole project-editor save behind the same
// impact confirmation the tag picker raises. The editor resolved the edit but
// wrote nothing, so a canceled prompt leaves every field of the project as it
// was rather than committing the rename and description alone.
func (m *AppModel) requestProjectEditorPark(save *projecthub.ProjectEditorSave) tea.Cmd {
	if save == nil || save.Project == nil {
		return nil
	}
	if m.domain.runtime.IsSystemUnfiledProject(save.Project.ID) {
		return status.ErrorCmd("cannot park the system Unfiled project")
	}
	m.project.ArmPark(
		projectops.ParkPending{EditorSave: save},
		projectParkPrompt(save.Name, save.Project),
	)
	return nil
}

// projectParkPrompt is the one park prompt every route shows, so the sentence
// cannot drift between the picker, the bulk actions, and the editor. name is
// passed separately because an editor save may be renaming the project it parks.
func projectParkPrompt(name string, project *models.Project) string {
	return fmt.Sprintf("Park %s? %s Active Projects.", name, projectParkImpactClause(project))
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
		m.feedback.err = err
		return nil
	}
	if parking {
		m.anchorOutlinerCursorAfterProjectPark(updated.ID)
	}
	coordinator.CopyCatalogMetadata(project, &updated)
	m.rebuildPlanningFromRuntime()
	feedbackCmd := m.setRecoveryFeedback(projectTagStatus(&updated, parking, unparking), transactionID, false)
	if parking {
		m.feedback.appendDetail(projectTaskCountPhrase(&updated) + " hidden · Fp to show")
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

// requestProjectBulkTags applies one tag edit across a project selection,
// prompting first only when the edit newly parks something. The prompt is
// decided by the resolved effect rather than by which action was chosen, so
// typing the reserved tag into Add tag confirms exactly like Park does.
func (m *AppModel) requestProjectBulkTags(msg projecthub.ProjectBulkTagsMsg) tea.Cmd {
	edit, err := m.planProjectBulkTags(msg)
	if err != nil {
		return status.ErrorCmd("%v", err)
	}
	if len(edit.Updates) == 0 {
		text := projectBulkTagNoChange(edit)
		return func() tea.Msg { return status.InfoStatusMsg(text) }
	}
	if len(edit.Parking) == 0 {
		return m.commitProjectBulkTags(edit)
	}
	m.project.ArmPark(projectops.ParkPending{Bulk: &edit}, fmt.Sprintf(
		"Park %s? %s Active Projects.",
		filterops.ProjectCountPhrase(len(edit.Parking)),
		projectBulkParkImpactClause(edit.Parking),
	))
	return nil
}

// planProjectBulkTags resolves every named project and computes its replacement
// tag set. Any project that cannot legally take the edit fails the whole
// request, so a bulk action never silently applies to a subset.
func (m *AppModel) planProjectBulkTags(msg projecthub.ProjectBulkTagsMsg) (projectops.BulkTagEdit, error) {
	edit := projectops.BulkTagEdit{Action: msg.Action}
	tag, err := projectBulkTagName(msg)
	if err != nil {
		return edit, err
	}
	edit.Tag = tag
	replacement, err := projectBulkTagReplacement(msg)
	if err != nil {
		return edit, err
	}
	edit.Replacement = replacement

	for _, projectID := range msg.ProjectIDs {
		project := m.projectByID(projectID)
		if project == nil {
			return edit, fmt.Errorf("project %s is no longer available", projectID)
		}
		if project.Archived {
			return edit, fmt.Errorf("project tags are unavailable while %s is archived", project.Name)
		}
		tags, err := models.NormalizeProjectTags(
			projectBulkTagReplacementSet(msg.Action, project.Tags, tag, replacement),
		)
		if err != nil {
			return edit, err
		}
		if slices.Equal(tags, project.Tags) {
			continue
		}
		parks := models.ProjectTagsParked(tags) && !models.ProjectTagsParked(project.Tags)
		if parks && m.domain.runtime.IsSystemUnfiledProject(project.ID) {
			return edit, fmt.Errorf("cannot park the system Unfiled project")
		}
		updated := *project
		updated.Tags = tags
		edit.Updates = append(edit.Updates, projectops.BulkTagUpdate{Project: project, Updated: &updated})
		switch {
		case parks:
			edit.Parking = append(edit.Parking, project)
		case !models.ProjectTagsParked(tags) && models.ProjectTagsParked(project.Tags):
			edit.Unparking = append(edit.Unparking, project)
		}
	}
	return edit, nil
}

// projectBulkTagName resolves the tag one bulk action edits. Park and unpark
// name the reserved tag themselves so no caller has to spell it.
func projectBulkTagName(msg projecthub.ProjectBulkTagsMsg) (string, error) {
	switch msg.Action {
	case projecthub.ProjectBulkTagPark, projecthub.ProjectBulkTagUnpark:
		return models.ParkedProjectTag, nil
	case projecthub.ProjectBulkTagAdd, projecthub.ProjectBulkTagRemove, projecthub.ProjectBulkTagRename:
		tag, err := models.NormalizeTaskTag(msg.Tag)
		if err != nil {
			return "", err
		}
		return strings.ToLower(tag), nil
	default:
		return "", fmt.Errorf("unknown bulk project tag action %q", msg.Action)
	}
}

// projectBulkTagReplacement validates the second tag an edit writes. A rename
// needs one; a removal destination is optional because leaving the stranded
// members untagged is a legitimate answer.
func projectBulkTagReplacement(msg projecthub.ProjectBulkTagsMsg) (string, error) {
	if msg.Replacement == "" {
		if msg.Action == projecthub.ProjectBulkTagRename {
			return "", fmt.Errorf("a rename needs a new tag name")
		}
		return "", nil
	}
	replacement, err := models.NormalizeTaskTag(msg.Replacement)
	if err != nil {
		return "", err
	}
	return strings.ToLower(replacement), nil
}

// projectBulkTagReplacementSet returns one project's replacement tag set.
func projectBulkTagReplacementSet(
	action projecthub.ProjectBulkTagAction,
	tags []string,
	tag string,
	replacement string,
) []string {
	adding := action == projecthub.ProjectBulkTagAdd || action == projecthub.ProjectBulkTagPark
	edited := projectTagsEdited(tags, tag, adding)
	carried := len(edited) < len(tags)
	switch action {
	case projecthub.ProjectBulkTagRename:
		if carried {
			edited = append(edited, replacement)
		}
	case projecthub.ProjectBulkTagRemove:
		// A member whose only tag was the removed one would keep no tags at all,
		// so the destination the prompt collected takes its place.
		if carried && replacement != "" && len(edited) == 0 {
			edited = append(edited, replacement)
		}
	case projecthub.ProjectBulkTagAdd, projecthub.ProjectBulkTagPark, projecthub.ProjectBulkTagUnpark:
	}
	return edited
}

func projectTagsEdited(tags []string, tag string, adding bool) []string {
	edited := make([]string, 0, len(tags)+1)
	for _, existing := range tags {
		if strings.EqualFold(existing, tag) {
			continue
		}
		edited = append(edited, existing)
	}
	if adding {
		edited = append(edited, tag)
	}
	return edited
}

// commitProjectBulkTags writes every replacement tag set in one catalog
// transaction, so the whole bulk edit is a single workspace undo entry rather
// than one entry per project.
func (m *AppModel) commitProjectBulkTags(edit projectops.BulkTagEdit) tea.Cmd {
	projectIDs := make([]string, 0, len(edit.Updates))
	for _, update := range edit.Updates {
		projectIDs = append(projectIDs, update.Updated.ID)
	}
	action := projectBulkTagUndoAction(edit)
	transactionID, err := m.commitCatalogProjectMutation(action, filterops.ProjectCountPhrase(len(edit.Updates)), projectIDs, func(catalog *config.WorkspaceCatalog) error {
		for _, update := range edit.Updates {
			if err := catalog.UpsertProject(update.Updated); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		m.feedback.err = err
		return nil
	}
	if len(edit.Parking) > 0 {
		parked := make([]string, 0, len(edit.Parking))
		for _, project := range edit.Parking {
			parked = append(parked, project.ID)
		}
		m.anchorOutlinerCursorAfterProjectPark(parked...)
	}
	for _, update := range edit.Updates {
		coordinator.CopyCatalogMetadata(update.Project, update.Updated)
	}
	m.rebuildPlanningFromRuntime()
	feedbackCmd := m.setRecoveryFeedback(projectBulkTagStatus(edit), transactionID, false)
	switch {
	case len(edit.Parking) > 0:
		m.feedback.appendDetail(projectBulkTaskCountPhrase(edit.Parking) + " hidden · Fp to show")
	case edit.Action == projecthub.ProjectBulkTagRename:
		m.feedback.appendDetail("#" + edit.Tag + " → #" + edit.Replacement)
	case edit.Action == projecthub.ProjectBulkTagRemove:
		m.feedback.appendDetail("#" + edit.Tag)
	default:
		m.feedback.appendDetail("#" + edit.Tag)
	}
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
	)
}

func projectBulkTagUndoAction(edit projectops.BulkTagEdit) string {
	switch {
	case len(edit.Parking) > 0:
		return "Park Projects"
	case len(edit.Unparking) > 0:
		return "Unpark Projects"
	case edit.Action == projecthub.ProjectBulkTagRename:
		return "Rename Project Tag"
	case edit.Action == projecthub.ProjectBulkTagRemove:
		return "Remove Project Tag"
	default:
		return "Add Project Tag"
	}
}

// projectBulkTagStatus names the effect that happened rather than the action
// that was chosen, and carries the recovery routes whenever rows left Active
// Projects.
func projectBulkTagStatus(edit projectops.BulkTagEdit) string {
	changed := filterops.ProjectCountPhrase(len(edit.Updates))
	switch {
	case len(edit.Parking) > 0:
		return fmt.Sprintf(
			"Parked %s · %s hidden · Fp to show",
			filterops.ProjectCountPhrase(len(edit.Parking)),
			projectBulkTaskCountPhrase(edit.Parking),
		)
	case len(edit.Unparking) > 0:
		return fmt.Sprintf(
			"Unparked %s · back in Active Projects",
			filterops.ProjectCountPhrase(len(edit.Unparking)),
		)
	case edit.Action == projecthub.ProjectBulkTagRename:
		return fmt.Sprintf(
			"Renamed #%s to #%s on %s",
			edit.Tag,
			edit.Replacement,
			changed,
		)
	case edit.Action == projecthub.ProjectBulkTagRemove:
		return fmt.Sprintf("Removed #%s from %s", edit.Tag, changed)
	default:
		return fmt.Sprintf("Tagged %s #%s", changed, edit.Tag)
	}
}

// projectBulkTagNoChange explains a request that would write nothing. Every
// selected project already agreed with the edit, which is a different outcome
// from a failure and reads as one.
func projectBulkTagNoChange(edit projectops.BulkTagEdit) string {
	switch edit.Action {
	case projecthub.ProjectBulkTagPark:
		return "Every selected project is already parked"
	case projecthub.ProjectBulkTagUnpark:
		return "No selected project is parked"
	case projecthub.ProjectBulkTagRemove, projecthub.ProjectBulkTagRename:
		return fmt.Sprintf("No selected project carries #%s", edit.Tag)
	default:
		return fmt.Sprintf("Every selected project already carries #%s", edit.Tag)
	}
}

func projectBulkParkImpactClause(projects []*models.Project) string {
	if len(projects) == 1 {
		return projectParkImpactClause(projects[0])
	}
	return fmt.Sprintf("Their %s leave", projectBulkTaskCountPhrase(projects))
}

// projectBulkTaskCountPhrase sums the tasks the shell actually read. An unloaded
// project contributes no number, so the phrase degrades rather than claiming a
// count that excludes it.
func projectBulkTaskCountPhrase(projects []*models.Project) string {
	total := 0
	for _, project := range projects {
		if !project.IsLoaded() {
			return "their tasks"
		}
		total += project.CountTasks()
	}
	if total == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", total)
}
