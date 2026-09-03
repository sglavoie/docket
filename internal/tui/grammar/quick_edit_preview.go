package grammar

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

func buildQuickEditPreview(result *QuickEditResult, task *models.Task, project *models.Project) error {
	preview := *task
	preview.Tags = slices.Clone(task.Tags)
	preview.Children = slices.Clone(task.Children)
	preview.Comments = slices.Clone(task.Comments)
	preview.Attachments = slices.Clone(task.Attachments)
	preview.Checklist = slices.Clone(task.Checklist)
	preview.Dependencies = applyQuickEditPreviewDependencies(result, task)
	preview.Title = result.Title
	if result.PrioritySet {
		preview.Priority = result.Priority
	}
	for _, tag := range result.TagsToRemove {
		preview.Tags = slices.DeleteFunc(preview.Tags, func(existing string) bool {
			normalized, err := models.NormalizeTaskTag(existing)
			return err == nil && normalized == tag
		})
	}
	for _, tag := range result.TagsToAdd {
		if !preview.HasTag(tag) {
			preview.Tags = append(preview.Tags, tag)
		}
	}
	if result.ContextSet {
		preview.Context = result.ContextValue
		if result.ContextClear {
			preview.Context = ""
		}
	}
	if result.DueDateSet {
		preview.DueDate = result.DueDate
		if result.DueDateClear {
			preview.DueDate = nil
		}
	}
	if result.DeferSet {
		preview.DeferredUntil = result.DeferUntil
		if result.DeferClear {
			preview.DeferredUntil = nil
		}
	}
	if result.PinnedSet {
		preview.Pinned = result.Pinned
	}
	if result.ReviewSet {
		preview.Review = result.Review
	}
	switch result.ParentMode {
	case QuickEditParentRoot:
		preview.ParentID = nil
		preview.Parent = nil
	case QuickEditParentSet:
		parentID := result.Parent.ID
		preview.ParentID = &parentID
		preview.Parent = result.Parent
	case QuickEditParentPreserve:
	}
	for _, child := range result.Children {
		if !slices.ContainsFunc(preview.Children, func(existing *models.Task) bool {
			return existing != nil && existing.ID == child.ID
		}) {
			preview.Children = append(preview.Children, child)
		}
	}
	if err := applyQuickEditPreviewLifecycle(result, task, project, &preview); err != nil {
		return err
	}
	// An adoption is the one patch that can leave a task terminal above an open
	// child, which the application layer's post-mutation reconciliation reopens.
	if len(result.Children) > 0 && preview.IsTerminalState() && hasIncompleteFinalChildren(result, task, project) {
		preview.Uncomplete()
	}
	result.State = preview.State()
	result.Preview = preview
	return nil
}

// applyQuickEditPreviewLifecycle moves the preview to the state the edit names.
// The three planning destinations route through the planning action and settle
// the state themselves; every other destination, and a bare state edit, is an
// ordinary transition. Only a destination-driven terminal move consults the
// task's children, because a bare state edit is refused later by the same rule
// in the application layer.
func applyQuickEditPreviewLifecycle(result *QuickEditResult, task *models.Task, project *models.Project, preview *models.Task) error {
	if result.DestinationSet {
		switch result.Destination {
		case textgrammar.DestinationToday:
			_, _, err := app.ApplyPlanningActionToTask(preview, project.ID, app.PlanningAddToday, time.Now())
			return err
		case textgrammar.DestinationNext:
			_, _, err := app.ApplyPlanningActionToTask(preview, project.ID, app.PlanningAddNext, time.Now())
			return err
		case textgrammar.DestinationBacklog:
			_, _, err := app.ApplyPlanningActionToTask(preview, project.ID, app.PlanningBacklog, time.Now())
			return err
		default:
			// Every other destination settles below as a state transition.
		}
	} else if !result.StateSet {
		return nil
	}
	// The children check precedes the equal-state return because a destination the
	// task already holds is still an explicit terminal claim, and the application
	// layer's guard refuses it on the same terms.
	if result.DestinationSet && result.State.IsTerminal() && hasIncompleteFinalChildren(result, task, project) {
		return fmt.Errorf("task #%d has incomplete children and cannot move to %s", task.ShortID, result.State)
	}
	if preview.State() == result.State {
		return nil
	}
	if reason := models.CanTransitionState(preview.State(), result.State); reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return preview.ApplyStateTransition(result.State)
}

// applyQuickEditPreviewDependencies overlays the soft-reference patch on the
// stored set with the same rules resolveDependencyPatch applies at commit:
// removals drop first, additions append in first-occurrence order, and a clear
// replaces everything with nothing. Keeping the two in step is what stops the
// preview from promising a set the commit would not produce.
func applyQuickEditPreviewDependencies(result *QuickEditResult, task *models.Task) []models.TaskReference {
	if result.dependencyClear {
		return nil
	}
	next := make([]models.TaskReference, 0, len(task.Dependencies)+len(result.dependencyAdds))
	for _, existing := range task.Dependencies {
		if slices.ContainsFunc(result.dependencyRemovals, func(removal CapturedDependencyRef) bool {
			return removal.Locator.Reference() == existing
		}) {
			continue
		}
		if !slices.Contains(next, existing) {
			next = append(next, existing)
		}
	}
	for _, addition := range result.dependencyAdds {
		reference := addition.Locator.Reference()
		if !slices.Contains(next, reference) {
			next = append(next, reference)
		}
	}
	return models.CloneTaskReferences(next)
}

func hasIncompleteDirectChildren(task *models.Task, project *models.Project) bool {
	for _, candidate := range project.Tasks {
		if candidate != nil && candidate.ParentID != nil && *candidate.ParentID == task.ID && !candidate.IsTerminalState() {
			return true
		}
	}
	return false
}

// hasIncompleteFinalChildren reads the children the edit ends with: the ones the
// task already owns plus the ones it adopts. Adoption is additive, so nothing
// leaves the set and the two sources simply combine.
func hasIncompleteFinalChildren(result *QuickEditResult, task *models.Task, project *models.Project) bool {
	if hasIncompleteDirectChildren(task, project) {
		return true
	}
	return slices.ContainsFunc(result.Children, func(child *models.Task) bool {
		return child != nil && !child.IsTerminalState()
	})
}
