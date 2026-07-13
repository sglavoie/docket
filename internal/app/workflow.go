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
	TaskID    string
	TaskTitle string
	Field     string
	Rules     []string
}

// WorkflowMove records a task relocated to another project by a move_to_project
// rule action, executed as part of the cascade.
type WorkflowMove struct {
	TaskID        string
	DestProjectID string
}

// WorkflowResult reports the outcome of a workflow cascade executed as one
// transaction: which tasks the rules modified, which follow-up tasks they
// created, which cross-project moves they triggered, any field conflicts, and the
// new revision of every project the transaction touched.
type WorkflowResult struct {
	ProjectID       string
	ModifiedTaskIDs []string
	FollowupTaskIDs []string
	Moves           []WorkflowMove
	Conflicts       []WorkflowConflict
	Revisions       map[string]string
}

// ApplyWorkflows evaluates the given task-level workflow rules against a project
// and executes the resulting cascade — field mutations, created follow-up tasks,
// and cross-project moves — as ONE transaction. Rule evaluation itself never
// touches storage: the engine produces proposed application actions (the modified
// task pointers, follow-ups, and deferred moves) which this use case turns into
// ChangeSets committed together, so a cascade that spans projects is atomic.
//
// prevStates (taskID -> pre-change snapshot) enables event-based triggers; pass
// nil for a stateless evaluation. When no rule fires, nothing is committed and the
// result reports the project's unchanged revision.
func (s *Service) ApplyWorkflows(projectID string, rules []workflow.WorkflowRule, prevStates map[string]*models.Task) (WorkflowResult, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return WorkflowResult{}, err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return WorkflowResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	var result *workflow.EvaluationResult
	if prevStates != nil {
		evalCtx := workflow.BuildEvaluationContext(project.Tasks, prevStates, project.Name, project.ID)
		result = workflow.EvaluateRulesWithContext(evalCtx, rules)
	} else {
		result = workflow.EvaluateRules(project.Tasks, rules)
	}

	out := WorkflowResult{ProjectID: project.ID, Revisions: map[string]string{}}
	for _, c := range result.Conflicts {
		out.Conflicts = append(out.Conflicts, WorkflowConflict{
			TaskID:    c.TaskID,
			TaskTitle: c.TaskTitle,
			Field:     c.Field,
			Rules:     c.Rules,
		})
	}
	for _, t := range result.ModifiedTasks {
		out.ModifiedTaskIDs = append(out.ModifiedTaskIDs, t.ID)
	}

	// Append created follow-up tasks to the project, assigning fresh short IDs.
	for _, followup := range result.NewFollowupTasks {
		followup.ShortID = project.GetNextShortID()
		project.Tasks = append(project.Tasks, followup)
		out.FollowupTaskIDs = append(out.FollowupTaskIDs, followup.ID)
	}

	// A cascade that changed nothing (or only detected conflicts) must not commit
	// a no-op revision bump.
	if len(result.ModifiedTasks) == 0 && len(result.NewFollowupTasks) == 0 && len(result.DeferredMoves) == 0 {
		out.Revisions[project.ID] = snap.Revision(project.ID)
		return out, nil
	}

	commits := []projectCommit{{project: project, expectedRevision: snap.Revision(project.ID)}}
	destSeen := map[string]bool{}

	// Execute deferred cross-project moves inside the same transaction.
	for _, move := range result.DeferredMoves {
		task := project.GetTaskByID(move.TaskID)
		if task == nil {
			continue
		}
		dest := resolveWorkflowMoveDest(snap, move.Destination)
		if dest == nil || dest.ID == project.ID {
			continue
		}
		if _, err := project.MoveTaskToProject(task, dest); err != nil {
			return WorkflowResult{}, fmt.Errorf("workflow move_to_project failed: %w", err)
		}
		if !destSeen[dest.ID] {
			destSeen[dest.ID] = true
			commits = append(commits, projectCommit{project: dest, expectedRevision: snap.Revision(dest.ID)})
		}
		out.Moves = append(out.Moves, WorkflowMove{TaskID: move.TaskID, DestProjectID: dest.ID})
	}

	results, err := s.commitProjects(commits, "Apply Workflow Rules", project.Name)
	if err != nil {
		return WorkflowResult{}, err
	}
	for i, c := range commits {
		out.Revisions[c.project.ID] = revisionAt(results, i)
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
