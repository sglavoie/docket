package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

type preparedTaskLineEdit struct {
	title              string
	description        *string
	resumeNote         *string
	priority           *int
	dueSet             bool
	due                *time.Time
	deferSet           bool
	deferUntil         *time.Time
	context            *string
	pinned             *bool
	review             *bool
	tags               []string
	destination        *CaptureDestination
	parentMode         TaskLineParentMode
	parent             *models.Task
	parentChanged      bool
	children           []*models.Task
	destinationProject *models.Project
	action             string
	// dependencies is the complete post-patch reference set, resolved before any
	// mutation. dependenciesSet distinguishes "store this set" from "the request
	// said nothing about dependencies".
	dependencies    []models.TaskReference
	dependenciesSet bool
}

// prepareTaskLineScalars copies the preserve-on-nil scalar fields, which are the
// ones needing no snapshot lookup.
func prepareTaskLineScalars(prepared *preparedTaskLineEdit, req TaskLineEditRequest) error {
	if req.Description != nil {
		description := *req.Description
		prepared.description = &description
	}
	if req.ResumeNote != nil {
		if err := models.ValidateResumeNote(*req.ResumeNote); err != nil {
			return invalidInput("resume note", err.Error())
		}
		note := *req.ResumeNote
		prepared.resumeNote = &note
	}
	if req.Priority != nil {
		if err := models.ValidatePriority(*req.Priority, true); err != nil {
			return invalidInput("priority", err.Error())
		}
		priority := *req.Priority
		prepared.priority = &priority
	}
	if req.Pinned != nil {
		pinned := *req.Pinned
		prepared.pinned = &pinned
	}
	if req.Review != nil {
		review := *req.Review
		prepared.review = &review
	}
	if req.Context != nil {
		context := ""
		if *req.Context != "" {
			normalized, err := models.NormalizeTaskContext(*req.Context)
			if err != nil {
				return invalidInput("context", err.Error())
			}
			context = normalized
		}
		prepared.context = &context
	}
	return nil
}

// prepareTaskLineDeferral routes an inline deferral through deferTask's policy.
// The refusal is measured against the state this edit produces, so a line that
// reopens and defers in one breath is judged on its result rather than on the
// terminal state it replaced.
func prepareTaskLineDeferral(prepared *preparedTaskLineEdit, ref TaskRef, req TaskLineEditRequest) error {
	if !req.DeferSet {
		return nil
	}
	prepared.deferSet = true
	if req.Defer == nil {
		return nil
	}
	prepared.deferUntil = cloneInstantPtr(req.Defer)
	state := ref.Task.State()
	if prepared.destination != nil {
		state = prepared.destination.WorkflowState()
	}
	if err := deferralRefusesState(state); err != nil {
		return err
	}
	backlog := CaptureDestinationBacklog
	prepared.destination = &backlog
	return nil
}

func prepareTaskLineEdit(snap Snapshot, ref TaskRef, req TaskLineEditRequest) (preparedTaskLineEdit, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return preparedTaskLineEdit{}, invalidInput("title", err.Error())
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		action = "Edit Task Line"
	}
	prepared := preparedTaskLineEdit{
		title:              title,
		dueSet:             req.DueSet,
		due:                cloneDueDatePtr(req.Due),
		parentMode:         req.Parent.Mode,
		destinationProject: ref.Project,
		action:             action,
	}
	if err := prepareTaskLineScalars(&prepared, req); err != nil {
		return preparedTaskLineEdit{}, err
	}
	if err := prepareTaskLineDeferral(&prepared, ref, req); err != nil {
		return preparedTaskLineEdit{}, err
	}

	add, err := normalizeTagList(req.TagsToAdd)
	if err != nil {
		return preparedTaskLineEdit{}, err
	}
	remove, err := normalizeTagList(req.TagsToRemove)
	if err != nil {
		return preparedTaskLineEdit{}, err
	}
	for _, tag := range add {
		if slices.Contains(remove, tag) {
			return preparedTaskLineEdit{}, invalidInput("tag", fmt.Sprintf("cannot add and remove %q in one edit", tag))
		}
	}
	prepared.tags = patchedTaskLineTags(ref.Task.Tags, add, remove)

	effectiveProjectID := ref.Project.ID
	if projectID := strings.TrimSpace(req.DestinationProjectID); projectID != "" {
		destinationProject := snap.ProjectByID(projectID)
		if destinationProject == nil {
			return preparedTaskLineEdit{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
		}
		prepared.destinationProject = destinationProject
		effectiveProjectID = projectID
	}

	switch req.Parent.Mode {
	case TaskLineParentPreserve:
		if req.Parent.Parent.ProjectID != "" || req.Parent.Parent.TaskID != "" {
			return preparedTaskLineEdit{}, invalidInput("parent", "preserve mode cannot include a parent locator")
		}
	case TaskLineParentRoot:
		if req.Parent.Parent.ProjectID != "" || req.Parent.Parent.TaskID != "" {
			return preparedTaskLineEdit{}, invalidInput("parent", "root mode cannot include a parent locator")
		}
		prepared.parentChanged = effectiveProjectID != ref.Project.ID || ref.Task.ParentID != nil
	case TaskLineParentSet:
		parentRef, resolveErr := resolveTaskSelection(snap, "", req.Parent.Parent)
		if resolveErr != nil {
			return preparedTaskLineEdit{}, resolveErr
		}
		if parentRef.Project.ID != effectiveProjectID {
			return preparedTaskLineEdit{}, invalidInput("parent", "parent must belong to the effective project")
		}
		prepared.parent = parentRef.Task
		prepared.parentChanged = effectiveProjectID != ref.Project.ID || ref.Task.ParentID == nil || *ref.Task.ParentID != parentRef.Task.ID
	default:
		return preparedTaskLineEdit{}, invalidInput("parent", fmt.Sprintf("unknown parent mode %d", req.Parent.Mode))
	}

	children, err := resolveTaskLineChildren(snap, req.Children, effectiveProjectID)
	if err != nil {
		return preparedTaskLineEdit{}, err
	}
	prepared.children = children

	if !req.Dependencies.IsZero() {
		resolver := newReferenceResolver(snap, TaskLocator{ProjectID: effectiveProjectID, TaskID: ref.Task.ID})
		dependencies, depErr := resolver.resolveDependencyPatch(ref.Task.Dependencies, req.Dependencies)
		if depErr != nil {
			return preparedTaskLineEdit{}, depErr
		}
		prepared.dependencies = dependencies
		prepared.dependenciesSet = true
	}

	if req.Destination != nil {
		destination, ok := ParseCaptureDestination(string(*req.Destination))
		if !ok {
			return preparedTaskLineEdit{}, invalidInput("destination", InvalidCaptureDestinationError(string(*req.Destination)).Error())
		}
		prepared.destination = &destination
		if err := validateTaskLineDestination(ref, destination); err != nil {
			return preparedTaskLineEdit{}, err
		}
	}
	return prepared, nil
}

// resolveTaskLineChildren resolves every adopted child against the same fresh
// snapshot the rest of the request is resolved from. A child belongs to the
// effective project — the destination when the edit crosses a boundary — so a
// referenced subtree is never dragged between projects by an adoption. Repeated
// locators collapse to their first occurrence. Depth and cycles are deliberately
// left to the single hierarchy patch, which measures them on the final graph.
func resolveTaskLineChildren(snap Snapshot, locators []TaskLocator, effectiveProjectID string) ([]*models.Task, error) {
	if len(locators) == 0 {
		return nil, nil
	}
	children := make([]*models.Task, 0, len(locators))
	seen := make(map[string]bool, len(locators))
	for _, locator := range locators {
		childRef, err := resolveTaskSelection(snap, "", locator)
		if err != nil {
			return nil, err
		}
		if childRef.Project.ID != effectiveProjectID {
			return nil, invalidInput("child", "child must belong to the effective project")
		}
		if seen[childRef.Task.ID] {
			continue
		}
		seen[childRef.Task.ID] = true
		children = append(children, childRef.Task)
	}
	return children, nil
}
