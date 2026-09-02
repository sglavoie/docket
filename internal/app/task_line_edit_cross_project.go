package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (s *Service) editTaskLineAcrossProjects(snap Snapshot, sourceRef TaskRef, prepared preparedTaskLineEdit, builder PlanningArtifactBuilder) (TaskLineEditResult, error) {
	destination := prepared.destinationProject
	backups := []hierarchyProjectBackup{
		captureHierarchyProject(sourceRef.Project),
		captureHierarchyProject(destination),
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for i := len(backups) - 1; i >= 0; i-- {
			backups[i].restore()
		}
	}()

	applyPreparedTaskLineFields(sourceRef.Task, prepared, s.clock.Now())

	var destinationParent *models.Task
	if prepared.parentMode == TaskLineParentSet {
		destinationParent = prepared.parent
	}
	adopting := len(prepared.children) > 0
	moveParent := destinationParent
	if adopting {
		// The one structural batch below owns the parent edge so the new parent
		// and every adopted child are validated as a single hierarchy.
		moveParent = nil
	}
	// The subtree lands first, so an adopted child is judged against the task set
	// the destination actually ends up with.
	move, err := moveTaskSubtreeBetweenProjects(sourceRef.Project, sourceRef.Task, destination, moveParent)
	if err != nil {
		return TaskLineEditResult{}, invalidInput("move", err.Error())
	}
	if _, edgeErr := applyTaskLineEditEdges(destination, sourceRef.Task, destinationParent, adopting && destinationParent != nil, prepared.children); edgeErr != nil {
		return TaskLineEditResult{}, edgeErr
	}

	finalRef := TaskRef{Task: sourceRef.Task, Project: destination, Base: snap.Document(destination.ID)}
	if guardErr := taskLineDestinationChildrenGuard(finalRef, prepared.destination); guardErr != nil {
		return TaskLineEditResult{}, guardErr
	}
	planningTargets, _, err := s.applyTaskLineDestination(finalRef, prepared.destination)
	if err != nil {
		return TaskLineEditResult{}, err
	}
	if _, deferErr := applyPreparedTaskLineDeferral(sourceRef.Task, prepared, s.clock.Now()); deferErr != nil {
		return TaskLineEditResult{}, deferErr
	}
	artifacts, err := taskLineCrossProjectPlanningArtifacts(
		planningTargets,
		move.PlanningMoves,
		s.defaultedPlanningArtifacts(builder),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	// Every moved task keeps its identity and changes address, so each incoming
	// reference across the whole catalog is re-pointed in this same transaction.
	// References the moved subtree holds among itself are re-pointed by the same
	// map; references it holds elsewhere are untouched.
	maintenance := TaskReferenceMaintenance{Moved: make(map[models.TaskReference]models.TaskReference, len(move.MovedTasks))}
	for _, moved := range move.MovedTasks {
		if moved == nil || moved.ID == "" {
			continue
		}
		maintenance.Moved[models.TaskReference{ProjectID: sourceRef.Project.ID, TaskID: moved.ID}] =
			models.TaskReference{ProjectID: destination.ID, TaskID: moved.ID}
	}
	pass, err := s.maintainTaskReferences(snap, maintenance, map[string]bool{
		sourceRef.Project.ID: true,
		destination.ID:       true,
	})
	if err != nil {
		return TaskLineEditResult{}, err
	}
	defer func() {
		if !committed {
			pass.restore()
		}
	}()

	commits := []projectCommit{
		{
			project:          sourceRef.Project,
			expectedRevision: snap.Revision(sourceRef.Project.ID),
			base:             sourceRef.Base,
			artifacts:        artifacts,
			attachmentOps:    move.AttachmentOps,
		},
		{
			project:          destination,
			expectedRevision: snap.Revision(destination.ID),
			base:             snap.Document(destination.ID),
		},
	}
	commits = append(commits, pass.commits...)
	outcome, err := s.commitProjects(
		commits,
		prepared.action,
		fmt.Sprintf("#%d %s → %s", finalRef.Task.ShortID, finalRef.Task.Title, destination.Slug),
	)
	if err != nil {
		return TaskLineEditResult{}, err
	}

	result := taskLineEditResult(
		finalRef,
		transactionIDAt(outcome.results, 0),
		true,
	)
	result.MovedDescendantCount = max(0, len(move.MovedTasks)-1)
	result.AffectedProjectIDs = []string{sourceRef.Project.ID, destination.ID}
	committed = true
	return result, nil
}

func taskLineCrossProjectPlanningArtifacts(targets []PlanningReconcileTarget, moves []PlanningProjectMove, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if builder == nil {
		for _, target := range targets {
			if target.Today == PlanningTodayAdd || target.Next == PlanningNextAdd {
				return nil, invalidInput("destination", "planning persistence is unavailable")
			}
		}
		return nil, nil
	}
	return planningArtifactsForRequest(PlanningReconcileRequest{Targets: targets, Moves: moves}, builder)
}
