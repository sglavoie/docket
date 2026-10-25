package valuepicker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// TaskReferenceRowID is the picker's stable row identity: the durable
// `project-id:task-id` locator behind a prefix. A reference the task already
// stores and the candidate row offering that same target therefore collapse to
// one row, which is what lets the checked set be a diff rather than two lists.
func TaskReferenceRowID(locator app.TaskLocator) listsurface.ListItemID {
	return listsurface.ListItemID("reference:" + locator.Reference().Locator())
}

// TaskReferenceRowLocator is the inverse of TaskReferenceRowID. Row identity is
// the durable locator behind a prefix rather than an index into a population, so
// reading a row back asks nothing of the list the row came from.
func TaskReferenceRowLocator(id listsurface.ListItemID) (app.TaskLocator, bool) {
	value, prefixed := strings.CutPrefix(string(id), "reference:")
	if !prefixed {
		return app.TaskLocator{}, false
	}
	return TaskReferenceLocator(value)
}

// storedTaskReferenceBaseline is the reference set one task stored the moment
// the picker opened, in stored order.
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

// TaskReferenceMembership is the preselection: All for a reference every
// selected task stores, Some for one only part of the selection stores, and no
// entry at all for a row nobody stores — which the picker fills in as None.
// Preselecting is legal here and not on the `<child` picker because of a grammar
// fact rather than taste: `~` declares both `~0` and `-~task`, so unchecking a
// row means removal, while FieldChild declares no Clears and unchecking a child
// could mean nothing at all.
//
// Three states rather than two is what lets one picker edit N tasks carrying N
// different stored sets. The documented objection to bulk here — that N tasks
// carry N different baselines — is true of a replacement picker and false of
// this one: an untouched row stays Some and is emitted by neither half, so the
// tasks that disagree about it keep disagreeing.
func TaskReferenceMembership(tasks []*models.Task) map[listsurface.ListItemID]OperationValueMembership {
	counts := make(map[listsurface.ListItemID]int)
	selected := 0
	for _, task := range tasks {
		if task == nil {
			continue
		}
		selected++
		for _, locator := range storedTaskReferenceBaseline(task) {
			counts[TaskReferenceRowID(locator)]++
		}
	}
	membership := make(map[listsurface.ListItemID]OperationValueMembership, len(counts))
	for id, count := range counts {
		if count >= selected {
			membership[id] = OperationValueAll
			continue
		}
		membership[id] = OperationValueSome
	}
	return membership
}

// OperationTaskReferenceListSource is the population `~` opens over. It is two
// halves and the first is never filtered: every stored reference gets an enabled
// row, terminal and unreadable targets included, because a set editable only by
// hand is how a stored reference gets silently stranded. Leaving one checked
// round-trips as a no-op through the application layer's known-addition rule, so
// admitting those rows costs nothing.
//
// The second half is every open task in every loaded non-archived project, minus
// the selection and minus the first half — exactly the population an addition is
// allowed to name, which is why the picker needs no inline add-by-locator mode.
// Over a selection the two halves are the union across it: the stored half lists
// every reference anybody carries, in the order the selection carries them, and
// the addition half withholds every selected task, since naming one is a
// relationship inside the selection the application layer refuses by name.
//
// Rows group into one category per project, so a filtered list stays
// project-qualified without the label having to repeat the project name.
func OperationTaskReferenceListSource(tasks []*models.Task, projects []*models.Project) listsurface.ListSource {
	stored := make(map[app.TaskLocator]bool)
	subjects := make(map[string]bool, len(tasks))
	candidates := make([]grammar.DependencyCandidate, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		subjects[task.ID] = true
		for _, reference := range task.Dependencies {
			stored[app.LocatorForReference(reference)] = true
		}
		candidates = append(candidates, grammar.DependencyRemovalCandidates(task, projects, nil)...)
	}
	if len(subjects) == 0 {
		return listsurface.ListSource{}
	}
	for _, candidate := range grammar.DependencyAdditionCandidates(projects, "", stored) {
		if candidate.Task != nil && subjects[candidate.Task.ID] {
			continue
		}
		candidates = append(candidates, candidate)
	}

	items := make([]listsurface.ListItem, 0, len(candidates))
	seen := make(map[listsurface.ListItemID]bool, len(candidates))
	grouped := make(map[listsurface.ListCategoryID]bool, len(projects))
	for _, candidate := range candidates {
		id := TaskReferenceRowID(candidate.Locator)
		if seen[id] {
			continue
		}
		seen[id] = true
		item := listsurface.ListItem{
			ID:      id,
			Aliases: []string{grammar.DependencyCandidateSearchText(candidate)},
			Data:    candidate.Locator.Reference().Locator(),
		}
		if candidate.Task == nil {
			// A reference this view could not read keeps its stored locator and says
			// only that it is unavailable, the same word Peek uses. It stays enabled
			// like every other stored row: nothing can be said about the target, but
			// the reference itself is still the user's to drop.
			item.PrimaryText = candidate.Locator.Reference().Locator()
			item.SecondaryText = outliner.DependencyDetailUnavailable
			items = append(items, item)
			continue
		}
		item.PrimaryText = candidate.Task.Title
		item.SecondaryText = fmt.Sprintf("%s #%d", candidate.Task.State().Icon(), candidate.Task.ShortID)
		if candidate.Project != nil {
			item.CategoryID = listsurface.ListCategoryID(candidate.Project.ID)
			grouped[item.CategoryID] = true
		}
		items = append(items, item)
	}

	categories := make([]listsurface.ListCategory, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		id := listsurface.ListCategoryID(project.ID)
		if !grouped[id] {
			continue
		}
		grouped[id] = false
		categories = append(categories, listsurface.ListCategory{ID: id, Label: project.Name})
	}
	return listsurface.ListSource{Items: items, Categories: categories}
}

// TaskReferenceSelection is what the picker emits on Enter: the rows whose
// membership the user actually moved, and never the whole checked set. A full
// checked set would be an assertion about every reference at once, including the
// ones the user never looked at, and the overlay is not modal against the
// workspace — an accepted cross-project load republishes task trees through
// adoptCrossProjectSnapshot while `~` is still open. Emitting a difference is
// what keeps such a change out of the commit instead of silently reverting it,
// and over a selection it is also what leaves a row nobody agreed on alone.
//
// Both halves are sorted, so the emitted value never depends on the order rows
// were ticked in or on map iteration.
func TaskReferenceSelection(initial, current map[listsurface.ListItemID]OperationValueMembership) operation.OperationTaskReferenceValue {
	value := operation.OperationTaskReferenceValue{}
	for id, state := range current {
		locator, ok := TaskReferenceRowLocator(id)
		if !ok {
			continue
		}
		canonical := locator.Reference().Locator()
		was := initial[id]
		switch {
		case state == OperationValueAll && was != OperationValueAll:
			value.Add = append(value.Add, canonical)
		case state == OperationValueNone && was != OperationValueNone:
			value.Remove = append(value.Remove, canonical)
		}
	}
	sort.Strings(value.Add)
	sort.Strings(value.Remove)
	return value
}

// TaskReferenceBulkPatch is the selection turned into a patch for a batch, where
// no single stored set exists to merge against. Every checked row becomes an
// addition and every unchecked one a removal, and the application layer supplies
// the idempotence the single-task path gets from merging: an addition a target
// already stores round-trips through the known-addition rule, and a removal
// naming nothing it stores is a no-op.
func TaskReferenceBulkPatch(value operation.OperationTaskReferenceValue) app.DependencyPatch {
	patch := app.DependencyPatch{}
	for _, raw := range value.Add {
		if locator, ok := TaskReferenceLocator(raw); ok {
			patch.Add = append(patch.Add, locator)
		}
	}
	for _, raw := range value.Remove {
		if locator, ok := TaskReferenceLocator(raw); ok {
			patch.Remove = append(patch.Remove, locator)
		}
	}
	return patch
}

// TaskReferencePatch merges the picker's selection into what the task stores at
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
func TaskReferencePatch(task *models.Task, value operation.OperationTaskReferenceValue) app.DependencyPatch {
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
		locator, ok := TaskReferenceLocator(raw)
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
		locator, ok := TaskReferenceLocator(raw)
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

// TaskReferenceLocator splits one canonical row payload. Rows carry the durable
// locator rather than a display alias, so resolving one asks nothing about an
// effective project and cannot pick up a slug the user never typed.
func TaskReferenceLocator(value string) (app.TaskLocator, bool) {
	projectID, taskID, qualified := strings.Cut(value, models.TaskReferenceSeparator)
	projectID, taskID = strings.TrimSpace(projectID), strings.TrimSpace(taskID)
	if !qualified || projectID == "" || taskID == "" {
		return app.TaskLocator{}, false
	}
	return app.TaskLocator{ProjectID: projectID, TaskID: taskID}, true
}
