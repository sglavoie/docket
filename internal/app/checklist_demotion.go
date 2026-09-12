package app

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const checklistDemotionUndoAction = "Make Checklist Item"

// ChecklistDemotionRequest names a child and freezes all of its persisted row
// values. A stale gesture cannot silently consume a task that changed meanwhile.
type ChecklistDemotionRequest struct {
	Child          TaskLocator
	FrozenTaskJSON string
}

type ChecklistDemotionResult struct {
	Parent TaskResult
	Index  int
}

func (s *Service) MakeChecklistItem(req ChecklistDemotionRequest) (ChecklistDemotionResult, error) {
	if !req.Child.Valid() || req.FrozenTaskJSON == "" {
		return ChecklistDemotionResult{}, invalidInput("task", "child identity and frozen row are required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	child, err := resolveTaskSelection(snap, "", req.Child)
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	frozen, err := json.Marshal(child.Task)
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	if string(frozen) != req.FrozenTaskJSON {
		return ChecklistDemotionResult{}, invalidInput("task", "the child changed since it was selected")
	}
	if child.Task.ParentID == nil {
		return ChecklistDemotionResult{}, invalidInput("task", "only a child task can become a checklist item")
	}
	parent := child.Project.GetTaskByID(*child.Task.ParentID)
	if parent == nil {
		return ChecklistDemotionResult{}, invalidInput("task", "the parent is no longer available")
	}
	// A catalog revision guards the population of possible incoming reference
	// holders, including a project created while this conversion is in flight.
	var census WorkspaceCensus
	if source, ok := s.repo.(WorkspaceCensusSource); ok {
		census, err = source.WorkspaceCensus()
		if err != nil {
			return ChecklistDemotionResult{}, err
		}
	}
	referenceDocuments, err := s.referenceDocuments()
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	if err := requireChecklistOnlyChild(snap, referenceDocuments, child); err != nil {
		return ChecklistDemotionResult{}, err
	}
	if s.planning == nil {
		return ChecklistDemotionResult{}, invalidInput("planning", "planning state is unavailable")
	}
	artifacts, err := s.planning(PlanningReconcileRequest{RequireAbsent: &req.Child, GuardOnly: true})
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	if len(artifacts) == 0 {
		return ChecklistDemotionResult{}, invalidInput("planning", "planning revision guard is unavailable")
	}

	before, err := cloneSnapshot(snap)
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	index := len(parent.Checklist)
	if origin := child.Task.ChecklistOrigin; origin != nil && origin.ProjectID == child.Project.ID && origin.TaskID == parent.ID {
		index = min(max(origin.Index, 0), index)
	}
	item := models.ChecklistItem{Text: child.Task.Title, Done: child.Task.State() == models.StateDone}
	parent.Checklist = slices.Insert(slices.Clone(parent.Checklist), index, item)
	parent.UpdatedAt = s.clock.Now()
	for i, task := range child.Project.Tasks {
		if task.ID == child.Task.ID {
			child.Project.Tasks = slices.Delete(child.Project.Tasks, i, i+1)
			break
		}
	}
	child.Project.BuildTaskTree()
	commits := []projectCommit{{project: child.Project, expectedRevision: snap.Revision(child.Project.ID), base: snap.Document(child.Project.ID), artifacts: artifacts}}
	if census.CatalogPath != "" {
		commits[0].artifactPreconditions = append(commits[0].artifactPreconditions, storage.ArtifactPrecondition{
			Path: census.CatalogPath, ExpectedRevision: census.CatalogRevision,
		})
	}
	// An incoming reference can be added in any project. Guard the full read set,
	// including projects with no document changes of their own.
	guarded := make(map[string]bool)
	for _, project := range snap.Projects() {
		commits[0].preconditions = append(commits[0].preconditions, storage.ProjectRevisionPrecondition{
			ProjectID: project.ID, ProjectName: project.Name, Path: project.Path,
			ExpectedRevision: snap.Revision(project.ID),
		})
		guarded[project.ID] = true
	}
	for _, document := range referenceDocuments {
		if document.Project == nil || guarded[document.Project.ID] {
			continue
		}
		commits[0].preconditions = append(commits[0].preconditions, storage.ProjectRevisionPrecondition{
			ProjectID: document.Project.ID, ProjectName: document.Project.Name, Path: document.Project.Path,
			ExpectedRevision: document.Revision,
		})
		guarded[document.Project.ID] = true
	}
	for _, entry := range census.Projects {
		if entry.Project == nil || guarded[entry.Project.ID] {
			continue
		}
		commits[0].preconditions = append(commits[0].preconditions, storage.ProjectRevisionPrecondition{
			ProjectID: entry.Project.ID, ProjectName: entry.Project.Name, Path: entry.Project.Path,
			ExpectedRevision: entry.Revision,
		})
	}
	awaitTaskCommitBarrier(child, snap.Revision(child.Project.ID), checklistDemotionUndoAction)
	outcome, err := s.commitProjects(commits, checklistDemotionUndoAction, fmt.Sprintf("#%d %s", child.Task.ShortID, child.Task.Title))
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	committed = true
	return ChecklistDemotionResult{Parent: newTaskResult(TaskRef{Task: parent, Project: child.Project}, revisionAt(outcome.results, 0), transactionIDAt(outcome.results, 0)), Index: index}, nil
}

func requireChecklistOnlyChild(snap Snapshot, documents []ReferenceDocument, child TaskRef) error {
	task := child.Task
	if task.State() != models.StateBacklog && task.State() != models.StateDone {
		return invalidInput("state", "only Backlog or Done children can become checklist items")
	}
	for _, candidate := range child.Project.Tasks {
		if candidate.ParentID != nil && *candidate.ParentID == task.ID {
			return invalidInput("task", "a child with descendants cannot become a checklist item")
		}
	}
	if task.Description != "" || task.Priority != models.PriorityUnset ||
		len(task.Tags) != 0 || task.Context != "" || task.DeferCount != 0 || task.DueDate != nil ||
		task.Pinned || task.PinnedAt != nil || task.Review || task.ResumeNote != "" ||
		len(task.Comments) != 0 || len(task.Attachments) != 0 || task.DeferredUntil != nil ||
		len(task.Checklist) != 0 || len(task.Dependencies) != 0 {
		return invalidInput("task", "child has content a checklist item cannot retain")
	}
	projects := append([]*models.Project(nil), snap.Projects()...)
	seen := make(map[string]bool, len(projects))
	for _, project := range projects {
		seen[project.ID] = true
	}
	for _, document := range documents {
		if document.Project != nil && !seen[document.Project.ID] {
			projects = append(projects, document.Project)
			seen[document.Project.ID] = true
		}
	}
	for _, project := range projects {
		for _, source := range project.Tasks {
			if source.ID == task.ID && project.ID == child.Project.ID {
				continue
			}
			for _, dependency := range source.Dependencies {
				if dependency.ProjectID == child.Project.ID && dependency.TaskID == task.ID {
					return invalidInput("task", "another task references this child")
				}
			}
		}
	}
	return nil
}
