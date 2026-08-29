package tui

import (
	"fmt"
	"sort"
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

// storedTaskReferenceBaseline is the reference set as it stood the moment the
// picker opened, in stored order. The picker keeps it so its commit can be the
// difference between two sets rather than one absolute set: a row the user never
// touched belongs to neither half of the emitted value and therefore cannot
// contradict a change that landed elsewhere while the overlay was up.
func storedTaskReferenceBaseline(task *models.Task) []app.TaskLocator {
	if task == nil {
		return nil
	}
	baseline := make([]app.TaskLocator, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		baseline = append(baseline, app.LocatorForReference(reference))
	}
	return baseline
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
	baseline := storedTaskReferenceBaseline(task)
	ids := make([]ListItemID, 0, len(baseline))
	for _, locator := range baseline {
		ids = append(ids, taskReferenceRowID(locator))
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

// taskReferenceSelection is what the picker emits on Enter: the rows the user
// actually checked and unchecked, measured against the baseline the picker
// preselected, and never the whole checked set. A full checked set would be an
// assertion about every reference at once, including the ones the user never
// looked at, and the overlay is not modal against the workspace — an accepted
// cross-project load republishes task trees through adoptCrossProjectSnapshot
// while `~` is still open. Emitting a difference is what keeps such a change out
// of the commit instead of silently reverting it.
//
// Add is sorted so the emitted value never depends on the order rows were
// ticked in; Remove stays in stored order, which is the order the picker listed
// the baseline in.
func taskReferenceSelection(baseline []app.TaskLocator, checked []string) OperationTaskReferenceValue {
	initial := make(map[string]bool, len(baseline))
	for _, locator := range baseline {
		initial[locator.Reference().Locator()] = true
	}

	value := OperationTaskReferenceValue{}
	seen := make(map[string]bool, len(checked))
	for _, raw := range checked {
		locator, ok := taskReferenceLocator(raw)
		if !ok {
			continue
		}
		canonical := locator.Reference().Locator()
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		if initial[canonical] {
			continue
		}
		value.Add = append(value.Add, canonical)
	}
	sort.Strings(value.Add)
	for _, locator := range baseline {
		canonical := locator.Reference().Locator()
		if seen[canonical] {
			continue
		}
		value.Remove = append(value.Remove, canonical)
	}
	return value
}

// taskReferencePatch merges the picker's selection into what the task stores at
// execute time. The stored side is read fresh from the workspace rather than
// from the snapshot the picker opened over, and the selection is an intent
// rather than a set, so the two compose instead of fighting: an addition the
// user did not make is never spelled as a removal, and a removal the user did
// not make is never spelled as an addition. A reference that appeared or
// vanished while the overlay was up is therefore left exactly as the fresh state
// has it.
//
// Each half is also idempotent against that fresh state. Checking a row the task
// already stores contributes nothing, and unchecking one it no longer stores
// contributes nothing, so the resolver never has to judge an addition that is
// already present or a removal with no target.
//
// Clear stays false even when nothing is checked: the application layer refuses
// Clear beside Add or Remove, and removing every stored reference says exactly
// the same thing. Add and Remove cannot overlap by construction, so the
// "cannot add and remove" refusal is unreachable from this surface.
func taskReferencePatch(task *models.Task, value OperationTaskReferenceValue) app.DependencyPatch {
	if task == nil {
		return app.DependencyPatch{}
	}
	stored := make(map[string]bool, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		stored[reference.Locator()] = true
	}

	patch := app.DependencyPatch{}
	added := make(map[string]bool, len(value.Add))
	for _, raw := range value.Add {
		locator, ok := taskReferenceLocator(raw)
		if !ok {
			continue
		}
		canonical := locator.Reference().Locator()
		if added[canonical] || stored[canonical] {
			continue
		}
		added[canonical] = true
		patch.Add = append(patch.Add, locator)
	}
	removed := make(map[string]bool, len(value.Remove))
	for _, raw := range value.Remove {
		locator, ok := taskReferenceLocator(raw)
		if !ok {
			continue
		}
		canonical := locator.Reference().Locator()
		if removed[canonical] || !stored[canonical] {
			continue
		}
		removed[canonical] = true
		patch.Remove = append(patch.Remove, locator)
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
