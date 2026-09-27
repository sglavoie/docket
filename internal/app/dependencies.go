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
//
// Only the self-reference test is the patch's own: a subject task is a fact
// about the mutation being validated, not about the workspace. Everything past
// it is DependencyTargetStateFor, the one classifier every reader shares, so a
// patch and a rendered row can never disagree about what a locator points at.
func (r referenceResolver) classify(locator TaskLocator) (models.TaskReference, DependencyTargetState) {
	reference := locator.Reference()
	if r.selfTaskID != "" && locator.TaskID == r.selfTaskID {
		return reference, DependencyTargetSelf
	}
	if r.self.Valid() && locator == r.self {
		return reference, DependencyTargetSelf
	}
	var target *models.Task
	loaded := r.snap != nil && r.snap.ProjectByID(locator.ProjectID) != nil
	if loaded {
		if ref, ok := r.snap.LookupLocator(locator); ok {
			target = ref.Task
		}
	}
	return reference, DependencyTargetStateFor(loaded, target)
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
	case DependencyTargetOpen:
		return reference, nil
	case DependencyTargetSelf:
		return models.TaskReference{}, invalidInput("dependency", "a task cannot depend on itself")
	case DependencyTargetTerminal:
		return models.TaskReference{}, invalidInput("dependency", fmt.Sprintf("dependency target %q is already closed", reference.Locator()))
	case DependencyTargetUnavailable:
		return models.TaskReference{}, invalidInput("dependency", fmt.Sprintf("dependency target %q is not available", reference.Locator()))
	default:
		return models.TaskReference{}, invalidInput("dependency", fmt.Sprintf("dependency target %q does not exist", reference.Locator()))
	}
}

// resolveKnownOrNewAddition is the existing-versus-new distinction every
// addition goes through. A locator the task already stores round-trips on shape
// alone: the workspace deliberately preserves a reference after its target
// closes or its project stops being loaded, so re-submitting one must stay the
// no-op it was the moment before the target changed. Anything the request newly
// introduces goes through the ordinary addition rule, which is what keeps a
// genuinely new terminal or unavailable target invalid.
func (r referenceResolver) resolveKnownOrNewAddition(existing []models.TaskReference, raw TaskLocator) (models.TaskReference, error) {
	locator, err := normalizeDependencyLocator(raw)
	if err != nil {
		return models.TaskReference{}, err
	}
	if reference := locator.Reference(); slices.Contains(existing, reference) {
		return reference, nil
	}
	return r.resolveAddition(locator)
}

// resolveDependencyAdditions validates every addition and collapses repeated
// locators to their first occurrence, which is the order the set is stored in.
// A later duplicate never moves an entry.
func (r referenceResolver) resolveDependencyAdditions(existing []models.TaskReference, locators []TaskLocator) ([]models.TaskReference, error) {
	if len(locators) == 0 {
		return nil, nil
	}
	resolved := make([]models.TaskReference, 0, len(locators))
	for _, locator := range locators {
		reference, err := r.resolveKnownOrNewAddition(existing, locator)
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
// them semantic no-ops rather than errors — including when the already-present
// target has since become terminal or unavailable.
func (r referenceResolver) resolveDependencyPatch(existing []models.TaskReference, patch DependencyPatch) ([]models.TaskReference, error) {
	if patch.Clear && (len(patch.Add) > 0 || len(patch.Remove) > 0) {
		return nil, invalidInput("dependency", "clearing dependencies cannot be combined with adding or removing them")
	}
	if patch.Clear {
		return nil, nil
	}
	additions, err := r.resolveDependencyAdditions(existing, patch.Add)
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

// bulkPatchDependencies applies one tri-state reference edit to every selected
// task. Locators in Add are made present everywhere, locators in Remove are made
// absent everywhere, and every other stored reference is preserved
// independently on each target — which is what lets one picker edit N tasks
// carrying N different baselines without a replacement set flattening them.
//
// Each target resolves its own patch through its own resolver, so the
// self-reference test stays that task's, but all of them read the one snapshot
// the commit writes. A refusal on any target aborts the batch before a single
// task is touched.
func (s *Service) bulkPatchDependencies(locators []TaskLocator, patch DependencyPatch) (BulkResult, error) {
	return s.bulkAttributeOverSnapshot(locators, "Set Dependencies", "dependencies", func(snap Snapshot, refs []TaskRef) (func(TaskRef) error, error) {
		if err := refuseSelectedDependencyTargets(refs, patch.Add); err != nil {
			return nil, err
		}
		resolved := make(map[string][]models.TaskReference, len(refs))
		for _, ref := range refs {
			resolver := newReferenceResolver(snap, TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID})
			next, err := resolver.resolveDependencyPatch(ref.Task.Dependencies, patch)
			if err != nil {
				return nil, err
			}
			resolved[ref.Task.ID] = next
		}
		now := s.clock.Now()
		return func(ref TaskRef) error {
			next := resolved[ref.Task.ID]
			// A target the patch left alone still commits, but its touch time must
			// not move: UpdatedAt answers "when did this task last change".
			if models.TaskReferencesEqual(ref.Task.Dependencies, next) {
				return nil
			}
			ref.Task.Dependencies = next
			ref.Task.UpdatedAt = now
			return nil
		}, nil
	})
}

// refuseSelectedDependencyTargets rejects an addition naming a task inside the
// selection, by name. One target of the batch would be recording a reference to
// another, which is a relationship between two selected tasks rather than the
// one relationship the gesture asked for; the picker cannot express which
// direction was meant, so neither is guessed. The subject's own self-reference
// is refused a second time downstream, in wording that names it as the subject.
func refuseSelectedDependencyTargets(refs []TaskRef, additions []TaskLocator) error {
	if len(additions) < 1 || len(refs) < 2 {
		return nil
	}
	selected := make(map[string]TaskRef, len(refs))
	for _, ref := range refs {
		selected[ref.Task.ID] = ref
	}
	for _, raw := range additions {
		locator, err := normalizeDependencyLocator(raw)
		if err != nil {
			return err
		}
		ref, ok := selected[locator.TaskID]
		if !ok {
			continue
		}
		return invalidInput("dependency", fmt.Sprintf(
			"#%d %s is in the selection", ref.Task.ShortID, ref.Task.Title))
	}
	return nil
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

	before := cloneSnapshot(snap)
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
