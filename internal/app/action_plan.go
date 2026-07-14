package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/actionplan"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ActionPlanVersion is the only structured AI action-plan version this build
// accepts. Provider output with any other version fails closed before preview or
// mutation.
const ActionPlanVersion = actionplan.Version

// ActionType is the allowlisted tagged union discriminator for AI mutations.
// There is intentionally no shell-command or arbitrary executable action.
type ActionType = actionplan.ActionType

const (
	ActionCreateTask       = actionplan.ActionCreateTask
	ActionEditTask         = actionplan.ActionEditTask
	ActionSetState         = actionplan.ActionSetState
	ActionSetPriority      = actionplan.ActionSetPriority
	ActionSetDueDate       = actionplan.ActionSetDueDate
	ActionClearDueDate     = actionplan.ActionClearDueDate
	ActionSetTags          = actionplan.ActionSetTags
	ActionAddDependency    = actionplan.ActionAddDependency
	ActionRemoveDependency = actionplan.ActionRemoveDependency
	ActionDeleteTask       = actionplan.ActionDeleteTask
)

// Action is one member of the allowlisted action union. Pointer fields preserve
// the distinction between an omitted property and an explicitly empty value.
// Validation rejects fields that do not belong to the selected Type.
type Action = actionplan.Action

// ActionPlan is the complete provider response. Summary is display-only; all
// executable meaning comes from the validated Actions union.
type ActionPlan = actionplan.ActionPlan

// ActionRisk is computed locally. Provider output cannot lower or override it.
type ActionRisk string

const (
	ActionRiskSafe        ActionRisk = "safe"
	ActionRiskDestructive ActionRisk = "destructive"
)

// ActionPreview is one exact, locally resolved action shown before confirmation.
type ActionPreview struct {
	Index       int        `json:"index"`
	Type        ActionType `json:"type"`
	Risk        ActionRisk `json:"risk"`
	Description string     `json:"description"`
}

// PlanPreview is the confirmation-ready local interpretation of an ActionPlan.
type PlanPreview struct {
	Summary          string          `json:"summary"`
	Risk             ActionRisk      `json:"risk"`
	SafeCount        int             `json:"safe_count"`
	DestructiveCount int             `json:"destructive_count"`
	Actions          []ActionPreview `json:"actions"`
}

// ApplyActionPlanOptions records the two independent confirmation gates. Safe
// actions require the batch confirmation; destructive actions require that plus
// a distinct destructive confirmation.
type ApplyActionPlanOptions struct {
	ConfirmSafe        bool
	ConfirmDestructive bool
	Artifacts          []storage.ArtifactChange
	// DeletedTaskArtifacts is called with the complete, sorted set of task IDs
	// removed by accepted delete actions, including every subtree descendant. The
	// callback runs after the plan has been resolved against the same fresh
	// snapshot the commit uses and before any mutation is persisted, allowing an
	// adapter to fold durable planning-reference cleanup into the transaction.
	DeletedTaskArtifacts func([]string) ([]storage.ArtifactChange, error)
}

// ActionPlanResult reports the single application transaction. UndoID equals
// TransactionID because the workspace ledger keys its entry by the committed
// transaction manifest ID.
type ActionPlanResult struct {
	TransactionID   string          `json:"transaction_id"`
	UndoID          string          `json:"undo_id"`
	TouchedProjects []string        `json:"touched_projects"`
	DeletedTaskIDs  []string        `json:"deleted_task_ids,omitempty"`
	Actions         []ActionPreview `json:"actions"`
}

var (
	// ErrActionPlanConfirmationRequired is returned before any snapshot mutation
	// when the caller did not record the safe batch confirmation.
	ErrActionPlanConfirmationRequired = errors.New("app: AI action plan requires batch confirmation")
	// ErrActionPlanDestructiveConfirmationRequired is the separate escalation
	// gate for locally classified destructive actions.
	ErrActionPlanDestructiveConfirmationRequired = errors.New("app: AI action plan contains destructive actions requiring separate confirmation")
)

// ValidateActionPlan validates the version, tagged union, required properties,
// and value syntax without consulting workspace state.
func ValidateActionPlan(plan ActionPlan) error {
	if err := actionplan.Validate(plan); err != nil {
		return invalidInput("action plan", err.Error())
	}
	return nil
}

// RiskForAction classifies an action locally. Terminal cancellation and task
// deletion are destructive regardless of any provider-authored prose.
func RiskForAction(action Action) ActionRisk {
	if action.Type == ActionDeleteTask {
		return ActionRiskDestructive
	}
	if action.Type == ActionSetState && action.State != nil && *action.State == models.StateCanceled {
		return ActionRiskDestructive
	}
	return ActionRiskSafe
}

// FilterActionPlan returns a plan containing only actions of the requested risk
// tier. It is used when a user accepts the safe batch but declines the separate
// destructive escalation.
func FilterActionPlan(plan ActionPlan, risk ActionRisk) ActionPlan {
	filtered := ActionPlan{Version: plan.Version, Summary: plan.Summary}
	for _, action := range plan.Actions {
		if RiskForAction(action) == risk {
			filtered.Actions = append(filtered.Actions, action)
		}
	}
	return filtered
}

// PreviewActionPlan resolves every target against one fresh workspace snapshot
// and returns the exact operations and locally computed risk shown to the user.
// It performs no mutation.
func (s *Service) PreviewActionPlan(plan ActionPlan) (PlanPreview, error) {
	if err := ValidateActionPlan(plan); err != nil {
		return PlanPreview{}, err
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return PlanPreview{}, err
	}
	preview := PlanPreview{Summary: strings.TrimSpace(plan.Summary), Risk: ActionRiskSafe}
	for i, action := range plan.Actions {
		description, err := describeAction(snap, action)
		if err != nil {
			return PlanPreview{}, fmt.Errorf("action %d: %w", i+1, err)
		}
		risk := RiskForAction(action)
		if risk == ActionRiskDestructive {
			preview.Risk = ActionRiskDestructive
			preview.DestructiveCount++
		} else {
			preview.SafeCount++
		}
		preview.Actions = append(preview.Actions, ActionPreview{Index: i + 1, Type: action.Type, Risk: risk, Description: description})
	}
	return preview, nil
}

func describeAction(snap Snapshot, action Action) (string, error) {
	projectName := func(id string) (string, error) {
		project := snap.ProjectByID(id)
		if project == nil {
			return "", fmt.Errorf("%w: %s", ErrProjectNotFound, id)
		}
		return project.Name, nil
	}
	taskLabel := func(id string) (string, TaskRef, error) {
		ref, ok := snap.Lookup(id)
		if !ok {
			return "", TaskRef{}, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
		}
		return fmt.Sprintf("%s:#%d %q", ref.Project.Slug, ref.Task.ShortID, ref.Task.Title), ref, nil
	}

	switch action.Type {
	case ActionCreateTask:
		name, err := projectName(action.ProjectID)
		if err != nil {
			return "", err
		}
		taskType := models.TaskTypeTask
		if action.TaskType != nil {
			taskType = *action.TaskType
		}
		priority := models.PriorityDefault
		if action.Priority != nil {
			priority = *action.Priority
		}
		details := fmt.Sprintf("Create %s %q in %s with priority %d", taskType, strings.TrimSpace(*action.Title), name, priority)
		if action.Tags != nil {
			details += fmt.Sprintf(" and tags [%s]", strings.Join(action.Tags, ", "))
		}
		return details, nil
	case ActionEditTask:
		label, _, err := taskLabel(action.TaskID)
		if err != nil {
			return "", err
		}
		var changes []string
		if action.Title != nil {
			changes = append(changes, fmt.Sprintf("title=%q", strings.TrimSpace(*action.Title)))
		}
		if action.Description != nil {
			changes = append(changes, fmt.Sprintf("description=%q", *action.Description))
		}
		if action.TaskType != nil {
			changes = append(changes, fmt.Sprintf("type=%s", *action.TaskType))
		}
		return fmt.Sprintf("Edit %s: %s", label, strings.Join(changes, ", ")), nil
	case ActionSetState:
		label, _, err := taskLabel(action.TaskID)
		return fmt.Sprintf("Set %s to %s", label, *action.State), err
	case ActionSetPriority:
		label, _, err := taskLabel(action.TaskID)
		return fmt.Sprintf("Set %s priority to %d", label, *action.Priority), err
	case ActionSetDueDate:
		label, _, err := taskLabel(action.TaskID)
		return fmt.Sprintf("Set %s due date to %s", label, *action.Due), err
	case ActionClearDueDate:
		label, _, err := taskLabel(action.TaskID)
		return fmt.Sprintf("Clear %s due date", label), err
	case ActionSetTags:
		label, _, err := taskLabel(action.TaskID)
		return fmt.Sprintf("Set %s tags to [%s]", label, strings.Join(action.Tags, ", ")), err
	case ActionAddDependency, ActionRemoveDependency:
		label, _, err := taskLabel(action.TaskID)
		if err != nil {
			return "", err
		}
		blocker, _, err := taskLabel(action.BlockerTaskID)
		verb := "Add"
		if action.Type == ActionRemoveDependency {
			verb = "Remove"
		}
		return fmt.Sprintf("%s dependency %s → %s", verb, label, blocker), err
	case ActionDeleteTask:
		label, _, err := taskLabel(action.TaskID)
		return fmt.Sprintf("Delete %s and its subtree", label), err
	default:
		return "", invalidInput("action type", fmt.Sprintf("%q is not allowlisted", action.Type))
	}
}

// ApplyActionPlan validates, locally risk-gates, and executes all accepted
// actions as one application-layer transaction. A canceled context, invalid
// action, or revision conflict commits nothing.
//
//nolint:gocyclo // The explicit phases preserve the all-or-nothing transaction boundary.
func (s *Service) ApplyActionPlan(ctx context.Context, plan ActionPlan, opts ApplyActionPlanOptions) (ActionPlanResult, error) {
	if err := ValidateActionPlan(plan); err != nil {
		return ActionPlanResult{}, err
	}
	if len(plan.Actions) == 0 {
		return ActionPlanResult{}, nil
	}
	if !opts.ConfirmSafe {
		return ActionPlanResult{}, ErrActionPlanConfirmationRequired
	}
	for _, action := range plan.Actions {
		if RiskForAction(action) == ActionRiskDestructive && !opts.ConfirmDestructive {
			return ActionPlanResult{}, ErrActionPlanDestructiveConfirmationRequired
		}
	}
	if err := ctx.Err(); err != nil {
		return ActionPlanResult{}, err
	}

	original, snapshotErr := s.repo.Snapshot()
	if snapshotErr != nil {
		return ActionPlanResult{}, snapshotErr
	}
	working, cloneErr := cloneActionSnapshot(original)
	if cloneErr != nil {
		return ActionPlanResult{}, cloneErr
	}

	previews := make([]ActionPreview, 0, len(plan.Actions))
	for i, action := range plan.Actions {
		description, describeErr := describeAction(working, action)
		if describeErr != nil {
			return ActionPlanResult{}, fmt.Errorf("action %d: %w", i+1, describeErr)
		}
		previews = append(previews, ActionPreview{Index: i + 1, Type: action.Type, Risk: RiskForAction(action), Description: description})
	}

	deletedIDs, deleteRoots, deletionErr := plannedDeletions(working, plan.Actions)
	if deletionErr != nil {
		return ActionPlanResult{}, deletionErr
	}
	for i, action := range plan.Actions {
		if action.Type != ActionDeleteTask && deletedIDs[action.TaskID] {
			return ActionPlanResult{}, invalidInput("action plan", fmt.Sprintf("action %d targets a task deleted by the same plan", i+1))
		}
		if deletedIDs[action.BlockerTaskID] {
			return ActionPlanResult{}, invalidInput("action plan", fmt.Sprintf("action %d references a blocker deleted by the same plan", i+1))
		}
	}

	touched := make(map[string]bool)
	for i, action := range plan.Actions {
		if action.Type == ActionDeleteTask {
			continue
		}
		if applyErr := s.applyPlannedAction(working, action, touched); applyErr != nil {
			return ActionPlanResult{}, fmt.Errorf("action %d: %w", i+1, applyErr)
		}
		if contextErr := ctx.Err(); contextErr != nil {
			return ActionPlanResult{}, contextErr
		}
	}

	attachmentOps := make(map[string][]storage.AttachmentOp)
	if len(deleteRoots) > 0 {
		for _, root := range deleteRoots {
			project := root.Project
			for _, task := range tasksMatching(project.Tasks, collectSubtreeIDs(root.Task)) {
				attachmentOps[project.ID] = append(attachmentOps[project.ID], storage.AttachmentOp{
					Kind:          storage.AttachmentDelete,
					Task:          task,
					SourceProject: project,
				})
			}
			project.Tasks = filterOutTasks(project.Tasks, collectSubtreeIDs(root.Task))
			touched[project.ID] = true
		}
		for _, project := range working.Projects() {
			for _, root := range deleteRoots {
				if project.ID == root.Project.ID {
					continue
				}
				if clearCrossProjectRefs(project, root.Project, collectSubtreeIDs(root.Task)) > 0 {
					touched[project.ID] = true
				}
			}
		}
	}

	if contextErr := ctx.Err(); contextErr != nil {
		return ActionPlanResult{}, contextErr
	}
	deletedTaskIDs := sortedActionTaskIDs(deletedIDs)
	artifacts := append([]storage.ArtifactChange(nil), opts.Artifacts...)
	if len(deletedTaskIDs) > 0 && opts.DeletedTaskArtifacts != nil {
		deleteArtifacts, artifactErr := opts.DeletedTaskArtifacts(append([]string(nil), deletedTaskIDs...))
		if artifactErr != nil {
			return ActionPlanResult{}, fmt.Errorf("prepare deleted-task artifacts: %w", artifactErr)
		}
		artifacts = append(artifacts, deleteArtifacts...)
	}
	var changesets []storage.ChangeSet
	var touchedOrder []string
	for _, project := range working.Projects() {
		if !touched[project.ID] {
			continue
		}
		applyPostMutationHooks(project)
		var projectArtifacts []storage.ArtifactChange
		if len(changesets) == 0 {
			projectArtifacts = artifacts
		}
		changesets = append(changesets, storage.ChangeSet{
			Project:          project,
			ExpectedRevision: original.Revision(project.ID),
			Base:             original.Document(project.ID),
			Artifacts:        projectArtifacts,
			AttachmentOps:    attachmentOps[project.ID],
			UndoAction:       "Apply AI Action Plan",
			UndoContext:      strings.TrimSpace(plan.Summary),
		})
		touchedOrder = append(touchedOrder, project.ID)
	}
	if len(changesets) == 0 {
		return ActionPlanResult{DeletedTaskIDs: deletedTaskIDs, Actions: previews}, nil
	}
	results, commitErr := s.repo.Commit(changesets...)
	if commitErr != nil {
		return ActionPlanResult{}, commitErr
	}
	txnID := ""
	if len(results) > 0 && results[0] != nil {
		txnID = results[0].TransactionID
	}
	return ActionPlanResult{
		TransactionID:   txnID,
		UndoID:          txnID,
		TouchedProjects: touchedOrder,
		DeletedTaskIDs:  deletedTaskIDs,
		Actions:         previews,
	}, nil
}

func sortedActionTaskIDs(ids map[string]bool) []string {
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

//nolint:gocyclo // The action union is intentionally dispatched in one auditable allowlist.
func (s *Service) applyPlannedAction(snap Snapshot, action Action, touched map[string]bool) error {
	if action.Type == ActionCreateTask {
		project := snap.ProjectByID(action.ProjectID)
		if project == nil {
			return fmt.Errorf("%w: %s", ErrProjectNotFound, action.ProjectID)
		}
		taskType := models.TaskTypeTask
		if action.TaskType != nil {
			taskType = *action.TaskType
		}
		priority := models.PriorityDefault
		if action.Priority != nil {
			priority = *action.Priority
		}
		tags, err := normalizeTags(action.Tags)
		if err != nil {
			return err
		}
		task, err := models.NewTask(s.ids.NewID(), project.GetNextShortID(), taskType, strings.TrimSpace(*action.Title))
		if err != nil {
			return invalidInput("title", err.Error())
		}
		now := s.clock.Now()
		task.CreatedAt = now
		task.UpdatedAt = now
		task.Priority = priority
		task.Tags = tags
		if action.Description != nil {
			task.Description = *action.Description
		}
		project.Tasks = append(project.Tasks, task)
		touched[project.ID] = true
		return nil
	}

	ref, ok := snap.Lookup(action.TaskID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, action.TaskID)
	}
	task := ref.Task
	switch action.Type {
	case ActionEditTask:
		if action.Title != nil {
			task.Title = strings.TrimSpace(*action.Title)
		}
		if action.Description != nil {
			task.Description = *action.Description
		}
		if action.TaskType != nil && task.Type != *action.TaskType {
			task.ClearTypeAttrs()
			task.Type = *action.TaskType
		}
	case ActionSetState:
		from, target := task.State(), *action.State
		if reason := models.CanTransitionState(from, target); reason != "" {
			return &TransitionError{From: string(from), To: string(target), Reason: reason}
		}
		if target == models.StateDone && NewWorkspaceResolver(snap).HasIncompleteDependencies(task, ref.Project) {
			return fmt.Errorf("%w: task #%d", ErrDependencyBlocked, task.ShortID)
		}
		if err := childrenCompletionGuard(task, target); err != nil {
			return err
		}
		applyState(task, target)
	case ActionSetPriority:
		task.Priority = *action.Priority
	case ActionSetDueDate:
		due, hasTime, err := parseActionDue(*action.Due)
		if err != nil {
			return err
		}
		task.DueDate = &due
		task.HasTime = action.HasTime || hasTime
	case ActionClearDueDate:
		task.DueDate = nil
		task.HasTime = false
	case ActionSetTags:
		tags, err := normalizeTags(action.Tags)
		if err != nil {
			return err
		}
		task.Tags = tags
	case ActionAddDependency, ActionRemoveDependency:
		blocker, ok := snap.Lookup(action.BlockerTaskID)
		if !ok {
			return fmt.Errorf("%w: blocker %s", ErrTaskNotFound, action.BlockerTaskID)
		}
		resolver := NewWorkspaceResolver(snap)
		storedRef := blocker.Task.ID
		if blocker.Project.ID != ref.Project.ID {
			storedRef = blocker.Project.ID + ":" + blocker.Task.ID
		}
		if action.Type == ActionAddDependency {
			if blocker.Task.ID == task.ID && blocker.Project.ID == ref.Project.ID {
				return invalidInput("dependency", "a task cannot depend on itself")
			}
			if resolver.WouldCreateDependencyCycle(ref.Project, task.ID, storedRef) {
				return invalidInput("dependency", "would create a circular dependency")
			}
			task.DependsOn, _ = resolver.deduplicateTaskReferences(task.DependsOn, ref.Project)
			if !resolver.containsTaskReference(task.DependsOn, ref.Project, blocker.Project, blocker.Task.ID) {
				if err := task.AddDependency(storedRef); err != nil {
					return invalidInput("dependency", err.Error())
				}
			}
		} else {
			task.DependsOn, _ = resolver.removeTaskReferences(task.DependsOn, ref.Project, blocker.Project, blocker.Task.ID)
		}
	default:
		return invalidInput("action type", fmt.Sprintf("%q is not allowlisted", action.Type))
	}
	task.UpdatedAt = s.clock.Now()
	touched[ref.Project.ID] = true
	return nil
}

func plannedDeletions(snap Snapshot, actions []Action) (map[string]bool, []TaskRef, error) {
	selected := make(map[string]TaskRef)
	for _, action := range actions {
		if action.Type != ActionDeleteTask {
			continue
		}
		ref, ok := snap.Lookup(action.TaskID)
		if !ok {
			return nil, nil, fmt.Errorf("%w: %s", ErrTaskNotFound, action.TaskID)
		}
		selected[ref.Task.ID] = ref
	}
	var roots []TaskRef
	added := make(map[string]bool)
	for _, action := range actions {
		if action.Type != ActionDeleteTask {
			continue
		}
		ref, ok := selected[action.TaskID]
		if !ok {
			continue
		}
		hasSelectedAncestor := false
		for parent := ref.Task.Parent; parent != nil; parent = parent.Parent {
			if _, found := selected[parent.ID]; found {
				hasSelectedAncestor = true
				break
			}
		}
		if !hasSelectedAncestor && !added[ref.Task.ID] {
			roots = append(roots, ref)
			added[ref.Task.ID] = true
		}
	}
	deleted := make(map[string]bool)
	for _, root := range roots {
		for id := range collectSubtreeIDs(root.Task) {
			deleted[id] = true
		}
	}
	return deleted, roots, nil
}

func parseActionDue(value string) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, invalidInput("due", "cannot be empty")
	}
	if due, err := time.Parse(time.RFC3339, value); err == nil {
		return due, true, nil
	}
	if due, err := time.Parse("2006-01-02", value); err == nil {
		return due, false, nil
	}
	return time.Time{}, false, invalidInput("due", "must be YYYY-MM-DD or RFC3339")
}

func cloneActionSnapshot(source Snapshot) (Snapshot, error) {
	projects := source.Projects()
	clones := make([]*models.Project, 0, len(projects))
	revisions := make(map[string]string, len(projects))
	documents := make(map[string]*storage.ProjectDocument, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		clone := *project
		clone.Tasks = make([]*models.Task, 0, len(project.Tasks))
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			copyTask := *task
			copyTask.Parent = nil
			copyTask.Project = nil
			copyTask.Children = nil
			copyTask.Tags = append([]string(nil), task.Tags...)
			copyTask.DependsOn = append([]string(nil), task.DependsOn...)
			copyTask.RelatedTo = append([]string(nil), task.RelatedTo...)
			copyTask.Checklist = append([]models.ChecklistItem(nil), task.Checklist...)
			copyTask.Attachments = append([]models.Attachment(nil), task.Attachments...)
			copyTask.Comments = append([]*models.Comment(nil), task.Comments...)
			copyTask.Decisions = append([]*models.Decision(nil), task.Decisions...)
			copyTask.Activity = append([]*models.ActivityEntry(nil), task.Activity...)
			if task.ScoreDimensions != nil {
				copyTask.ScoreDimensions = make(map[string]int, len(task.ScoreDimensions))
				for key, value := range task.ScoreDimensions {
					copyTask.ScoreDimensions[key] = value
				}
			}
			clone.Tasks = append(clone.Tasks, &copyTask)
		}
		clone.Sections = append([]string(nil), project.Sections...)
		clone.Tags = append([]string(nil), project.Tags...)
		clone.PinnedTags = append([]string(nil), project.PinnedTags...)
		clone.SectionColors = make(map[string]string, len(project.SectionColors))
		for key, value := range project.SectionColors {
			clone.SectionColors[key] = value
		}
		clone.BuildTaskTree()
		clones = append(clones, &clone)
		revisions[clone.ID] = source.Revision(clone.ID)
		documents[clone.ID] = source.Document(clone.ID)
	}
	return NewSnapshot(clones, revisions, documents), nil
}
