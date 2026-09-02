package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// applyOutlinePlan mutates the scope project in memory. Parentage and sibling
// order settle before any state moves, so the Done-with-nonterminal-child guard
// reads the tree the buffer actually asked for rather than the one it replaced.
func (s *Service) applyOutlinePlan(project *models.Project, base *storage.ProjectDocument, prepared []preparedOutlineLine) ([]PlanningReconcileTarget, error) {
	if err := s.materializeOutlineTasks(project, prepared); err != nil {
		return nil, err
	}
	applyOutlineParentage(prepared)
	applyOutlineOrder(project, prepared)
	if err := validateOutlineNesting(project, prepared); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for _, item := range prepared {
		item.task.Title = item.title
		item.task.Priority = item.priority
		item.task.Tags = append([]string{}, item.tags...)
		item.task.Context = item.context
		item.task.DueDate = cloneDueDatePtr(item.line.Due)
		item.task.Dependencies = models.CloneTaskReferences(item.dependencies)
		item.task.ApplyPinned(item.line.Pinned, now)
		item.task.ApplyReview(item.line.Review, now)
	}
	return s.applyOutlineDestinations(project, base, prepared, now)
}

func (s *Service) materializeOutlineTasks(project *models.Project, prepared []preparedOutlineLine) error {
	for i := range prepared {
		if !prepared[i].creates() {
			continue
		}
		task, err := newTaskForCreation(s.ids.NewID(), project.GetNextShortID(), prepared[i].title)
		if err != nil {
			return outlineLineError(prepared[i].line, prepared[i].position, invalidInput("title", err.Error()))
		}
		prepared[i].task = task
		project.Tasks = append(project.Tasks, task)
	}
	return nil
}

// applyOutlineParentage derives parentage from indentation alone. Indentation is
// the buffer's only parentage channel, which is why >parent and <child are both
// rejected: two channels would need a precedence rule nobody can see in the text.
func applyOutlineParentage(prepared []preparedOutlineLine) {
	deepestAt := make(map[int]string, len(prepared))
	for _, item := range prepared {
		depth := item.line.Depth
		if depth == 0 {
			item.task.ParentID = nil
		} else {
			parentID := deepestAt[depth-1]
			item.task.ParentID = &parentID
		}
		deepestAt[depth] = item.task.ID
	}
}

// applyOutlineOrder rewrites the project's task slice so the buffer's order is
// the sibling order. The buffered block takes the position of the first task it
// already held, so tasks the buffer never showed keep their relative placement.
func applyOutlineOrder(project *models.Project, prepared []preparedOutlineLine) {
	ordered := make([]*models.Task, 0, len(prepared))
	buffered := make(map[string]bool, len(prepared))
	for _, item := range prepared {
		ordered = append(ordered, item.task)
		buffered[item.task.ID] = true
	}
	out := make([]*models.Task, 0, len(project.Tasks))
	spliced := false
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		if buffered[task.ID] {
			if !spliced {
				spliced = true
				out = append(out, ordered...)
			}
			continue
		}
		out = append(out, task)
	}
	if !spliced {
		out = append(out, ordered...)
	}
	project.Tasks = out
}

// validateOutlineNesting rejects a buffer whose indentation pushes any task past
// the nesting cap, including a descendant the buffer never showed. The check is
// explicit because BuildTaskTree silently detaches an over-deep task instead of
// failing, which would turn an indentation mistake into lost parentage.
func validateOutlineNesting(project *models.Project, prepared []preparedOutlineLine) error {
	byID := make(map[string]*models.Task, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil {
			byID[task.ID] = task
		}
	}
	depths := make(map[string]int, len(project.Tasks))
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		depth, err := outlineTaskDepth(task, byID, depths, make(map[string]bool))
		if err != nil {
			return err
		}
		if depth > models.MaxNestingDepth {
			return outlineNestingError(task, prepared)
		}
	}
	return nil
}

func outlineTaskDepth(task *models.Task, byID map[string]*models.Task, depths map[string]int, visiting map[string]bool) (int, error) {
	if depth, ok := depths[task.ID]; ok {
		return depth, nil
	}
	if visiting[task.ID] {
		return 0, invalidInput("indentation", fmt.Sprintf("#%d has a circular parent reference", task.ShortID))
	}
	visiting[task.ID] = true
	depth := 0
	if task.ParentID != nil {
		if parent := byID[*task.ParentID]; parent != nil {
			parentDepth, err := outlineTaskDepth(parent, byID, depths, visiting)
			if err != nil {
				return 0, err
			}
			depth = parentDepth + 1
		}
	}
	depths[task.ID] = depth
	return depth, nil
}

func outlineNestingError(task *models.Task, prepared []preparedOutlineLine) error {
	err := invalidInput("indentation", fmt.Sprintf("#%d would nest deeper than the maximum of %d levels", task.ShortID, models.MaxNestingDepth))
	for _, item := range prepared {
		if item.task.ID == task.ID {
			return outlineLineError(item.line, item.position, err)
		}
	}
	return err
}

func (s *Service) applyOutlineDestinations(project *models.Project, base *storage.ProjectDocument, prepared []preparedOutlineLine, now time.Time) ([]PlanningReconcileTarget, error) {
	targets := make([]PlanningReconcileTarget, 0, len(prepared))
	for _, item := range prepared {
		target, err := s.applyOutlineDestination(project, base, item, now)
		if err != nil {
			return nil, outlineLineError(item.line, item.position, err)
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (s *Service) applyOutlineDestination(project *models.Project, base *storage.ProjectDocument, item preparedOutlineLine, now time.Time) (PlanningReconcileTarget, error) {
	if item.creates() {
		effect, err := normalizeCaptureDestination(item.line.Destination, item.task.ParentID != nil)
		if err != nil {
			return PlanningReconcileTarget{}, err
		}
		if err := s.initializeCreatedTask(item.task, effect.state); err != nil {
			return PlanningReconcileTarget{}, err
		}
		return effect.planningTarget(item.task.ID, project.ID), nil
	}
	// An omitted destination preserves the state the task already holds. A
	// declarative buffer may clear a tag or a date, but resetting a lifecycle
	// state through an absent sigil would launder a transition nobody typed.
	if item.line.Destination == "" {
		return PlanningReconcileTarget{TaskID: item.task.ID, ProjectID: project.ID, State: item.task.State()}, nil
	}
	if action, ok := outlinePlanningAction(item.line.Destination); ok {
		target, _, err := ApplyPlanningActionToTask(item.task, project.ID, action, now)
		return target, err
	}
	effect, err := normalizeCaptureDestination(item.line.Destination, false)
	if err != nil {
		return PlanningReconcileTarget{}, err
	}
	target := effect.planningTarget(item.task.ID, project.ID)
	if effect.state == item.task.State() {
		item.task.ApplyWorkflowState(effect.state)
		return target, nil
	}
	ref := TaskRef{Task: item.task, Project: project, Base: base}
	if err := validateEditStateTransition(ref, &effect.state); err != nil {
		return PlanningReconcileTarget{}, err
	}
	if err := applyState(item.task, effect.state, now); err != nil {
		return PlanningReconcileTarget{}, invalidInput("state", err.Error())
	}
	target.StateChanged = true
	return target, nil
}

// outlinePlanningAction routes the two planning destinations through the
// canonical planning primitive, so $today on an existing Doing task keeps Doing
// exactly as the Today operation does instead of demoting it to Backlog.
func outlinePlanningAction(destination CaptureDestination) (PlanningAction, bool) {
	switch destination {
	case CaptureDestinationToday:
		return PlanningAddToday, true
	case CaptureDestinationNext:
		return PlanningAddNext, true
	default:
		return "", false
	}
}
