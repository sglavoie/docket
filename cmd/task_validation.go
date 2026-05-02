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

// CheckCompletionBlockers checks whether a task can be completed,
// returning an error if it has incomplete dependencies or children.
// When force is true, the incomplete-children check is skipped.
func CheckCompletionBlockers(project *models.Project, task *models.Task, force bool) error {
	if project.HasIncompleteDependencies(task) {
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

	if !force {
		var incompleteChildren []*models.Task
		for _, child := range task.Children {
			if child.Status != models.StatusDone && child.Status != models.StatusCanceled {
				incompleteChildren = append(incompleteChildren, child)
			}
		}
		if len(incompleteChildren) > 0 {
			var msg strings.Builder
			fmt.Fprintf(&msg, "cannot complete task #%d: has %d incomplete children:\n",
				task.ShortID, len(incompleteChildren))
			for _, child := range incompleteChildren {
				fmt.Fprintf(&msg, "  - #%d %s (%s)\n", child.ShortID, child.Title, child.Status)
			}
			return fmt.Errorf("%s", msg.String())
		}
	}

	return nil
}
