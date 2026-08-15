package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// OutlinePlanLine is one nonblank line of an edited outline buffer, already
// parsed out of the capture grammar by the adapter that owns the editor round
// trip. A line without a TaskID creates a task; a line carrying one edits the
// task it names.
type OutlinePlanLine struct {
	// SourceLine is the one-based buffer line the values came from. Zero falls
	// back to the line's position in the request when an error is reported.
	SourceLine int
	// TaskID names an existing task in the scope project. Empty creates a task.
	TaskID string
	// Depth is the zero-based indentation depth. The nearest preceding line one
	// level shallower is the parent; depth zero is a root.
	Depth int
	// Title is the required task title.
	Title string
	// Priority is 0..3, where zero is the durable Unset value.
	Priority int
	// Tags replace the task's tag set outright. The dump is always complete, so
	// an omitted tag is a removal rather than an absent instruction.
	Tags []string
	// Context replaces the single-valued task context; empty clears it.
	Context string
	// Due replaces the calendar due date; nil clears it.
	Due *time.Time
	// Pinned replaces the task's pinned state outright. The dump is always
	// complete, so an omitted *pin is an unpin rather than an absent instruction.
	Pinned bool
	// Destination is the typed lifecycle/planning destination. Empty preserves an
	// existing task's state and defaults a new task the way capture does.
	Destination CaptureDestination
}

// OutlinePlanRequest is one edited outline buffer, resolved against a single
// project and committed as one transaction.
type OutlinePlanRequest struct {
	// ProjectID owns every line in the buffer. An outline plan never moves a task
	// between projects, which is why %project has no field here.
	ProjectID string
	// ExpectedRevision is the revision the buffer was dumped at. It is required:
	// a buffer describes one exact tree, so an apply that cannot name the tree it
	// was authored against has nothing to be atomic with respect to.
	ExpectedRevision string
	// ScopeTaskIDs are the tasks the buffer was dumped from, and therefore the
	// only tasks absence can speak about. A task the dump never printed is
	// invisible to the buffer and must not be reachable by removing a line.
	ScopeTaskIDs []string
	// DeleteTaskIDs are the scope tasks the returned buffer no longer holds,
	// exactly as the adapter presented them for confirmation. It must name the
	// same set the buffer leaves absent, so what was confirmed and what is
	// removed are provably the same list.
	DeleteTaskIDs []string
	// ConfirmDeletions records explicit destructive intent for DeleteTaskIDs, the
	// way DeleteRequest.Confirm does for a single delete.
	ConfirmDeletions bool
	// Lines are the buffer's nonblank lines in their final order.
	Lines []OutlinePlanLine
	// PlanningArtifacts overrides the service planning builder.
	PlanningArtifacts PlanningArtifactBuilder
}

// OutlinePlanResult reports what one applied buffer did: the tasks it created,
// the existing tasks it actually changed, and the dumped tasks it removed.
type OutlinePlanResult struct {
	Created []TaskResult
	Updated []TaskResult
	Deleted []TaskResult
}

// Summary is the one-line report an adapter shows after applying a buffer. It
// names the deletions separately from the edits, because a removed line and an
// edited line are the two things the buffer cannot tell apart on its own.
func (r OutlinePlanResult) Summary() string {
	return outlinePlanSummary(len(r.Created)+len(r.Updated), len(r.Deleted))
}

// ProjectAtRevision returns a project together with the revision it was loaded
// at, both from one snapshot. An adapter that dumps a project to an external
// editor renders the returned project and carries the returned revision back
// into OutlinePlanRequest, so the buffer and the revision it claims to describe
// can never come from two different reads.
func (s *Service) ProjectAtRevision(projectID string) (*models.Project, string, error) {
	return s.resolveProject(projectID)
}

// ApplyOutlinePlan commits one edited outline buffer as a single transaction
// with one undo entry: heterogeneous edits, creations and reparenting resolve
// from one snapshot, apply in memory against the single scope project, reconcile
// planning once, and reach storage through one project commit.
//
// The buffer is declarative and complete for the scope it was dumped from, so a
// scope task whose line the buffer no longer holds is deleted. The Delete guard
// still owns the decision: the adapter counts the removals when the editor
// returns, presents them, and sends the confirmed list back as DeleteTaskIDs.
// Those counts cannot go stale between the prompt and the write, because the
// revision check below refuses any buffer whose project moved in the meantime.
func (s *Service) ApplyOutlinePlan(req OutlinePlanRequest) (OutlinePlanResult, error) {
	if strings.TrimSpace(req.ProjectID) == "" {
		return OutlinePlanResult{}, invalidInput("project", "project is required")
	}
	// An empty buffer is a legitimate request to empty the project, so it is
	// refused only when it also removes nothing: that buffer asks for no change at
	// all and is far more likely to be a truncated file than an intent.
	if len(req.Lines) == 0 && len(req.DeleteTaskIDs) == 0 {
		return OutlinePlanResult{}, invalidInput("outline", "the buffer holds no tasks")
	}
	if strings.TrimSpace(req.ExpectedRevision) == "" {
		return OutlinePlanResult{}, invalidInput("revision", "the revision the buffer was dumped at is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return OutlinePlanResult{}, err
	}
	project := snap.ProjectByID(req.ProjectID)
	if project == nil {
		return OutlinePlanResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, req.ProjectID)
	}
	// The dump revision is enforced here rather than only at the commit
	// precondition, because everything between this line and the commit mutates
	// the snapshot's live tasks. A writer that landed while the editor was open
	// must be refused before a single title is overwritten.
	if current := snap.Revision(project.ID); current != req.ExpectedRevision {
		return OutlinePlanResult{}, &storage.RevisionConflictError{
			ProjectID:   project.ID,
			ProjectName: project.Name,
			Expected:    req.ExpectedRevision,
			Actual:      current,
		}
	}
	base := snap.Document(project.ID)
	if base == nil {
		return OutlinePlanResult{}, fmt.Errorf("app: project %s has no immutable base document", project.ID)
	}
	prepared, err := prepareOutlinePlan(snap, project, req.Lines)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	deleted, err := outlineDeletions(project, req, prepared)
	if err != nil {
		return OutlinePlanResult{}, err
	}

	before, err := cloneSnapshot(snap)
	if err != nil {
		return OutlinePlanResult{}, fmt.Errorf("capture outline rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	attachmentOps, removedLocators := removeOutlineTasks(project, deleted)
	targets, err := s.applyOutlinePlan(project, base, prepared)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	created, updated := partitionOutlineChanges(before.ProjectByID(project.ID), prepared)
	// Only a task the buffer really changed is touched. A line that restated a
	// task verbatim must not move it to the top of Recent.
	for _, task := range updated {
		task.UpdatedAt = s.clock.Now()
	}

	artifacts, err := planningArtifactsForTargets(targets, removedLocators, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return OutlinePlanResult{}, err
	}
	summary := outlinePlanSummary(len(created)+len(updated), len(deleted))
	outcome, err := s.commitProjects([]projectCommit{{
		project:          project,
		expectedRevision: snap.Revision(project.ID),
		base:             base,
		artifacts:        artifacts,
		attachmentOps:    attachmentOps,
	}}, "Apply Outline", summary)
	if err != nil {
		return OutlinePlanResult{}, err
	}
	committed = true

	revision := revisionAt(outcome.results, 0)
	transactionID := transactionIDAt(outcome.results, 0)
	return OutlinePlanResult{
		Created: outlineTaskResults(project, created, revision, transactionID),
		Updated: outlineTaskResults(project, updated, revision, transactionID),
		Deleted: outlineTaskResults(project, deleted, revision, transactionID),
	}, nil
}

// preparedOutlineLine is one validated buffer line paired with the task it acts
// on. A nil task means the line creates one; it is materialized only after the
// rollback guard is installed, because minting a short ID mutates the project.
type preparedOutlineLine struct {
	line     OutlinePlanLine
	position int
	title    string
	priority int
	tags     []string
	context  string
	task     *models.Task
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
	return item, nil
}

// applyOutlinePlan mutates the scope project in memory. Parentage and sibling
// order settle before any state moves, so the Done-with-nonterminal-child guard
// reads the tree the buffer actually asked for rather than the one it replaced.
func (s *Service) applyOutlinePlan(project *models.Project, base *storage.ProjectDocument, prepared []preparedOutlineLine) ([]PlanningReconcileTarget, error) {
	if err := s.materializeOutlineTasks(project, prepared); err != nil {
		return nil, err
	}
	applyOutlineParentage(prepared)
	applyOutlineOrder(project, prepared)
	if err := validateOutlineNesting(project, prepared); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for _, item := range prepared {
		item.task.Title = item.title
		item.task.Priority = item.priority
		item.task.Tags = append([]string{}, item.tags...)
		item.task.Context = item.context
		item.task.DueDate = cloneCalendarDatePtr(item.line.Due)
		item.task.ApplyPinned(item.line.Pinned, now)
	}
	return s.applyOutlineDestinations(project, base, prepared, now)
}

func (s *Service) materializeOutlineTasks(project *models.Project, prepared []preparedOutlineLine) error {
	for i := range prepared {
		if !prepared[i].creates() {
			continue
		}
		task, err := newTaskForCreation(s.ids.NewID(), project.GetNextShortID(), prepared[i].title)
		if err != nil {
			return outlineLineError(prepared[i].line, prepared[i].position, invalidInput("title", err.Error()))
		}
		prepared[i].task = task
		project.Tasks = append(project.Tasks, task)
	}
	return nil
}

// applyOutlineParentage derives parentage from indentation alone. Indentation is
// the buffer's only parentage channel, which is why >parent and <child are both
// rejected: two channels would need a precedence rule nobody can see in the text.
func applyOutlineParentage(prepared []preparedOutlineLine) {
	deepestAt := make(map[int]string, len(prepared))
	for _, item := range prepared {
		depth := item.line.Depth
		if depth == 0 {
			item.task.ParentID = nil
		} else {
			parentID := deepestAt[depth-1]
			item.task.ParentID = &parentID
		}
		deepestAt[depth] = item.task.ID
	}
}

// applyOutlineOrder rewrites the project's task slice so the buffer's order is
// the sibling order. The buffered block takes the position of the first task it
// already held, so tasks the buffer never showed keep their relative placement.
func applyOutlineOrder(project *models.Project, prepared []preparedOutlineLine) {
	ordered := make([]*models.Task, 0, len(prepared))
	buffered := make(map[string]bool, len(prepared))
	for _, item := range prepared {
		ordered = append(ordered, item.task)
		buffered[item.task.ID] = true
	}
	out := make([]*models.Task, 0, len(project.Tasks))
	spliced := false
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		if buffered[task.ID] {
			if !spliced {
				spliced = true
				out = append(out, ordered...)
			}
			continue
		}
		out = append(out, task)
	}
	if !spliced {
		out = append(out, ordered...)
	}
	project.Tasks = out
}

// validateOutlineNesting rejects a buffer whose indentation pushes any task past
// the nesting cap, including a descendant the buffer never showed. The check is
// explicit because BuildTaskTree silently detaches an over-deep task instead of
// failing, which would turn an indentation mistake into lost parentage.
func validateOutlineNesting(project *models.Project, prepared []preparedOutlineLine) error {
	byID := make(map[string]*models.Task, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil {
			byID[task.ID] = task
		}
	}
	depths := make(map[string]int, len(project.Tasks))
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		depth, err := outlineTaskDepth(task, byID, depths, make(map[string]bool))
		if err != nil {
			return err
		}
		if depth > models.MaxNestingDepth {
			return outlineNestingError(task, prepared)
		}
	}
	return nil
}

func outlineTaskDepth(task *models.Task, byID map[string]*models.Task, depths map[string]int, visiting map[string]bool) (int, error) {
	if depth, ok := depths[task.ID]; ok {
		return depth, nil
	}
	if visiting[task.ID] {
		return 0, invalidInput("indentation", fmt.Sprintf("#%d has a circular parent reference", task.ShortID))
	}
	visiting[task.ID] = true
	depth := 0
	if task.ParentID != nil {
		if parent := byID[*task.ParentID]; parent != nil {
			parentDepth, err := outlineTaskDepth(parent, byID, depths, visiting)
			if err != nil {
				return 0, err
			}
			depth = parentDepth + 1
		}
	}
	depths[task.ID] = depth
	return depth, nil
}

func outlineNestingError(task *models.Task, prepared []preparedOutlineLine) error {
	err := invalidInput("indentation", fmt.Sprintf("#%d would nest deeper than the maximum of %d levels", task.ShortID, models.MaxNestingDepth))
	for _, item := range prepared {
		if item.task.ID == task.ID {
			return outlineLineError(item.line, item.position, err)
		}
	}
	return err
}

func (s *Service) applyOutlineDestinations(project *models.Project, base *storage.ProjectDocument, prepared []preparedOutlineLine, now time.Time) ([]PlanningReconcileTarget, error) {
	targets := make([]PlanningReconcileTarget, 0, len(prepared))
	for _, item := range prepared {
		target, err := s.applyOutlineDestination(project, base, item, now)
		if err != nil {
			return nil, outlineLineError(item.line, item.position, err)
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (s *Service) applyOutlineDestination(project *models.Project, base *storage.ProjectDocument, item preparedOutlineLine, now time.Time) (PlanningReconcileTarget, error) {
	if item.creates() {
		effect, err := normalizeCaptureDestination(item.line.Destination, item.task.ParentID != nil)
		if err != nil {
			return PlanningReconcileTarget{}, err
		}
		if err := s.initializeCreatedTask(item.task, effect.state); err != nil {
			return PlanningReconcileTarget{}, err
		}
		return effect.planningTarget(item.task.ID, project.ID), nil
	}
	// An omitted destination preserves the state the task already holds. A
	// declarative buffer may clear a tag or a date, but resetting a lifecycle
	// state through an absent sigil would launder a transition nobody typed.
	if item.line.Destination == "" {
		return PlanningReconcileTarget{TaskID: item.task.ID, ProjectID: project.ID, State: item.task.State()}, nil
	}
	if action, ok := outlinePlanningAction(item.line.Destination); ok {
		target, _, err := ApplyPlanningActionToTask(item.task, project.ID, action, now)
		return target, err
	}
	effect, err := normalizeCaptureDestination(item.line.Destination, false)
	if err != nil {
		return PlanningReconcileTarget{}, err
	}
	target := effect.planningTarget(item.task.ID, project.ID)
	if effect.state == item.task.State() {
		item.task.ApplyWorkflowState(effect.state)
		return target, nil
	}
	ref := TaskRef{Task: item.task, Project: project, Base: base}
	if err := validateEditStateTransition(ref, &effect.state); err != nil {
		return PlanningReconcileTarget{}, err
	}
	if err := applyState(item.task, effect.state, now); err != nil {
		return PlanningReconcileTarget{}, invalidInput("state", err.Error())
	}
	target.StateChanged = true
	return target, nil
}

// outlinePlanningAction routes the two planning destinations through the
// canonical planning primitive, so $today on an existing Doing task keeps Doing
// exactly as the Today operation does instead of demoting it to Backlog.
func outlinePlanningAction(destination CaptureDestination) (PlanningAction, bool) {
	switch destination {
	case CaptureDestinationToday:
		return PlanningAddToday, true
	case CaptureDestinationNext:
		return PlanningAddNext, true
	default:
		return "", false
	}
}

// partitionOutlineChanges splits the applied buffer into the tasks it created
// and the existing tasks it really changed, so the reported count is the number
// of edits the buffer made rather than the number of lines it held.
func partitionOutlineChanges(baseProject *models.Project, prepared []preparedOutlineLine) (created, updated []*models.Task) {
	baseRanks := outlineBaseRanks(baseProject, prepared)
	rank := 0
	for _, item := range prepared {
		if item.creates() {
			created = append(created, item.task)
			continue
		}
		if outlineTaskChanged(baseProject.GetTaskByID(item.task.ID), item.task, baseRanks[item.task.ID], rank) {
			updated = append(updated, item.task)
		}
		rank++
	}
	return created, updated
}

// outlineBaseRanks ranks the buffer's existing tasks by their prior order among
// each other, so a pure reorder counts as a change while an unrelated insertion
// elsewhere in the project does not.
func outlineBaseRanks(baseProject *models.Project, prepared []preparedOutlineLine) map[string]int {
	buffered := make(map[string]bool, len(prepared))
	for _, item := range prepared {
		if !item.creates() {
			buffered[item.task.ID] = true
		}
	}
	ranks := make(map[string]int, len(buffered))
	rank := 0
	for _, task := range baseProject.Tasks {
		if task == nil || !buffered[task.ID] {
			continue
		}
		ranks[task.ID] = rank
		rank++
	}
	return ranks
}

func outlineTaskChanged(base, task *models.Task, baseRank, rank int) bool {
	if base == nil {
		return true
	}
	return base.Title != task.Title ||
		base.Priority != task.Priority ||
		base.Context != task.Context ||
		base.State() != task.State() ||
		base.Pinned != task.Pinned ||
		baseRank != rank ||
		!slices.Equal(base.Tags, task.Tags) ||
		!sameOptionalID(base.ParentID, task.ParentID) ||
		!sameOptionalDate(base.DueDate, task.DueDate)
}

func sameOptionalID(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameOptionalDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// outlineDeletions resolves the tasks the buffer removed, in dump order, and
// refuses anything the request cannot account for.
//
// The absent set is derived here rather than trusted from DeleteTaskIDs, and the
// two must agree exactly. That equality is the whole guarantee: it proves the
// list the adapter counted and showed is the list this transaction removes.
func outlineDeletions(project *models.Project, req OutlinePlanRequest, prepared []preparedOutlineLine) ([]*models.Task, error) {
	buffered := make(map[string]int, len(prepared))
	for _, item := range prepared {
		if !item.creates() {
			buffered[item.task.ID] = item.line.SourceLine
		}
	}
	confirmed := make(map[string]bool, len(req.DeleteTaskIDs))
	for _, taskID := range req.DeleteTaskIDs {
		if taskID == "" {
			return nil, invalidInput("delete", "task id is required")
		}
		if project.GetTaskByID(taskID) == nil {
			return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
		}
		if line, held := buffered[taskID]; held {
			return nil, invalidInput("delete", fmt.Sprintf("task %s is on line %d and cannot also be deleted", taskID, line))
		}
		confirmed[taskID] = true
	}

	seen := make(map[string]bool, len(req.ScopeTaskIDs))
	deleted := make([]*models.Task, 0, len(confirmed))
	for _, taskID := range req.ScopeTaskIDs {
		if taskID == "" {
			return nil, invalidInput("scope", "task id is required")
		}
		task := project.GetTaskByID(taskID)
		if task == nil {
			return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
		}
		if _, held := buffered[taskID]; held || seen[taskID] {
			continue
		}
		seen[taskID] = true
		if !confirmed[taskID] {
			return nil, invalidInput("delete", fmt.Sprintf("task %s is missing from the buffer but was not confirmed for deletion", taskID))
		}
		deleted = append(deleted, task)
	}
	for taskID := range confirmed {
		if !seen[taskID] {
			return nil, invalidInput("delete", fmt.Sprintf("task %s was confirmed for deletion but the buffer still holds it", taskID))
		}
	}
	if len(deleted) > 0 && !req.ConfirmDeletions {
		return nil, fmt.Errorf("%w: deleting %d tasks the buffer no longer holds", ErrDestructiveConfirmationRequired, len(deleted))
	}
	return deleted, nil
}

// removeOutlineTasks drops the deleted tasks from the project and returns the
// side effects the same transaction has to carry. It runs before the buffer's
// own parentage is applied, so a survivor whose parent line vanished is
// re-parented by its own indentation rather than by the removal.
func removeOutlineTasks(project *models.Project, deleted []*models.Task) ([]storage.AttachmentOp, []TaskLocator) {
	if len(deleted) == 0 {
		return nil, nil
	}
	removed := make(map[string]bool, len(deleted))
	ops := make([]storage.AttachmentOp, 0, len(deleted))
	locators := make([]TaskLocator, 0, len(deleted))
	for _, task := range deleted {
		removed[task.ID] = true
		ops = append(ops, storage.AttachmentOp{
			Kind:          storage.AttachmentDelete,
			Task:          task,
			SourceProject: project,
		})
		locators = append(locators, TaskLocator{ProjectID: project.ID, TaskID: task.ID})
	}
	project.Tasks = filterOutTasks(project.Tasks, removed)
	return ops, locators
}

func outlineTaskResults(project *models.Project, tasks []*models.Task, revision, transactionID string) []TaskResult {
	if len(tasks) == 0 {
		return nil
	}
	results := make([]TaskResult, 0, len(tasks))
	for _, task := range tasks {
		results = append(results, newTaskResult(TaskRef{Task: task, Project: project}, revision, transactionID))
	}
	return results
}

func outlinePlanSummary(changes, deleted int) string {
	unit := "changes"
	if changes == 1 {
		unit = "change"
	}
	summary := fmt.Sprintf("Applied %d %s", changes, unit)
	if deleted > 0 {
		noun := "tasks"
		if deleted == 1 {
			noun = "task"
		}
		summary += fmt.Sprintf(" · %d %s deleted", deleted, noun)
	}
	return summary
}

func outlineLineError(line OutlinePlanLine, index int, err error) error {
	number := line.SourceLine
	if number <= 0 {
		number = index + 1
	}
	return fmt.Errorf("line %d: %w", number, err)
}
