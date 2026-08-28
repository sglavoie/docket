package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// DependencyPatch is the preserve-by-default dependency intent one mutation
// carries. Add and Remove are locator lists in the order the caller expressed
// them; Clear replaces the whole stored set with nothing and cannot be combined
// with either list, because "forget everything" and "forget this one" are
// different requests and honoring both at once would hide which one won.
//
// The zero value expresses no intent at all, which is what keeps an edit that
// never mentioned a dependency from rewriting the stored set.
type DependencyPatch struct {
	Add    []TaskLocator
	Remove []TaskLocator
	Clear  bool
}

// IsZero reports whether the patch expresses no dependency intent.
func (p DependencyPatch) IsZero() bool {
	return !p.Clear && len(p.Add) == 0 && len(p.Remove) == 0
}

// Reference converts an application locator into the durable model reference
// stored on a task. The two carry the same pair of stable identities; keeping
// the conversion in one place is what stops a presentation locator from being
// persisted with a display value in either component.
func (l TaskLocator) Reference() models.TaskReference {
	return models.TaskReference{ProjectID: l.ProjectID, TaskID: l.TaskID}
}

// LocatorForReference is the inverse of TaskLocator.Reference, for the surfaces
// that read a stored dependency back and need to resolve it against a snapshot.
func LocatorForReference(reference models.TaskReference) TaskLocator {
	return TaskLocator{ProjectID: reference.ProjectID, TaskID: reference.TaskID}
}

// dependencyTargetState is what a dependency locator resolves to inside one
// snapshot. The four failure shapes are kept apart because they are different
// facts about the workspace, not different wordings of one: a target in an
// unloaded project may well exist, a missing one does not, a terminal one exists
// and is simply closed, and a self-reference is the task itself.
type dependencyTargetState uint8

const (
	dependencyTargetOpen dependencyTargetState = iota
	dependencyTargetTerminal
	dependencyTargetMissing
	dependencyTargetUnavailable
	dependencyTargetSelf
)

// referenceResolver answers the workspace-shaped questions a dependency patch
// asks, against exactly one revision-consistent snapshot. Resolving every
// locator through a single snapshot is what makes a multi-target patch judged
// on one coherent view of the workspace rather than on a view that moved
// between two lookups.
type referenceResolver struct {
	snap Snapshot
	// self is the locator the subject task owns once the mutation lands. On an
	// edit that moves the task it is the destination locator, which is why a
	// line that both relocates and depends on its own new address is refused.
	self TaskLocator
	// selfTaskID catches the same task named through the locator it is leaving.
	// Task IDs are workspace-stable, so a locator resolving to this ID is the
	// subject however it was qualified.
	selfTaskID string
}

// newReferenceResolver builds a resolver whose self-reference test is the given
// effective destination locator. A zero locator means the subject task does not
// exist yet (capture), so no locator can name it.
func newReferenceResolver(snap Snapshot, self TaskLocator) referenceResolver {
	return referenceResolver{snap: snap, self: self, selfTaskID: self.TaskID}
}

// classify reports what one locator resolves to. It never mutates and never
// walks the dependency graph: cycles and reciprocal references are legal, so
// there is nothing beyond the direct target to look at.
func (r referenceResolver) classify(locator TaskLocator) (models.TaskReference, dependencyTargetState) {
	reference := locator.Reference()
	if r.selfTaskID != "" && locator.TaskID == r.selfTaskID {
		return reference, dependencyTargetSelf
	}
	if r.self.Valid() && locator == r.self {
		return reference, dependencyTargetSelf
	}
	if r.snap == nil || r.snap.ProjectByID(locator.ProjectID) == nil {
		return reference, dependencyTargetUnavailable
	}
	ref, ok := r.snap.LookupLocator(locator)
	if !ok {
		return reference, dependencyTargetMissing
	}
	if ref.Task.IsTerminalState() {
		return reference, dependencyTargetTerminal
	}
	return reference, dependencyTargetOpen
}

// normalizeDependencyLocator trims both components and refuses a shape that
// could never be persisted, so a malformed locator fails before it is measured
// against the workspace and the same wording covers additions and removals.
func normalizeDependencyLocator(locator TaskLocator) (TaskLocator, error) {
	locator.ProjectID = strings.TrimSpace(locator.ProjectID)
	locator.TaskID = strings.TrimSpace(locator.TaskID)
	if err := models.ValidateTaskReference(locator.ProjectID, locator.TaskID); err != nil {
		return TaskLocator{}, invalidInput("dependency", err.Error())
	}
	return locator, nil
}

// resolveAddition validates one addition. An addition is the only direction
// that asks anything of the workspace: it must name a real loaded task that is
// still open, because recording a reference to something the user cannot see is
// how a dangling set is created rather than reported.
func (r referenceResolver) resolveAddition(raw TaskLocator) (models.TaskReference, error) {
	locator, err := normalizeDependencyLocator(raw)
	if err != nil {
		return models.TaskReference{}, err
	}
	reference, state := r.classify(locator)
	switch state {
	case dependencyTargetOpen:
		return reference, nil
	case dependencyTargetSelf:
		return models.TaskReference{}, invalidInput("dependency", "a task cannot depend on itself")
	case dependencyTargetTerminal:
		return models.TaskReference{}, invalidInput("dependency", fmt.Sprintf("dependency target %q is already closed", reference.Locator()))
	case dependencyTargetUnavailable:
		return models.TaskReference{}, invalidInput("dependency", fmt.Sprintf("dependency target %q is not available", reference.Locator()))
	default:
		return models.TaskReference{}, invalidInput("dependency", fmt.Sprintf("dependency target %q does not exist", reference.Locator()))
	}
}

// resolveDependencyAdditions validates every addition and collapses repeated
// locators to their first occurrence, which is the order the set is stored in.
// A later duplicate never moves an entry.
func (r referenceResolver) resolveDependencyAdditions(locators []TaskLocator) ([]models.TaskReference, error) {
	if len(locators) == 0 {
		return nil, nil
	}
	resolved := make([]models.TaskReference, 0, len(locators))
	for _, locator := range locators {
		reference, err := r.resolveAddition(locator)
		if err != nil {
			return nil, err
		}
		if slices.Contains(resolved, reference) {
			continue
		}
		resolved = append(resolved, reference)
	}
	return resolved, nil
}

// resolveDependencyRemovals validates removal locators only for shape. A stored
// reference whose target has since become terminal, been deleted, or landed in
// a project this snapshot did not load must stay removable — refusing it would
// strand exactly the references a user most wants to drop.
func resolveDependencyRemovals(locators []TaskLocator) ([]models.TaskReference, error) {
	if len(locators) == 0 {
		return nil, nil
	}
	resolved := make([]models.TaskReference, 0, len(locators))
	for _, raw := range locators {
		locator, err := normalizeDependencyLocator(raw)
		if err != nil {
			return nil, err
		}
		reference := locator.Reference()
		if slices.Contains(resolved, reference) {
			continue
		}
		resolved = append(resolved, reference)
	}
	return resolved, nil
}

// resolveDependencyPatch resolves and validates a complete patch against the
// currently stored set and returns the set the mutation should store. It runs
// before anything is mutated, so a patch with one bad target leaves the task
// exactly as it was. An addition already present, a removal naming nothing, and
// a clear over an empty set all resolve to the existing set, which is what makes
// them semantic no-ops rather than errors.
func (r referenceResolver) resolveDependencyPatch(existing []models.TaskReference, patch DependencyPatch) ([]models.TaskReference, error) {
	if patch.Clear && (len(patch.Add) > 0 || len(patch.Remove) > 0) {
		return nil, invalidInput("dependency", "clearing dependencies cannot be combined with adding or removing them")
	}
	if patch.Clear {
		return nil, nil
	}
	additions, err := r.resolveDependencyAdditions(patch.Add)
	if err != nil {
		return nil, err
	}
	removals, err := resolveDependencyRemovals(patch.Remove)
	if err != nil {
		return nil, err
	}
	for _, addition := range additions {
		if slices.Contains(removals, addition) {
			return nil, invalidInput("dependency", fmt.Sprintf("cannot add and remove %q in one edit", addition.Locator()))
		}
	}

	next := make([]models.TaskReference, 0, len(existing)+len(additions))
	for _, reference := range existing {
		if slices.Contains(removals, reference) {
			continue
		}
		if slices.Contains(next, reference) {
			continue
		}
		next = append(next, reference)
	}
	for _, addition := range additions {
		if slices.Contains(next, addition) {
			continue
		}
		next = append(next, addition)
	}
	return models.CloneTaskReferences(next), nil
}

// TaskDependents indexes the incoming references of every loaded task: the key
// is a referenced task and the value is every task naming it, in project and
// task order. It is a read-only presentation lookup — dependents are computed,
// never stored — and it deliberately sees only the projects the snapshot loaded,
// so an archived project is not opened merely to decorate a detail pane.
func TaskDependents(snap Snapshot) map[models.TaskReference][]models.TaskReference {
	if snap == nil {
		return nil
	}
	index := make(map[models.TaskReference][]models.TaskReference)
	for _, project := range snap.Projects() {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.ID == "" || len(task.Dependencies) == 0 {
				continue
			}
			source := models.TaskReference{ProjectID: project.ID, TaskID: task.ID}
			for _, target := range task.Dependencies {
				if slices.Contains(index[target], source) {
					continue
				}
				index[target] = append(index[target], source)
			}
		}
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

// AddTaskDependencies records one or more soft references on a task. The
// references are informational: nothing about the subject's readiness, order, or
// eligibility to complete changes because it gained one.
func (s *Service) AddTaskDependencies(locator TaskLocator, targets []TaskLocator) (TaskResult, error) {
	return s.patchTaskDependencies(locator, DependencyPatch{Add: targets}, "Add Dependencies")
}

// RemoveTaskDependencies drops the named references. A locator the task does not
// carry is a no-op, and a reference whose target is closed or unloaded is still
// removable.
func (s *Service) RemoveTaskDependencies(locator TaskLocator, targets []TaskLocator) (TaskResult, error) {
	return s.patchTaskDependencies(locator, DependencyPatch{Remove: targets}, "Remove Dependencies")
}

// ClearTaskDependencies drops every reference the task carries.
func (s *Service) ClearTaskDependencies(locator TaskLocator) (TaskResult, error) {
	return s.patchTaskDependencies(locator, DependencyPatch{Clear: true}, "Clear Dependencies")
}

// patchTaskDependencies is the single locator-based dependency mutation. It
// resolves one snapshot, validates the whole patch before touching the task, and
// restores the repository-owned projects when the commit fails, so a rejected or
// unpersisted patch leaves no partial set behind.
func (s *Service) patchTaskDependencies(locator TaskLocator, patch DependencyPatch, action string) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	resolver := newReferenceResolver(snap, TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID})
	next, err := resolver.resolveDependencyPatch(ref.Task.Dependencies, patch)
	if err != nil {
		return TaskResult{}, err
	}

	before, err := cloneSnapshot(snap)
	if err != nil {
		return TaskResult{}, fmt.Errorf("capture dependency rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	// A no-op patch still commits, matching the other idempotent mutations, but it
	// must not advance the touch time: UpdatedAt answers "when did this task last
	// change", and a request that changed nothing is not an answer.
	if !models.TaskReferencesEqual(ref.Task.Dependencies, next) {
		ref.Task.Dependencies = next
		ref.Task.UpdatedAt = s.clock.Now()
	}
	result, err := s.commitProject(ref, revision, action, fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title))
	if err != nil {
		return TaskResult{}, err
	}
	committed = true
	return result, nil
}
