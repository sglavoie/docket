package models

import (
	"fmt"
	"strings"
)

// AuditProject verifies the highest-value structural project invariants.
// It is deterministic, fail-fast, and returns the first actionable problem found.
func AuditProject(p *Project) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}

	taskMap := make(map[string]*Task, len(p.Tasks))
	for i, task := range p.Tasks {
		if task == nil {
			return fmt.Errorf("project has nil task at index %d", i)
		}
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("task at index %d (%q) has empty task ID", i, task.Title)
		}
		if existing, ok := taskMap[task.ID]; ok {
			return fmt.Errorf("duplicate task ID %q used by %s and %s", task.ID, auditTaskLabel(existing), auditTaskLabel(task))
		}
		taskMap[task.ID] = task
	}

	for _, task := range p.Tasks {
		if err := auditTaskHierarchy(task, taskMap); err != nil {
			return err
		}
		if err := auditTaskDependencies(p, task, taskMap); err != nil {
			return err
		}
		if err := auditTaskRelatedRefs(task, taskMap); err != nil {
			return err
		}
		if err := auditTaskSectionState(task); err != nil {
			return err
		}
	}

	return nil
}

func auditTaskHierarchy(task *Task, taskMap map[string]*Task) error {
	if task.ParentID == nil {
		return nil
	}

	parentID := *task.ParentID
	if strings.TrimSpace(parentID) == "" {
		return fmt.Errorf("%s has empty parent ID", auditTaskLabel(task))
	}

	parent, ok := taskMap[parentID]
	if !ok {
		return fmt.Errorf("%s references missing parent %q", auditTaskLabel(task), parentID)
	}

	if WouldCreateCycle(taskMap, task.ID, parentID) {
		return fmt.Errorf("%s has cyclic parent chain through %s", auditTaskLabel(task), auditTaskLabel(parent))
	}

	if !task.Type.CanBeChildOf(parent.Type) {
		return fmt.Errorf("%s of type %s cannot be child of %s of type %s",
			auditTaskLabel(task), task.Type, auditTaskLabel(parent), parent.Type)
	}

	return nil
}

func auditTaskDependencies(p *Project, task *Task, taskMap map[string]*Task) error {
	for _, depValue := range task.DependsOn {
		if strings.TrimSpace(depValue) == "" {
			return fmt.Errorf("%s has empty dependency reference", auditTaskLabel(task))
		}

		depRef := ParseTaskRef(depValue)
		if depRef.IsEmpty() {
			return fmt.Errorf("%s has malformed dependency reference %q", auditTaskLabel(task), depValue)
		}
		if depRef.IsCrossProject() {
			continue
		}
		if depRef.TaskID == task.ID {
			return fmt.Errorf("%s depends on itself", auditTaskLabel(task))
		}

		dependency, ok := taskMap[depRef.TaskID]
		if !ok {
			return fmt.Errorf("%s depends on missing local task %q", auditTaskLabel(task), depRef.TaskID)
		}

		if p.HasCircularDependency(task.ID, depRef.TaskID) {
			return fmt.Errorf("%s has cyclic dependency on %s", auditTaskLabel(task), auditTaskLabel(dependency))
		}
	}

	return nil
}

func auditTaskRelatedRefs(task *Task, taskMap map[string]*Task) error {
	for _, relatedValue := range task.RelatedTo {
		if strings.TrimSpace(relatedValue) == "" {
			return fmt.Errorf("%s has empty related reference", auditTaskLabel(task))
		}

		relatedRef := ParseTaskRef(relatedValue)
		if relatedRef.IsEmpty() {
			return fmt.Errorf("%s has malformed related reference %q", auditTaskLabel(task), relatedValue)
		}
		if relatedRef.IsCrossProject() {
			continue
		}
		if relatedRef.TaskID == task.ID {
			return fmt.Errorf("%s is related to itself", auditTaskLabel(task))
		}
		if _, ok := taskMap[relatedRef.TaskID]; !ok {
			return fmt.Errorf("%s is related to missing local task %q", auditTaskLabel(task), relatedRef.TaskID)
		}
	}

	return nil
}

// auditTaskSectionState validates the task's canonical workflow state: a stored
// state, when present, must be one of the ten valid states. The retired legacy
// Section field no longer exists, so there is nothing else to check here.
func auditTaskSectionState(task *Task) error {
	if task.WorkflowState != "" && !task.WorkflowState.IsValid() {
		return fmt.Errorf("%s has invalid workflow state %q", auditTaskLabel(task), task.WorkflowState)
	}

	return nil
}

func auditTaskLabel(task *Task) string {
	switch {
	case task == nil:
		return "task <nil>"
	case task.Title != "" && task.ID != "":
		return fmt.Sprintf("task %q (id %q)", task.Title, task.ID)
	case task.Title != "":
		return fmt.Sprintf("task %q", task.Title)
	case task.ID != "":
		return fmt.Sprintf("task id %q", task.ID)
	default:
		return "task <unnamed>"
	}
}
