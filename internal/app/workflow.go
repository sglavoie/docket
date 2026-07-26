package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// WorkflowConflict is a presentation-neutral description of two or more rules
// competing to set the same field on the same task. The conflicting field is left
// unchanged; the adapter surfaces this to the user.
type WorkflowConflict struct {
	ProjectID string
	TaskID    string
	TaskTitle string
	Field     string
	Rules     []string
}

// WorkflowMove records a task relocated to another project by a move_to_project
// rule action, executed as part of the cascade.
type WorkflowMove struct {
	SourceProjectID string
	TaskID          string
	DestProjectID   string
}

// WorkflowResult reports the outcome of a workflow cascade executed as one
// transaction: which tasks the rules modified, which follow-up tasks they
// created, which cross-project moves they triggered, any field conflicts, and the
// new revision of every project the transaction touched.
//
// ProjectStatusChanges reports catalog effects already committed through a
// revision-checked artifact in the same transaction.
// RuleTriggerCounts remain surfaced for the adapter's best-effort automation
// counter update.
type WorkflowResult struct {
	ProjectID            string
	ModifiedTaskIDs      []string
	FollowupTaskIDs      []string
	Moves                []WorkflowMove
	Conflicts            []WorkflowConflict
	Revisions            map[string]string
	ProjectStatusChanges []WorkflowProjectStatusChange
	RuleTriggerCounts    map[string]int
}

// ApplyWorkflows evaluates task- and project-level workflow rules and executes
// the resulting cascade — field mutations, follow-ups, catalog changes,
// and cross-project moves — as ONE transaction. Rule evaluation itself never
// touches storage: the engine produces proposed actions which this use case turns
// into project ChangeSets and checked artifacts committed together.
//
// prevStates (taskID -> pre-change snapshot) enables event-based triggers; pass
// nil for a stateless evaluation. When no rule fires, nothing is committed and the
// result reports the project's unchanged revision.
func (s *Service) ApplyWorkflows(
	projectID string,
	rules []workflow.WorkflowRule,
	prevStates map[string]*models.Task,
) (WorkflowResult, error) {
	return s.ApplyWorkflowsWithArtifacts(projectID, rules, prevStates, WorkflowArtifactBuilders{})
}

// ApplyWorkflowsWithArtifacts is ApplyWorkflows with a checked catalog-artifact
// builder available so task and project-status effects share one transaction.
func (s *Service) ApplyWorkflowsWithArtifacts(
	projectID string,
	rules []workflow.WorkflowRule,
	prevStates map[string]*models.Task,
	builders WorkflowArtifactBuilders,
) (WorkflowResult, error) {
	var project *models.Project
	orchestrated, err := s.runWorkflowOrchestration(workflowOrchestrationRequest{
		rules:                  rules,
		projectStatusArtifacts: builders.ProjectStatus,
		planningArtifacts:      builders.Planning,
		undoAction:             "Apply Workflow Rules",
		prepare: func(snap Snapshot) (workflowMutationPlan, error) {
			project = snap.ProjectByID(projectID)
			if project == nil {
				return workflowMutationPlan{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
			}
			plan := workflowMutationPlan{
				commits: []projectCommit{{
					project:          project,
					expectedRevision: snap.Revision(project.ID),
					base:             snap.Document(project.ID),
				}},
				sources:     []*models.Project{project},
				undoContext: project.Name,
			}
			if prevStates != nil {
				plan.previousStates = map[string]map[string]*models.Task{project.ID: prevStates}
			}
			return plan, nil
		},
	})
	if err != nil {
		return WorkflowResult{}, err
	}

	out := WorkflowResult{
		ProjectID:            project.ID,
		ModifiedTaskIDs:      orchestrated.modifiedTaskIDs,
		FollowupTaskIDs:      orchestrated.followupTaskIDs,
		Moves:                orchestrated.moves,
		Conflicts:            orchestrated.conflicts,
		Revisions:            make(map[string]string, len(orchestrated.commits)),
		ProjectStatusChanges: orchestrated.projectStatusChanges,
		RuleTriggerCounts:    orchestrated.ruleTriggerCounts,
	}
	for i, commit := range orchestrated.commits {
		if orchestrated.committed {
			out.Revisions[commit.project.ID] = revisionAt(orchestrated.commitOutcome.results, i)
		} else {
			out.Revisions[commit.project.ID] = commit.expectedRevision
		}
	}
	return out, nil
}

// resolveWorkflowMoveDest resolves a move_to_project destination — recorded by the
// engine as the rule's raw value — to a project in the snapshot. The value is a
// project name in the common case, but an ID or slug is accepted too so a rule
// authored against any of them still targets the right project.
func resolveWorkflowMoveDest(snap Snapshot, dest string) *models.Project {
	if p := snap.ProjectByID(dest); p != nil {
		return p
	}
	for _, p := range snap.Projects() {
		if strings.EqualFold(p.Name, dest) || strings.EqualFold(p.Slug, dest) {
			return p
		}
	}
	return nil
}
