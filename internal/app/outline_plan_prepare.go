package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// preparedOutlineLine is one validated buffer line paired with the task it acts
// on. A nil task means the line creates one; it is materialized only after the
// rollback guard is installed, because minting a short ID mutates the project.
type preparedOutlineLine struct {
	line         OutlinePlanLine
	position     int
	title        string
	priority     int
	tags         []string
	context      string
	dependencies []models.TaskReference
	task         *models.Task
}

func (p preparedOutlineLine) creates() bool { return p.line.TaskID == "" }

// prepareOutlinePlan validates every line before anything is mutated, so a
// rejected buffer commits nothing and the error names the offending line.
func prepareOutlinePlan(snap Snapshot, project *models.Project, lines []OutlinePlanLine) ([]preparedOutlineLine, error) {
	prepared := make([]preparedOutlineLine, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	previousDepth := -1
	for i, line := range lines {
		item, err := prepareOutlineLine(snap, project, line, previousDepth)
		if err != nil {
			return nil, outlineLineError(line, i, err)
		}
		if item.task != nil {
			if seen[item.task.ID] {
				return nil, outlineLineError(line, i, invalidInput("task", fmt.Sprintf("task %s appears on more than one line", item.task.ID)))
			}
			seen[item.task.ID] = true
		}
		item.position = i
		previousDepth = line.Depth
		prepared = append(prepared, item)
	}
	return prepared, nil
}

func prepareOutlineLine(snap Snapshot, project *models.Project, line OutlinePlanLine, previousDepth int) (preparedOutlineLine, error) {
	if line.Depth < 0 || line.Depth > previousDepth+1 {
		return preparedOutlineLine{}, invalidInput("indentation", fmt.Sprintf("depth %d has no parent one level above it", line.Depth))
	}
	title := strings.TrimSpace(line.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return preparedOutlineLine{}, invalidInput("title", err.Error())
	}
	if err := models.ValidatePriority(line.Priority, true); err != nil {
		return preparedOutlineLine{}, invalidInput("priority", err.Error())
	}
	tags, err := normalizeTags(line.Tags)
	if err != nil {
		return preparedOutlineLine{}, err
	}
	context := ""
	if line.Context != "" {
		context, err = models.NormalizeTaskContext(line.Context)
		if err != nil {
			return preparedOutlineLine{}, invalidInput("context", err.Error())
		}
	}
	if line.Destination != "" {
		if _, ok := ParseCaptureDestination(string(line.Destination)); !ok {
			return preparedOutlineLine{}, invalidInput("destination", InvalidCaptureDestinationError(string(line.Destination)).Error())
		}
	}
	item := preparedOutlineLine{line: line, title: title, priority: line.Priority, tags: tags, context: context}
	if line.TaskID == "" {
		dependencies, depErr := resolveOutlineDependencies(snap, project, nil, line.Dependencies)
		if depErr != nil {
			return preparedOutlineLine{}, depErr
		}
		item.dependencies = dependencies
		return item, nil
	}
	task := project.GetTaskByID(line.TaskID)
	if task == nil {
		if _, status := snap.LookupTaskID(line.TaskID); status != TaskLookupMissing {
			return preparedOutlineLine{}, invalidInput("task", fmt.Sprintf("task %s belongs to another project; an outline plan never moves a task across projects", line.TaskID))
		}
		return preparedOutlineLine{}, fmt.Errorf("%w: %s", ErrTaskNotFound, line.TaskID)
	}
	item.task = task
	dependencies, err := resolveOutlineDependencies(snap, project, task, line.Dependencies)
	if err != nil {
		return preparedOutlineLine{}, err
	}
	item.dependencies = dependencies
	return item, nil
}

// resolveOutlineDependencies turns one line's final locator list into the set the
// task should store. A locator the task already carries round-trips verbatim,
// because the dump prints closed and unloaded targets too and re-reading the
// buffer unchanged must never be a refusal. Anything the line newly introduces
// goes through the ordinary addition rule, so the buffer cannot mint a reference
// to a target the user could not have picked from a list.
func resolveOutlineDependencies(snap Snapshot, project *models.Project, task *models.Task, locators []TaskLocator) ([]models.TaskReference, error) {
	if len(locators) == 0 {
		return nil, nil
	}
	var existing []models.TaskReference
	self := TaskLocator{}
	if task != nil {
		existing = task.Dependencies
		self = TaskLocator{ProjectID: project.ID, TaskID: task.ID}
	}
	resolver := newReferenceResolver(snap, self)
	resolved := make([]models.TaskReference, 0, len(locators))
	for _, raw := range locators {
		reference, err := resolver.resolveKnownOrNewAddition(existing, raw)
		if err != nil {
			return nil, err
		}
		if slices.Contains(resolved, reference) {
			continue
		}
		resolved = append(resolved, reference)
	}
	return models.CloneTaskReferences(resolved), nil
}
