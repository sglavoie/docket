package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// ValidatePriority checks that a priority value is in the valid range.
// If allowZero is true, 0 is accepted (meaning "use default").
func ValidatePriority(value int, allowZero bool) error {
	return models.ValidatePriority(value, allowZero)
}

// ParseTaskTypeFlags determines the task type from boolean flags or a string flag.
// Returns an error if the type flag is invalid or if multiple type flags are set.
func ParseTaskTypeFlags(epic, story, bug, idea bool, typeFlag string) (models.TaskType, error) {
	// Count how many boolean type flags are set
	flagCount := 0
	var flags []string
	if epic {
		flagCount++
		flags = append(flags, "--epic")
	}
	if story {
		flagCount++
		flags = append(flags, "--story")
	}
	if bug {
		flagCount++
		flags = append(flags, "--bug")
	}
	if idea {
		flagCount++
		flags = append(flags, "--idea")
	}
	if typeFlag != "" {
		flagCount++
		flags = append(flags, "--type="+typeFlag)
	}
	if flagCount > 1 {
		return "", fmt.Errorf("conflicting type flags: %s (specify only one)", strings.Join(flags, ", "))
	}

	if epic {
		return models.TaskTypeEpic, nil
	}
	if story {
		return models.TaskTypeStory, nil
	}
	if bug {
		return models.TaskTypeBug, nil
	}
	if idea {
		return models.TaskTypeIdea, nil
	}
	if typeFlag != "" {
		t, ok := models.ParseTaskType(typeFlag)
		if !ok {
			return "", tterrors.NewInvalidTaskTypeError(typeFlag)
		}
		return t, nil
	}
	return models.TaskTypeTask, nil
}

// CheckCompletionBlockers checks whether a task can be completed, returning an
// error if it has incomplete dependencies or children. Completion is gated on
// both; cancellation is gated on children only via checkStatusChangeBlockers.
func CheckCompletionBlockers(project *models.Project, task *models.Task) error {
	if err := checkIncompleteDependencies(project, task); err != nil {
		return err
	}
	return checkIncompleteChildren(task)
}

// checkStatusChangeBlockers gates a status change on the completion invariant:
// completion (StatusDone) is blocked by incomplete dependencies and children,
// while cancellation (StatusCanceled) is blocked by incomplete children only.
// A parent must never become done/canceled while a child is still open, but
// dependency ordering is a separate rule that only applies to completion.
func checkStatusChangeBlockers(project *models.Project, task *models.Task, targetStatus models.TaskStatus) error {
	if targetStatus == models.StatusDone {
		if err := checkIncompleteDependencies(project, task); err != nil {
			return err
		}
	}
	if targetStatus.IsComplete() {
		if err := checkIncompleteChildren(task); err != nil {
			return err
		}
	}
	return nil
}

// checkIncompleteDependencies returns an error if the task has incomplete
// dependencies. Only completion is gated on dependencies.
func checkIncompleteDependencies(project *models.Project, task *models.Task) error {
	if !project.HasIncompleteDependencies(task) {
		return nil
	}
	incompleteDeps := project.GetIncompleteDependencies(task)
	var msg strings.Builder
	fmt.Fprintf(&msg, "cannot complete task #%d: has incomplete dependencies:\n", task.ShortID)
	for _, dep := range incompleteDeps {
		fmt.Fprintf(&msg, "  - #%d %s (%s)\n", dep.ShortID, dep.Title, dep.Status)
	}
	crossProjectCount := countCrossProjectDeps(task)
	if crossProjectCount > 0 {
		fmt.Fprintf(&msg, "  - %d cross-project dependency/dependencies (not resolvable locally)\n", crossProjectCount)
	}
	return fmt.Errorf("%s", msg.String())
}

// checkIncompleteChildren returns an error if the task has incomplete children.
// Both completing and canceling a parent are gated on children, so the message
// is status-neutral.
func checkIncompleteChildren(task *models.Task) error {
	incompleteChildren := task.IncompleteChildren()
	if len(incompleteChildren) == 0 {
		return nil
	}
	var msg strings.Builder
	fmt.Fprintf(&msg, "task #%d has %d incomplete children:\n", task.ShortID, len(incompleteChildren))
	for _, child := range incompleteChildren {
		fmt.Fprintf(&msg, "  - #%d %s (%s)\n", child.ShortID, child.Title, child.Status)
	}
	return fmt.Errorf("%s", msg.String())
}
