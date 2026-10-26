package app

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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
	if guardErr := requireChecklistOnlyChild(snap, referenceDocuments, child); guardErr != nil {
		return ChecklistDemotionResult{}, guardErr
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

	before := rollbackSnapshot(snap)
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
	commits[0].preconditions = checklistDemotionPreconditions(snap, referenceDocuments, census)
	awaitTaskCommitBarrier(child, snap.Revision(child.Project.ID), checklistDemotionUndoAction)
	outcome, err := s.commitProjects(commits, checklistDemotionUndoAction, fmt.Sprintf("#%d %s", child.Task.ShortID, child.Task.Title))
	if err != nil {
		return ChecklistDemotionResult{}, err
	}
	committed = true
	return ChecklistDemotionResult{Parent: newTaskResult(TaskRef{Task: parent, Project: child.Project}, revisionAt(outcome.results, 0), transactionIDAt(outcome.results, 0)), Index: index}, nil
}

// checklistDemotionPreconditions guards the full read set rather than the one
// project the demotion writes, because an incoming reference can be added in any
// project: the snapshot's own projects, then any reference document outside it,
// then any project the catalog census named that neither has seen.
func checklistDemotionPreconditions(
	snap Snapshot, documents []ReferenceDocument, census WorkspaceCensus,
) []storage.ProjectRevisionPrecondition {
	preconditions := make([]storage.ProjectRevisionPrecondition, 0, len(snap.Projects()))
	guarded := make(map[string]bool)
	for _, project := range snap.Projects() {
		preconditions = append(preconditions, storage.ProjectRevisionPrecondition{
			ProjectID: project.ID, ProjectName: project.Name, Path: project.Path,
			ExpectedRevision: snap.Revision(project.ID),
		})
		guarded[project.ID] = true
	}
	for _, document := range documents {
		if document.Project == nil || guarded[document.Project.ID] {
			continue
		}
		preconditions = append(preconditions, storage.ProjectRevisionPrecondition{
			ProjectID: document.Project.ID, ProjectName: document.Project.Name, Path: document.Project.Path,
			ExpectedRevision: document.Revision,
		})
		guarded[document.Project.ID] = true
	}
	for _, entry := range census.Projects {
		if entry.Project == nil || guarded[entry.Project.ID] {
			continue
		}
		preconditions = append(preconditions, storage.ProjectRevisionPrecondition{
			ProjectID: entry.Project.ID, ProjectName: entry.Project.Name, Path: entry.Project.Path,
			ExpectedRevision: entry.Revision,
		})
	}
	return preconditions
}

// childCarriesUnretainableContent answers the whole loss question in one place.
// A checklist item stores a title and a tick, so every other field a task can
// carry is content the demotion would drop silently. The check-in pair and the
// review instant are on this list for that reason: a scheduled child demoted
// without them would stop raising a reminder nobody withdrew, and the review
// history would vanish with no record that it had been answered.
func childCarriesUnretainableContent(task *models.Task) bool {
	return task.Description != "" || task.Priority != models.PriorityUnset ||
		len(task.Tags) != 0 || task.Context != "" || task.DeferCount != 0 || task.DueDate != nil ||
		task.Pinned || task.PinnedAt != nil || task.Review || task.Hidden || task.ResumeNote != "" ||
		len(task.Comments) != 0 || len(task.Attachments) != 0 || task.DeferredUntil != nil ||
		!task.CheckIn().IsEmpty() || task.LastReviewedAt != nil ||
		len(task.Checklist) != 0 || len(task.Dependencies) != 0
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
	if childCarriesUnretainableContent(task) {
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
