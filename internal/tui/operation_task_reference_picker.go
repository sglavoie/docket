package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// taskReferenceRowID is the picker's stable row identity: the durable
// `project-id:task-id` locator behind a prefix. A reference the task already
// stores and the candidate row offering that same target therefore collapse to
// one row, which is what lets the checked set be a diff rather than two lists.
func taskReferenceRowID(locator app.TaskLocator) ListItemID {
	return ListItemID("reference:" + locator.Reference().Locator())
}

// storedTaskReferenceIDs is the preselection: exactly the references the task
// stores, in stored order. Preselecting is legal here and not on the `<child`
// picker because of a grammar fact rather than taste — `~` declares both `~0`
// and `-~task`, so unchecking a row means removal, while FieldChild declares no
// Clears and unchecking a child could mean nothing at all.
func storedTaskReferenceIDs(task *models.Task) []ListItemID {
	if task == nil {
		return nil
	}
	ids := make([]ListItemID, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		ids = append(ids, taskReferenceRowID(app.LocatorForReference(reference)))
	}
	return ids
}

// operationTaskReferenceListSource is the population `~` opens over. It is two
// halves and the first is never filtered: every stored reference gets an enabled
// row, terminal and unreadable targets included, because a set editable only by
// hand is how a stored reference gets silently stranded. Leaving one checked
// round-trips as a no-op through the application layer's known-addition rule, so
// admitting those rows costs nothing.
//
// The second half is every open task in every loaded non-archived project, minus
// the source and minus the first half — exactly the population an addition is
// allowed to name, which is why the picker needs no inline add-by-locator mode.
//
// Rows group into one category per project, so a filtered list stays
// project-qualified without the label having to repeat the project name.
func operationTaskReferenceListSource(task *models.Task, projects []*models.Project) ListSource {
	if task == nil {
		return ListSource{}
	}
	stored := make(map[app.TaskLocator]bool, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		stored[app.LocatorForReference(reference)] = true
	}
	candidates := dependencyRemovalCandidates(task, projects, nil)
	candidates = append(candidates, dependencyAdditionCandidates(projects, task.ID, stored)...)

	items := make([]ListItem, 0, len(candidates))
	seen := make(map[ListItemID]bool, len(candidates))
	grouped := make(map[ListCategoryID]bool, len(projects))
	for _, candidate := range candidates {
		id := taskReferenceRowID(candidate.locator)
		if seen[id] {
			continue
		}
		seen[id] = true
		item := ListItem{
			ID:      id,
			Aliases: []string{dependencyCandidateSearchText(candidate)},
			Data:    candidate.locator.Reference().Locator(),
		}
		if candidate.task == nil {
			// A reference this view could not read keeps its stored locator and says
			// only that it is unavailable, the same word Peek uses. It stays enabled
			// like every other stored row: nothing can be said about the target, but
			// the reference itself is still the user's to drop.
			item.PrimaryText = candidate.locator.Reference().Locator()
			item.SecondaryText = dependencyDetailUnavailable
			items = append(items, item)
			continue
		}
		item.PrimaryText = candidate.task.Title
		item.SecondaryText = fmt.Sprintf("%s #%d", candidate.task.State().Icon(), candidate.task.ShortID)
		if candidate.project != nil {
			item.CategoryID = ListCategoryID(candidate.project.ID)
			grouped[item.CategoryID] = true
		}
		items = append(items, item)
	}

	categories := make([]ListCategory, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		id := ListCategoryID(project.ID)
		if !grouped[id] {
			continue
		}
		grouped[id] = false
		categories = append(categories, ListCategory{ID: id, Label: project.Name})
	}
	return ListSource{Items: items, Categories: categories}
}

// taskReferencePatch diffs the checked locators against what the task stores
// right now. The stored side is read fresh at execute time rather than from a
// selection captured when the picker opened, so a reference the picker never
// knew about is still measured and never silently dropped.
//
// Clear stays false even when nothing is checked: the application layer refuses
// Clear beside Add or Remove, and removing every stored reference says exactly
// the same thing. Add and Remove cannot overlap by construction, so the
// "cannot add and remove" refusal is unreachable from this surface.
func taskReferencePatch(task *models.Task, chosen []string) app.DependencyPatch {
	if task == nil {
		return app.DependencyPatch{}
	}
	stored := make(map[string]bool, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		stored[reference.Locator()] = true
	}

	patch := app.DependencyPatch{}
	checked := make(map[string]bool, len(chosen))
	for _, value := range chosen {
		locator, ok := taskReferenceLocator(value)
		if !ok {
			continue
		}
		canonical := locator.Reference().Locator()
		if checked[canonical] {
			continue
		}
		checked[canonical] = true
		if stored[canonical] {
			continue
		}
		patch.Add = append(patch.Add, locator)
	}
	for _, reference := range task.Dependencies {
		if checked[reference.Locator()] {
			continue
		}
		patch.Remove = append(patch.Remove, app.LocatorForReference(reference))
	}
	return patch
}

// taskReferenceLocator splits one canonical row payload. Rows carry the durable
// locator rather than a display alias, so resolving one asks nothing about an
// effective project and cannot pick up a slug the user never typed.
func taskReferenceLocator(value string) (app.TaskLocator, bool) {
	projectID, taskID, qualified := strings.Cut(value, models.TaskReferenceSeparator)
	projectID, taskID = strings.TrimSpace(projectID), strings.TrimSpace(taskID)
	if !qualified || projectID == "" || taskID == "" {
		return app.TaskLocator{}, false
	}
	return app.TaskLocator{ProjectID: projectID, TaskID: taskID}, true
}
