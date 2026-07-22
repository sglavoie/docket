package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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
	// DeletedTaskLocatorArtifacts is called with the complete, sorted set of
	// project-qualified tasks removed by accepted delete actions, including every
	// subtree descendant. Known-owner adapters should use this callback so durable
	// cleanup cannot affect an equal task ID in another project.
	DeletedTaskLocatorArtifacts func([]TaskLocator) ([]storage.ArtifactChange, error)
	// DeletedTaskArtifacts is called with the complete, sorted set of task IDs
	// removed by accepted delete actions, including every subtree descendant. The
	// ID-only callback remains as a compatibility seam for adapters that have not
	// migrated to DeletedTaskLocatorArtifacts. When both are set, the qualified
	// callback takes precedence.
	DeletedTaskArtifacts func([]string) ([]storage.ArtifactChange, error)
}

// ActionPlanResult reports the single application transaction. UndoID equals
// TransactionID because the workspace ledger keys its entry by the committed
// transaction manifest ID.
type ActionPlanResult struct {
	TransactionID       string          `json:"transaction_id"`
	UndoID              string          `json:"undo_id"`
	TouchedProjects     []string        `json:"touched_projects"`
	DeletedTaskIDs      []string        `json:"deleted_task_ids,omitempty"`
	DeletedTaskLocators []TaskLocator   `json:"deleted_task_locators,omitempty"`
	Actions             []ActionPreview `json:"actions"`
}

// PreparedActionPlan is an opaque, snapshot-bound interpretation of provider
// output. Its fields are deliberately private: callers may inspect defensive
// previews and select locally classified risk tiers, but cannot replace the
// retained actions, snapshot, resolved deletion subtrees, or revision guards.
// A prepared value and every risk-selected view derived from it share one
// single-use state.
type PreparedActionPlan struct {
	owner          *Service
	snapshot       Snapshot
	summary        string
	actions        []preparedAction
	deletionRoots  []preparedDeletionRoot
	deletedTaskIDs []string
	state          *preparedActionPlanState
}

type preparedAction struct {
	action            Action
	preview           ActionPreview
	targetProjectID   string
	blockerProjectID  string
	readProjectIDs    []string
	guardsAllProjects bool
}

type preparedDeletionRoot struct {
	projectID string
	taskID    string
	taskIDs   []string
}

type preparedActionPlanState struct {
	mu       sync.Mutex
	applying bool
	consumed bool
}

var (
	// ErrActionPlanConfirmationRequired is returned before any snapshot mutation
	// when the caller did not record the safe batch confirmation.
	ErrActionPlanConfirmationRequired = errors.New("app: AI action plan requires batch confirmation")
	// ErrActionPlanDestructiveConfirmationRequired is the separate escalation
	// gate for locally classified destructive actions.
	ErrActionPlanDestructiveConfirmationRequired = errors.New("app: AI action plan contains destructive actions requiring separate confirmation")
	// ErrPreparedActionPlanConsumed prevents a reviewed snapshot from being
	// committed more than once, including through a risk-selected view.
	ErrPreparedActionPlanConsumed = errors.New("app: prepared AI action plan has already been applied")
	// ErrPreparedActionPlanInUse prevents concurrent application of one retained
	// snapshot. Failed non-concurrent attempts remain retryable.
	ErrPreparedActionPlanInUse = errors.New("app: prepared AI action plan is already being applied")
	// ErrPreparedActionPlanServiceMismatch prevents a plan prepared by one
	// application service/repository from being applied through another.
	ErrPreparedActionPlanServiceMismatch = errors.New("app: prepared AI action plan belongs to a different service")
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

// PrepareActionPlan validates and defensively copies provider output, takes one
// workspace snapshot, and resolves every executable detail that must remain
// bound from preview through commit. ApplyPreparedActionPlan never snapshots or
// re-resolves deletion subtrees against live workspace state.
func (s *Service) PrepareActionPlan(plan ActionPlan) (*PreparedActionPlan, error) {
	if err := ValidateActionPlan(plan); err != nil {
		return nil, err
	}
	retainedPlan := cloneActionPlan(plan)
	live, err := s.repo.Snapshot()
	if err != nil {
		return nil, err
	}
	retained, err := cloneActionSnapshot(live)
	if err != nil {
		return nil, fmt.Errorf("retain action-plan snapshot: %w", err)
	}

	deleted, roots, err := plannedDeletions(retained, retainedPlan.Actions)
	if err != nil {
		return nil, err
	}
	prepared := &PreparedActionPlan{
		owner:          s,
		snapshot:       retained,
		summary:        strings.TrimSpace(retainedPlan.Summary),
		deletedTaskIDs: sortedActionTaskIDs(deleted),
		state:          &preparedActionPlanState{},
	}
	for _, root := range roots {
		ids := sortedActionTaskIDs(collectSubtreeIDs(root.Task))
		prepared.deletionRoots = append(prepared.deletionRoots, preparedDeletionRoot{
			projectID: root.Project.ID,
			taskID:    root.Task.ID,
			taskIDs:   ids,
		})
	}

	for i, action := range retainedPlan.Actions {
		description, err := describeAction(retained, action)
		if err != nil {
			return nil, fmt.Errorf("action %d: %w", i+1, err)
		}
		risk := RiskForAction(action)
		if action.Type == ActionDeleteTask {
			ref, err := resolveTaskSelection(retained, action.TaskID, TaskLocator{})
			if err != nil {
				return nil, fmt.Errorf("action %d: %w", i+1, err)
			}
			descendants := len(collectSubtreeIDs(ref.Task)) - 1
			noun := "descendants"
			if descendants == 1 {
				noun = "descendant"
			}
			description = fmt.Sprintf("%s (%d %s)", description, descendants, noun)
		}
		targetProjectID, blockerProjectID, readProjectIDs, resolveErr := resolvedActionProjects(retained, action)
		if resolveErr != nil {
			return nil, fmt.Errorf("action %d: %w", i+1, resolveErr)
		}
		prepared.actions = append(prepared.actions, preparedAction{
			action:            action,
			preview:           ActionPreview{Index: i + 1, Type: action.Type, Risk: risk, Description: description},
			targetProjectID:   targetProjectID,
			blockerProjectID:  blockerProjectID,
			readProjectIDs:    readProjectIDs,
			guardsAllProjects: action.Type == ActionDeleteTask,
		})
	}
	return prepared, nil
}

// Preview returns a defensive confirmation view. Original provider indexes are
// retained even after SelectRisks filters the executable set.
func (p *PreparedActionPlan) Preview() PlanPreview {
	if p == nil {
		return PlanPreview{}
	}
	preview := PlanPreview{Summary: p.summary, Risk: ActionRiskSafe}
	preview.Actions = make([]ActionPreview, len(p.actions))
	for i := range p.actions {
		action := p.actions[i].preview
		preview.Actions[i] = action
		if action.Risk == ActionRiskDestructive {
			preview.Risk = ActionRiskDestructive
			preview.DestructiveCount++
		} else {
			preview.SafeCount++
		}
	}
	return preview
}

// SelectRisks derives an executable view containing every action in the
// requested locally classified tiers. It neither re-snapshots nor re-resolves
// actions, and it shares the source plan's single-use state.
func (p *PreparedActionPlan) SelectRisks(risks ...ActionRisk) *PreparedActionPlan {
	if p == nil {
		return nil
	}
	selected := make(map[ActionRisk]bool, len(risks))
	for _, risk := range risks {
		selected[risk] = true
	}
	result := &PreparedActionPlan{
		owner:    p.owner,
		snapshot: p.snapshot,
		summary:  p.summary,
		state:    p.state,
	}
	includeDeletions := false
	for i := range p.actions {
		if !selected[p.actions[i].preview.Risk] {
			continue
		}
		result.actions = append(result.actions, p.actions[i])
		if p.actions[i].guardsAllProjects {
			includeDeletions = true
		}
	}
	if includeDeletions {
		result.deletedTaskIDs = append([]string(nil), p.deletedTaskIDs...)
		result.deletionRoots = clonePreparedDeletionRoots(p.deletionRoots)
	}
	return result
}

func resolvedActionProjects(snap Snapshot, action Action) (string, string, []string, error) {
	if action.Type == ActionCreateTask {
		project := snap.ProjectByID(action.ProjectID)
		if project == nil {
			return "", "", nil, fmt.Errorf("%w: %s", ErrProjectNotFound, action.ProjectID)
		}
		return project.ID, "", []string{project.ID}, nil
	}
	ref, err := resolveTaskSelection(snap, action.TaskID, TaskLocator{})
	if err != nil {
		return "", "", nil, err
	}
	read := map[string]bool{ref.Project.ID: true}
	if action.Type != ActionAddDependency && action.Type != ActionRemoveDependency {
		if action.Type == ActionSetState && action.State != nil && *action.State == models.StateDone {
			addReferencedProjectReads(snap, read, ref.Project, ref.Task.DependsOn)
		}
		return ref.Project.ID, "", orderedProjectIDs(snap, read), nil
	}
	blocker, err := resolveTaskSelection(snap, action.BlockerTaskID, TaskLocator{})
	if err != nil {
		return "", "", nil, fmt.Errorf("blocker: %w", err)
	}
	read[blocker.Project.ID] = true
	addReferencedProjectReads(snap, read, ref.Project, ref.Task.DependsOn)
	if action.Type == ActionAddDependency {
		addDependencyTraversalReads(snap, read, blocker.Task, blocker.Project, make(map[string]bool))
	}
	return ref.Project.ID, blocker.Project.ID, orderedProjectIDs(snap, read), nil
}

func addReferencedProjectReads(snap Snapshot, read map[string]bool, owner *models.Project, refs []string) {
	for _, stored := range refs {
		if project := projectForStoredTaskRef(snap, owner, stored); project != nil {
			read[project.ID] = true
		}
	}
}

func addDependencyTraversalReads(snap Snapshot, read map[string]bool, task *models.Task, owner *models.Project, visited map[string]bool) {
	if task == nil || owner == nil {
		return
	}
	key := owner.ID + "\x00" + task.ID
	if visited[key] {
		return
	}
	visited[key] = true
	read[owner.ID] = true
	for _, stored := range task.DependsOn {
		project := projectForStoredTaskRef(snap, owner, stored)
		if project == nil {
			continue
		}
		read[project.ID] = true
		parsed := models.ParseTaskRef(stored)
		if next := project.GetTaskByID(parsed.TaskID); next != nil {
			addDependencyTraversalReads(snap, read, next, project, visited)
		}
	}
}

func projectForStoredTaskRef(snap Snapshot, owner *models.Project, stored string) *models.Project {
	parsed := models.ParseTaskRef(stored)
	if !parsed.IsCrossProject() {
		return owner
	}
	if project := snap.ProjectByID(parsed.ProjectID); project != nil {
		return project
	}
	for _, project := range snap.Projects() {
		if project != nil && project.Slug == parsed.ProjectID {
			return project
		}
	}
	return nil
}

func orderedProjectIDs(snap Snapshot, selected map[string]bool) []string {
	ids := make([]string, 0, len(selected))
	for _, project := range snap.Projects() {
		if project != nil && selected[project.ID] {
			ids = append(ids, project.ID)
		}
	}
	return ids
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
		ref, err := resolveTaskSelection(snap, id, TaskLocator{})
		if err != nil {
			return "", TaskRef{}, err
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

// ApplyPreparedActionPlan locally risk-gates and executes the already-resolved
// actions against a clone of the exact snapshot retained at preparation. It
// never asks the repository for a second snapshot. Every project the accepted
// actions read is attached to the storage transaction as a read-only canonical
// revision guard, so relevant drift expires the plan instead of entering the
// ordinary three-way merge path.
//
//nolint:gocyclo // The explicit phases preserve the all-or-nothing transaction boundary.
func (s *Service) ApplyPreparedActionPlan(ctx context.Context, prepared *PreparedActionPlan, opts ApplyActionPlanOptions) (result ActionPlanResult, err error) {
	if prepared == nil {
		return ActionPlanResult{}, invalidInput("prepared action plan", "is required")
	}
	if prepared.owner != s {
		return ActionPlanResult{}, ErrPreparedActionPlanServiceMismatch
	}
	if beginErr := prepared.beginApply(); beginErr != nil {
		return ActionPlanResult{}, beginErr
	}
	succeeded := false
	defer func() { prepared.finishApply(succeeded) }()

	result, err = s.applyPreparedActionPlan(ctx, prepared, opts)
	if err == nil {
		succeeded = true
	}
	return result, err
}

//nolint:gocyclo // The explicit phases preserve the all-or-nothing transaction boundary.
func (s *Service) applyPreparedActionPlan(ctx context.Context, prepared *PreparedActionPlan, opts ApplyActionPlanOptions) (ActionPlanResult, error) {
	preview := prepared.Preview()
	if len(prepared.actions) == 0 {
		return ActionPlanResult{Actions: preview.Actions}, nil
	}
	if !opts.ConfirmSafe {
		return ActionPlanResult{}, ErrActionPlanConfirmationRequired
	}
	for i := range prepared.actions {
		if prepared.actions[i].preview.Risk == ActionRiskDestructive && !opts.ConfirmDestructive {
			return ActionPlanResult{}, ErrActionPlanDestructiveConfirmationRequired
		}
	}
	if err := ctx.Err(); err != nil {
		return ActionPlanResult{}, err
	}

	working, cloneErr := cloneActionSnapshot(prepared.snapshot)
	if cloneErr != nil {
		return ActionPlanResult{}, cloneErr
	}

	deletedIDs := make(map[string]bool, len(prepared.deletedTaskIDs))
	for _, id := range prepared.deletedTaskIDs {
		deletedIDs[id] = true
	}
	for i := range prepared.actions {
		action := prepared.actions[i].action
		if action.Type != ActionDeleteTask && deletedIDs[action.TaskID] {
			return ActionPlanResult{}, invalidInput("action plan", fmt.Sprintf("action %d targets a task deleted by the same plan", prepared.actions[i].preview.Index))
		}
		if deletedIDs[action.BlockerTaskID] {
			return ActionPlanResult{}, invalidInput("action plan", fmt.Sprintf("action %d references a blocker deleted by the same plan", prepared.actions[i].preview.Index))
		}
	}

	touched := make(map[string]bool)
	for i := range prepared.actions {
		action := prepared.actions[i].action
		if action.Type == ActionDeleteTask {
			continue
		}
		if applyErr := s.applyPlannedAction(working, action, touched); applyErr != nil {
			return ActionPlanResult{}, fmt.Errorf("action %d: %w", prepared.actions[i].preview.Index, applyErr)
		}
		if contextErr := ctx.Err(); contextErr != nil {
			return ActionPlanResult{}, contextErr
		}
	}

	attachmentOps := make(map[string][]storage.AttachmentOp)
	if len(prepared.deletionRoots) > 0 {
		for _, root := range prepared.deletionRoots {
			project := working.ProjectByID(root.projectID)
			if project == nil {
				return ActionPlanResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, root.projectID)
			}
			subtree := make(map[string]bool, len(root.taskIDs))
			for _, id := range root.taskIDs {
				subtree[id] = true
			}
			for _, task := range tasksMatching(project.Tasks, subtree) {
				attachmentOps[project.ID] = append(attachmentOps[project.ID], storage.AttachmentOp{
					Kind:          storage.AttachmentDelete,
					Task:          task,
					SourceProject: project,
				})
			}
			project.Tasks = filterOutTasks(project.Tasks, subtree)
			touched[project.ID] = true
		}
		for _, project := range working.Projects() {
			for _, root := range prepared.deletionRoots {
				if project.ID == root.projectID {
					continue
				}
				rootProject := working.ProjectByID(root.projectID)
				subtree := make(map[string]bool, len(root.taskIDs))
				for _, id := range root.taskIDs {
					subtree[id] = true
				}
				if clearCrossProjectRefs(project, rootProject, subtree) > 0 {
					touched[project.ID] = true
				}
			}
		}
	}

	if contextErr := ctx.Err(); contextErr != nil {
		return ActionPlanResult{}, contextErr
	}
	deletedTaskIDs := append([]string(nil), prepared.deletedTaskIDs...)
	deletedTaskLocators := actionPlanDeletedTaskLocators(prepared.deletionRoots)
	artifacts := append([]storage.ArtifactChange(nil), opts.Artifacts...)
	if len(deletedTaskLocators) > 0 && opts.DeletedTaskLocatorArtifacts != nil {
		deleteArtifacts, artifactErr := opts.DeletedTaskLocatorArtifacts(append([]TaskLocator(nil), deletedTaskLocators...))
		if artifactErr != nil {
			return ActionPlanResult{}, fmt.Errorf("prepare deleted-task artifacts: %w", artifactErr)
		}
		artifacts = append(artifacts, deleteArtifacts...)
	} else if len(deletedTaskIDs) > 0 && opts.DeletedTaskArtifacts != nil {
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
			ExpectedRevision: prepared.snapshot.Revision(project.ID),
			Base:             prepared.snapshot.Document(project.ID),
			Artifacts:        projectArtifacts,
			AttachmentOps:    attachmentOps[project.ID],
			UndoAction:       "Apply AI Action Plan",
			UndoContext:      prepared.summary,
		})
		touchedOrder = append(touchedOrder, project.ID)
	}
	if len(changesets) == 0 {
		return ActionPlanResult{DeletedTaskIDs: deletedTaskIDs, DeletedTaskLocators: deletedTaskLocators, Actions: preview.Actions}, nil
	}
	changesets[0].ProjectPreconditions = prepared.projectPreconditions()
	results, commitErr := s.repo.Commit(changesets...)
	if commitErr != nil {
		return ActionPlanResult{}, commitErr
	}
	txnID := ""
	if len(results) > 0 && results[0] != nil {
		txnID = results[0].TransactionID
	}
	return ActionPlanResult{
		TransactionID:       txnID,
		UndoID:              txnID,
		TouchedProjects:     touchedOrder,
		DeletedTaskIDs:      deletedTaskIDs,
		DeletedTaskLocators: deletedTaskLocators,
		Actions:             append([]ActionPreview(nil), preview.Actions...),
	}, nil
}

func actionPlanDeletedTaskLocators(roots []preparedDeletionRoot) []TaskLocator {
	qualified := make(map[TaskLocator]bool)
	for _, root := range roots {
		for _, taskID := range root.taskIDs {
			if root.projectID != "" && taskID != "" {
				qualified[TaskLocator{ProjectID: root.projectID, TaskID: taskID}] = true
			}
		}
	}
	return sortedTaskLocators(qualified)
}

func (p *PreparedActionPlan) beginApply() error {
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	if p.state.consumed {
		return ErrPreparedActionPlanConsumed
	}
	if p.state.applying {
		return ErrPreparedActionPlanInUse
	}
	p.state.applying = true
	return nil
}

func (p *PreparedActionPlan) finishApply(succeeded bool) {
	p.state.mu.Lock()
	p.state.applying = false
	if succeeded {
		p.state.consumed = true
	}
	p.state.mu.Unlock()
}

func (p *PreparedActionPlan) projectPreconditions() []storage.ProjectRevisionPrecondition {
	guardAll := false
	relevant := make(map[string]bool)
	for i := range p.actions {
		action := p.actions[i]
		guardAll = guardAll || action.guardsAllProjects
		for _, projectID := range action.readProjectIDs {
			relevant[projectID] = true
		}
	}
	preconditions := make([]storage.ProjectRevisionPrecondition, 0, len(relevant))
	for _, project := range p.snapshot.Projects() {
		if project == nil || (!guardAll && !relevant[project.ID]) {
			continue
		}
		preconditions = append(preconditions, storage.ProjectRevisionPrecondition{
			ProjectID:        project.ID,
			ProjectName:      project.Name,
			Path:             project.Path,
			ExpectedRevision: p.snapshot.Revision(project.ID),
		})
	}
	return preconditions
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

	ref, err := resolveTaskSelection(snap, action.TaskID, TaskLocator{})
	if err != nil {
		return err
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
		blocker, err := resolveTaskSelection(snap, action.BlockerTaskID, TaskLocator{})
		if err != nil {
			return fmt.Errorf("blocker: %w", err)
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
		ref, err := resolveTaskSelection(snap, action.TaskID, TaskLocator{})
		if err != nil {
			return nil, nil, err
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

func cloneActionPlan(source ActionPlan) ActionPlan {
	clone := ActionPlan{Version: source.Version, Summary: source.Summary}
	clone.Actions = make([]Action, len(source.Actions))
	for i := range source.Actions {
		clone.Actions[i] = cloneAction(source.Actions[i])
	}
	return clone
}

func cloneAction(source Action) Action {
	clone := Action{
		Type:          source.Type,
		ProjectID:     source.ProjectID,
		TaskID:        source.TaskID,
		HasTime:       source.HasTime,
		Tags:          append([]string(nil), source.Tags...),
		BlockerTaskID: source.BlockerTaskID,
	}
	if source.Title != nil {
		value := *source.Title
		clone.Title = &value
	}
	if source.Description != nil {
		value := *source.Description
		clone.Description = &value
	}
	if source.TaskType != nil {
		value := *source.TaskType
		clone.TaskType = &value
	}
	if source.State != nil {
		value := *source.State
		clone.State = &value
	}
	if source.Priority != nil {
		value := *source.Priority
		clone.Priority = &value
	}
	if source.Due != nil {
		value := *source.Due
		clone.Due = &value
	}
	return clone
}

func clonePreparedDeletionRoots(source []preparedDeletionRoot) []preparedDeletionRoot {
	clones := make([]preparedDeletionRoot, len(source))
	for i := range source {
		clones[i] = source[i]
		clones[i].taskIDs = append([]string(nil), source[i].taskIDs...)
	}
	return clones
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
		if project.ArchivedAt != nil {
			value := *project.ArchivedAt
			clone.ArchivedAt = &value
		}
		if project.PinnedAt != nil {
			value := *project.PinnedAt
			clone.PinnedAt = &value
		}
		clone.Tasks = make([]*models.Task, 0, len(project.Tasks))
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			data, err := json.Marshal(task)
			if err != nil {
				return nil, fmt.Errorf("clone task %s: %w", task.ID, err)
			}
			var copyTask models.Task
			if err := json.Unmarshal(data, &copyTask); err != nil {
				return nil, fmt.Errorf("clone task %s: %w", task.ID, err)
			}
			clone.Tasks = append(clone.Tasks, &copyTask)
		}
		clone.Sections = append([]string(nil), project.Sections...)
		clone.Tags = append([]string(nil), project.Tags...)
		clone.PinnedTags = append([]string(nil), project.PinnedTags...)
		clone.Warnings = append([]string(nil), project.Warnings...)
		if project.SectionColors != nil {
			clone.SectionColors = make(map[string]string, len(project.SectionColors))
			for key, value := range project.SectionColors {
				clone.SectionColors[key] = value
			}
		}
		clone.BuildTaskTree()
		clones = append(clones, &clone)
		revisions[clone.ID] = source.Revision(clone.ID)
		if document := source.Document(clone.ID); document != nil {
			copyDocument, err := storage.ParseProjectDocument(document.WriteV2())
			if err != nil {
				return nil, fmt.Errorf("clone document for project %s: %w", clone.ID, err)
			}
			documents[clone.ID] = copyDocument
		}
	}
	return NewSnapshot(clones, revisions, documents), nil
}
