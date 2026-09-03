package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
)

// projectBulkTagUpdate is one project's replacement tag set.
type projectBulkTagUpdate struct {
	project *models.Project
	updated *models.Project
}

// projectBulkTagEdit is a resolved bulk request: the projects that actually
// change, plus the ones whose change crosses the parking boundary. Resolving
// the whole edit before anything commits is what lets the prompt name the real
// impact and lets a no-op request say so instead of writing an empty
// transaction.
type projectBulkTagEdit struct {
	tag         string
	replacement string
	action      ProjectBulkTagAction
	updates     []projectBulkTagUpdate
	parking     []*models.Project
	unparking   []*models.Project
}

// requestProjectBulkTags applies one tag edit across a project selection,
// prompting first only when the edit newly parks something. The prompt is
// decided by the resolved effect rather than by which action was chosen, so
// typing the reserved tag into Add tag confirms exactly like Park does.
func (m *AppModel) requestProjectBulkTags(msg ProjectBulkTagsMsg) tea.Cmd {
	edit, err := m.planProjectBulkTags(msg)
	if err != nil {
		return status.ErrorCmd("%v", err)
	}
	if len(edit.updates) == 0 {
		return status.ErrorCmd("%s", projectBulkTagNoChange(edit))
	}
	if len(edit.parking) == 0 {
		return m.commitProjectBulkTags(edit)
	}
	m.pendingProjectBulkTags = &edit
	m.projectParkConfirm.Show(projectParkConfirmAction, fmt.Sprintf(
		"Park %s? %s Active Projects.",
		projectCountPhrase(len(edit.parking)),
		projectBulkParkImpactClause(edit.parking),
	))
	return nil
}

// planProjectBulkTags resolves every named project and computes its replacement
// tag set. Any project that cannot legally take the edit fails the whole
// request, so a bulk action never silently applies to a subset.
func (m *AppModel) planProjectBulkTags(msg ProjectBulkTagsMsg) (projectBulkTagEdit, error) {
	edit := projectBulkTagEdit{action: msg.Action}
	tag, err := projectBulkTagName(msg)
	if err != nil {
		return edit, err
	}
	edit.tag = tag
	replacement, err := projectBulkTagReplacement(msg)
	if err != nil {
		return edit, err
	}
	edit.replacement = replacement

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
		if parks && m.runtime.IsSystemUnfiledProject(project.ID) {
			return edit, fmt.Errorf("cannot park the system Unfiled project")
		}
		updated := *project
		updated.Tags = tags
		edit.updates = append(edit.updates, projectBulkTagUpdate{project: project, updated: &updated})
		switch {
		case parks:
			edit.parking = append(edit.parking, project)
		case !models.ProjectTagsParked(tags) && models.ProjectTagsParked(project.Tags):
			edit.unparking = append(edit.unparking, project)
		}
	}
	return edit, nil
}

// projectBulkTagName resolves the tag one bulk action edits. Park and unpark
// name the reserved tag themselves so no caller has to spell it.
func projectBulkTagName(msg ProjectBulkTagsMsg) (string, error) {
	switch msg.Action {
	case ProjectBulkTagPark, ProjectBulkTagUnpark:
		return models.ParkedProjectTag, nil
	case ProjectBulkTagAdd, ProjectBulkTagRemove, ProjectBulkTagRename:
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
func projectBulkTagReplacement(msg ProjectBulkTagsMsg) (string, error) {
	if msg.Replacement == "" {
		if msg.Action == ProjectBulkTagRename {
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
	action ProjectBulkTagAction,
	tags []string,
	tag string,
	replacement string,
) []string {
	adding := action == ProjectBulkTagAdd || action == ProjectBulkTagPark
	edited := projectTagsEdited(tags, tag, adding)
	carried := len(edited) < len(tags)
	switch action {
	case ProjectBulkTagRename:
		if carried {
			edited = append(edited, replacement)
		}
	case ProjectBulkTagRemove:
		// A member whose only tag was the removed one would keep no tags at all,
		// so the destination the prompt collected takes its place.
		if carried && replacement != "" && len(edited) == 0 {
			edited = append(edited, replacement)
		}
	case ProjectBulkTagAdd, ProjectBulkTagPark, ProjectBulkTagUnpark:
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
func (m *AppModel) commitProjectBulkTags(edit projectBulkTagEdit) tea.Cmd {
	projectIDs := make([]string, 0, len(edit.updates))
	for _, update := range edit.updates {
		projectIDs = append(projectIDs, update.updated.ID)
	}
	action := projectBulkTagUndoAction(edit)
	transactionID, err := m.commitCatalogProjectMutation(action, projectCountPhrase(len(edit.updates)), projectIDs, func(catalog *config.WorkspaceCatalog) error {
		for _, update := range edit.updates {
			if err := catalog.UpsertProject(update.updated); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		m.err = err
		return nil
	}
	if len(edit.parking) > 0 {
		parked := make([]string, 0, len(edit.parking))
		for _, project := range edit.parking {
			parked = append(parked, project.ID)
		}
		m.anchorOutlinerCursorAfterProjectPark(parked...)
	}
	for _, update := range edit.updates {
		copyCatalogMetadata(update.project, update.updated)
	}
	m.rebuildPlanningFromRuntime()
	feedbackCmd := m.setRecoveryFeedback(projectBulkTagStatus(edit), transactionID, false)
	switch {
	case len(edit.parking) > 0:
		m.appendRecoveryFeedbackDetail(projectBulkTaskCountPhrase(edit.parking) + " hidden · Fp to show")
	case edit.action == ProjectBulkTagRename:
		m.appendRecoveryFeedbackDetail("#" + edit.tag + " → #" + edit.replacement)
	case edit.action == ProjectBulkTagRemove:
		m.appendRecoveryFeedbackDetail("#" + edit.tag)
	default:
		m.appendRecoveryFeedbackDetail("#" + edit.tag)
	}
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
	)
}

func projectBulkTagUndoAction(edit projectBulkTagEdit) string {
	switch {
	case len(edit.parking) > 0:
		return "Park Projects"
	case len(edit.unparking) > 0:
		return "Unpark Projects"
	case edit.action == ProjectBulkTagRename:
		return "Rename Project Tag"
	case edit.action == ProjectBulkTagRemove:
		return "Remove Project Tag"
	default:
		return "Add Project Tag"
	}
}

// projectBulkTagStatus names the effect that happened rather than the action
// that was chosen, and carries the recovery routes whenever rows left Active
// Projects.
func projectBulkTagStatus(edit projectBulkTagEdit) string {
	changed := projectCountPhrase(len(edit.updates))
	switch {
	case len(edit.parking) > 0:
		return fmt.Sprintf(
			"Parked %s · %s hidden · Fp to show",
			projectCountPhrase(len(edit.parking)),
			projectBulkTaskCountPhrase(edit.parking),
		)
	case len(edit.unparking) > 0:
		return fmt.Sprintf(
			"Unparked %s · back in Active Projects",
			projectCountPhrase(len(edit.unparking)),
		)
	case edit.action == ProjectBulkTagRename:
		return fmt.Sprintf(
			"Renamed #%s to #%s on %s",
			edit.tag,
			edit.replacement,
			changed,
		)
	case edit.action == ProjectBulkTagRemove:
		return fmt.Sprintf("Removed #%s from %s", edit.tag, changed)
	default:
		return fmt.Sprintf("Tagged %s #%s", changed, edit.tag)
	}
}

// projectBulkTagNoChange explains a request that would write nothing. Every
// selected project already agreed with the edit, which is a different outcome
// from a failure and reads as one.
func projectBulkTagNoChange(edit projectBulkTagEdit) string {
	switch edit.action {
	case ProjectBulkTagPark:
		return "every selected project is already parked"
	case ProjectBulkTagUnpark:
		return "no selected project is parked"
	case ProjectBulkTagRemove, ProjectBulkTagRename:
		return fmt.Sprintf("no selected project carries #%s", edit.tag)
	default:
		return fmt.Sprintf("every selected project already carries #%s", edit.tag)
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

func projectCountPhrase(count int) string {
	if count == 1 {
		return "1 project"
	}
	return fmt.Sprintf("%d projects", count)
}
