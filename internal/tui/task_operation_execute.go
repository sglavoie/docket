package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

func (m *AppModel) executeTaskOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	if len(invocation.Targets.Targets) == 0 {
		return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID)}
	}
	if invocation.Descriptor.ValueProviderID != "" && !invocation.HasValue {
		return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s requires a value", invocation.Descriptor.ID)}
	}
	// Deletion takes every resolved target at once because one atomic
	// transaction removes single and multi-root selections alike.
	if invocation.Descriptor.ID == operation.OperationTaskDelete {
		return m.executeTaskDeleteOperation(invocation)
	}
	if invocation.Descriptor.ID == operation.OperationTaskTogglePin {
		return m.executeTaskTogglePinOperation(invocation)
	}
	if invocation.Descriptor.ID == operation.OperationTaskToggleReview {
		return m.executeTaskToggleReviewOperation(invocation)
	}
	if operationProjectsClipboard(invocation.Descriptor.ID) {
		return m.executeTaskCopyOperation(invocation)
	}
	if request, action, summary, handled, err := applicationTaskOperationRequest(invocation); handled {
		if err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
			Cmd: withWaitingCheckBackStep(invocation, withOutlinerRelocationCapture(
				m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary),
				capture,
			)),
		}}
	}
	if len(invocation.Targets.Targets) > 1 {
		return m.executeBulkTaskValueOperation(invocation)
	}
	target := invocation.Targets.Targets[0]
	identity := target.Task
	if !identity.Valid() {
		return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
	}
	return m.executeSingleTaskOperation(invocation, identity)
}

// withWaitingCheckBackStep follows a committed move into Waiting with the
// due-only picker rather than the combined one `@` opens, because the step is
// asking for a check-back deadline and a defer section would answer a different
// question. The chain is keyed off the operation's own value rather than runtime
// context, and it is gated on the transition's own outcome: a rejected move
// returns its error untouched, so no picker opens and no due date can be
// committed on a task that never reached Waiting. On the success path the state
// transition has already committed when the picker opens, so Esc there cancels
// the date alone and the Waiting move stands.
func withWaitingCheckBackStep(invocation operation.OperationInvocation, cmd tea.Cmd) tea.Cmd {
	if invocation.Descriptor.ID != operation.OperationTaskSetState {
		return cmd
	}
	if state, ok := invocation.Value.(models.WorkflowState); !ok || state != models.StateWaiting {
		return cmd
	}
	checkBack := operation.OperationValueRequestMsg{
		Request: operation.OperationExecutionRequest{
			ID:      operation.OperationTaskSetDueDate,
			Context: operation.CloneCommandContext(invocation.Context),
		},
		Provider: operation.ValueProviderDueDate,
	}
	return func() tea.Msg {
		msg := cmd()
		if !transactionCommitted(msg) {
			return msg
		}
		return tea.Sequence(
			func() tea.Msg { return msg },
			func() tea.Msg { return checkBack },
		)()
	}
}

// transactionCommitted reports whether a coordinator command resolved to a
// durable mutation. The two result types mirror withOutlinerRelocationCapture:
// a task operation lands on the batch path even for one target, while the
// single-task commit path still emits TransactionResultMsg.
func transactionCommitted(msg tea.Msg) bool {
	switch msg.(type) {
	case coordinator.BatchTransactionResultMsg, coordinator.TransactionResultMsg:
		return true
	default:
		return false
	}
}

// executeTaskTogglePinOperation applies one deterministic toggle to the whole
// frozen target set: if every target is pinned it unpins all of them, otherwise
// it pins all of them. The application operation validates and commits the
// project-qualified set atomically.
func (m *AppModel) executeTaskTogglePinOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	allPinned := true
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		task, _ := m.taskAndProjectForIdentity(target.Task)
		if task == nil {
			return operation.OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		allPinned = allPinned && task.Pinned
		locators = append(locators, target.Task.Locator())
	}
	pinned := !allPinned
	action, verb := "Pin Task", "Pinned"
	if !pinned {
		action, verb = "Unpin Task", "Unpinned"
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(
			app.TaskOperationRequest{
				Operation: app.TaskOperationSetPinned,
				Targets:   locators,
				Pinned:    pinned,
			},
			action,
			fmt.Sprintf("%s %d task(s)", verb, len(locators)),
		), capture),
	}}
}

// executeTaskToggleReviewOperation hands the whole frozen target set to the one
// application operation and lets it decide the direction. Unlike the pin toggle
// above, nothing here reads the flag: `bulkToggleReview` computes flag-all
// versus clear-all from the same snapshot it commits, so a second read in the
// presentation layer could only disagree with the mutation it is describing.
// The success line is left empty for the same reason — the coordinator builds it
// from the committed result, whose task count is the deduplicated one the
// transaction actually wrote.
func (m *AppModel) executeTaskToggleReviewOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		locators = append(locators, target.Task.Locator())
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(
			app.TaskOperationRequest{
				Operation: app.TaskOperationToggleReview,
				Targets:   locators,
			},
			"Toggle Review",
			"",
		), capture),
	}}
}

// operationProjectsClipboard names the three read-only projections. They branch
// off the mutation path before it because two things are true of them and of
// nothing else: the order of their targets is the order the copied text reads
// in, and they commit nothing, so there is no transaction for a relocation
// capture or a check-back step to follow.
func operationProjectsClipboard(id operation.OperationID) bool {
	switch id {
	case operation.OperationTaskCopyTitle,
		operation.OperationTaskCopyDetails,
		operation.OperationTaskCopyTitleDescription:
		return true
	default:
		return false
	}
}

// executeTaskCopyOperation copies the whole resolved selection, hidden members
// included, as one clipboard write. An empty visual selection never reaches it:
// the descriptors are one-or-more over the selected tasks, so the resolver
// refuses instead of falling back to the cursor.
func (m *AppModel) executeTaskCopyOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	invocation.Targets = m.clipboardTargetOrder(invocation.Targets)
	request, action, summary, handled, err := applicationTaskOperationRequest(invocation)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if !handled {
		return operation.OperationHandlerResult{Err: fmt.Errorf(
			"operation %s is not a clipboard projection",
			invocation.Descriptor.ID,
		)}
	}
	counts := operationSelectionCounts(invocation.Context, len(request.Targets))
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withClipboardHiddenCount(
			m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary),
			counts.Hidden,
		),
	}}
}

// clipboardTargetOrder replaces the resolver's identity ordering with the order
// the copied text reads in. The resolver stays sorted by identity for every
// other operation, because only copied text makes its target order visible.
func (m *AppModel) clipboardTargetOrder(
	targets operation.ResolvedOperationTargets,
) operation.ResolvedOperationTargets {
	if len(targets.Targets) < 2 {
		return targets
	}
	byIdentity := make(map[taskref.TaskIdentity]operation.OperationTarget, len(targets.Targets))
	identities := make([]taskref.TaskIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if !target.Task.Valid() {
			// A malformed set is the request builder's error to report, not this
			// reordering's to hide.
			return targets
		}
		identities = append(identities, target.Task)
		byIdentity[target.Task] = target
	}
	ordered := m.Outliner().ClipboardSelectionOrder(identities)
	reordered := make([]operation.OperationTarget, 0, len(ordered))
	for _, identity := range ordered {
		reordered = append(reordered, byIdentity[identity])
	}
	targets.Targets = reordered
	return targets
}

// withClipboardHiddenCount stamps the projection with the part of the selection
// the view was not showing. The application layer cannot supply it: a hidden
// member is a fact about the projection the selection was frozen against, not
// about the workspace the text was read from.
func withClipboardHiddenCount(cmd tea.Cmd, hidden int) tea.Cmd {
	if cmd == nil || hidden <= 0 {
		return cmd
	}
	return func() tea.Msg {
		msg := cmd()
		projected, ok := msg.(coordinator.TaskClipboardProjectedMsg)
		if !ok {
			return msg
		}
		projected.Hidden = hidden
		return projected
	}
}

func applicationTaskOperationRequest(
	invocation operation.OperationInvocation,
) (app.TaskOperationRequest, string, string, bool, error) {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return app.TaskOperationRequest{}, "", "", true, fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)
		}
		locators = append(locators, target.Task.Locator())
	}
	request := app.TaskOperationRequest{Targets: locators}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operation.OperationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return request, "", "", true, fmt.Errorf("invalid state value")
		}
		request.Operation = app.TaskOperationSetState
		request.State = state
		return request, "Change State",
			fmt.Sprintf("Change State · Changed %d task(s) to %s", count, state), true, nil

	case operation.OperationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return request, "", "", true, fmt.Errorf("invalid priority value")
		}
		request.Operation = app.TaskOperationSetPriority
		request.Priority = priority
		return request, "Set Priority",
			fmt.Sprintf("Set Priority · Set priority on %d task(s)", count), true, nil

	case operation.OperationTaskSetDueDate:
		value, ok := invocation.Value.(operation.OperationDateValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid date value")
		}
		request.Date = taskref.CloneOperationDate(value.Date)
		if value.Deferred {
			request.Operation = app.TaskOperationSetDefer
			verb := "Deferred"
			if value.Date == nil {
				verb = "Cleared defer on"
			}
			return request, "Set Defer",
				fmt.Sprintf("Set Defer · %s %d task(s)", verb, count), true, nil
		}
		request.Operation = app.TaskOperationSetDueDate
		request.PlanForToday = coordinator.DueDateIsToday(value.Date)
		verb := "Set due date on"
		if value.Date == nil {
			verb = "Cleared due date on"
		}
		return request, "Set Due Date",
			fmt.Sprintf("Set Due Date · %s %d task(s)", verb, count), true, nil

	case operation.OperationTaskSetTags:
		switch value := invocation.Value.(type) {
		case []string:
			request.Tags = append([]string(nil), value...)
		case operation.OperationTagsValue:
			request.TagPatch = true
			request.TagsToAdd = append([]string(nil), value.Add...)
			request.TagsToRemove = append([]string(nil), value.Remove...)
			request.ContextSet = value.ContextSet
			request.Context = value.Context
		default:
			return request, "", "", true, fmt.Errorf("invalid tag value")
		}
		request.Operation = app.TaskOperationSetTags
		if request.ContextSet {
			verb := "Set context on"
			if request.Context == "" {
				verb = "Cleared context on"
			}
			return request, "Set Tags & Context",
				fmt.Sprintf("Set Tags & Context · %s %d task(s)", verb, count), true, nil
		}
		return request, "Set Tags",
			fmt.Sprintf("Set Tags · Tagged %d task(s)", count), true, nil

	case operation.OperationTaskReferences:
		// Cardinality chooses the transaction, not the meaning, exactly as it does
		// for a move: one target is left to the task-line edit the `~task` sigil
		// writes, so a selection-free `~` stays byte-for-byte the edit it always
		// was, and only two or more take the batch.
		if count < 2 {
			return app.TaskOperationRequest{}, "", "", false, nil
		}
		value, ok := invocation.Value.(operation.OperationTaskReferenceValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid dependency value")
		}
		request.Operation = app.TaskOperationSetDependencies
		request.Dependencies = valuepicker.TaskReferenceBulkPatch(value)
		return request, "Set Dependencies",
			fmt.Sprintf("Set Dependencies · Updated %d task(s)", count), true, nil

	case operation.OperationTaskCopyTitle:
		request.Operation = app.TaskOperationCopyTitle
		return request, "Copy Title", "", true, nil

	case operation.OperationTaskCopyDetails:
		request.Operation = app.TaskOperationCopyDescription
		return request, "Copy Description", "", true, nil

	case operation.OperationTaskCopyTitleDescription:
		request.Operation = app.TaskOperationCopyTitleDescription
		return request, "Copy Title and Description", "", true, nil

	default:
		return app.TaskOperationRequest{}, "", "", false, nil
	}
}

func (m *AppModel) executeSingleTaskOperation(
	invocation operation.OperationInvocation,
	identity taskref.TaskIdentity,
) operation.OperationHandlerResult {
	cmdResult := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	relocationResult := func(cmd tea.Cmd) operation.OperationHandlerResult {
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		return cmdResult(withOutlinerRelocationCapture(cmd, capture))
	}

	switch invocation.Descriptor.ID {
	case operation.OperationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.Transition(identity, state))
	case operation.OperationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetPriority(identity, priority))
	case operation.OperationTaskSetDueDate:
		value, ok := invocation.Value.(operation.OperationDateValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid date value")}
		}
		if value.Deferred {
			return cmdResult(m.secondaryFeatures.TaskItem.Defer(identity, taskref.CloneOperationDate(value.Date)))
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetDueDate(identity, taskref.CloneOperationDate(value.Date)))
	case operation.OperationTaskSetResumeNote:
		// The empty string is a real value here, not a missing one: it is how the
		// note is cleared. So the type assertion is the whole check.
		note, ok := invocation.Value.(string)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid resume note value")}
		}
		// The retry request is rebuilt from the invocation rather than carried
		// through the mutation: ID and context are all the prompt factory reads,
		// and the value it would carry is exactly the one being retried.
		return cmdResult(m.setResumeNoteCmd(operation.OperationExecutionRequest{
			ID:      invocation.Descriptor.ID,
			Context: invocation.Context,
		}, identity, note))
	case operation.OperationTaskSetParent:
		value, ok := invocation.Value.(operation.OperationParentValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid parent value")}
		}
		return relocationResult(m.secondaryFeatures.TaskItem.Reparent(identity, value.ParentID))
	case operation.OperationTaskAdoptChildren:
		children, ok := invocation.Value.([]string)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid child value")}
		}
		return cmdResult(m.adoptChildrenCmd(
			identity,
			children,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	case operation.OperationTaskReferences:
		// No relocation capture: a reference edit records a soft link and moves no
		// row, so there is no destination for the outliner to follow.
		value, ok := invocation.Value.(operation.OperationTaskReferenceValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid dependency value")}
		}
		return cmdResult(m.setTaskReferencesCmd(identity, value))
	case operation.OperationTaskMoveToProject:
		projectID, ok := invocation.Value.(string)
		if !ok || strings.TrimSpace(projectID) == "" {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid project value")}
		}
		return cmdResult(m.moveTaskToProjectCmd(
			[]taskref.TaskIdentity{identity},
			projectID,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	case operation.OperationTaskIndent:
		return relocationResult(m.secondaryFeatures.TaskItem.Indent(identity))
	case operation.OperationTaskOutdent:
		return relocationResult(m.secondaryFeatures.TaskItem.Outdent(identity))
	default:
		return m.executeAuxiliaryTaskOperation(invocation, identity)
	}
}

// executeTaskDeleteOperation turns the frozen action-sheet targets into the one
// canonical delete request. The registry never mutates task data: confirmation
// and the atomic transaction stay in handleTaskDeleteRequested/confirmTaskDelete,
// which is why the binding delegates confirmation instead of adding a second,
// generic prompt in front of the canonical one.
func (m *AppModel) executeTaskDeleteOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	refs := make([]TaskProjectRef, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		task, project := m.taskAndProjectForIdentity(target.Task)
		if task == nil || project == nil {
			return operation.OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID),
		}
	}
	return operation.OperationHandlerResult{Message: TaskDeleteRequestedMsg{
		Refs:      refs,
		Selection: operationSelectionCounts(invocation.Context, len(refs)),
	}}
}

// operationSelectionCounts describes the targets an operation resolved against
// the projection the selection was frozen in. The frozen classification is
// trusted only when it still accounts for exactly the resolved targets;
// otherwise the operation reports what it actually has, all of it visible.
// Delete puts the split in its confirmation and copy puts it in its status line,
// so both read one classification rather than two.
func operationSelectionCounts(context operation.CommandContext, targetCount int) listsurface.SelectionCounts {
	counts := listsurface.SelectionCounts{Total: targetCount, Visible: targetCount}
	if context.Selection.VisibleCount+context.Selection.HiddenCount == targetCount {
		counts.Visible = context.Selection.VisibleCount
		counts.Hidden = context.Selection.HiddenCount
	}
	return counts
}

// TaskDeleteRequestedMsg is the canonical, shell-owned request to permanently
// delete one or more task subtrees. Any task surface emits it instead of
// running its own delete flow: the shell
// canonicalizes the roots, prompts for confirmation, and — only after the user
// confirms — executes ONE atomic BulkCoordinator.DeleteLocators transaction that folds
// the durable planning-ref cleanup into the same commit. Refs carry each root
// task with its owning project so heterogeneous cross-project selections delete
// together.
type TaskDeleteRequestedMsg struct {
	Refs      []TaskProjectRef
	Selection listsurface.SelectionCounts
}

type taskDeleteImpact struct {
	selectedRoots     int
	descendants       int
	totalTasks        int
	selection         listsurface.SelectionCounts
	storedAttachments int
	linkedAttachments int
}

// handleTaskDeleteRequested canonicalizes the requested roots, resolves the
// confirmation prompt, and arms the shell-owned confirm dialog. It never mutates
// task data — the delete transaction runs only after the user confirms in
// confirmTaskDelete. An empty or fully-invalid request is a no-op.
func (m *AppModel) handleTaskDeleteRequested(msg TaskDeleteRequestedMsg) {
	requested := canonicalizeTaskDeleteRoots(msg.Refs)
	if len(requested) == 0 {
		return
	}
	roots := collapseNestedTaskDeleteRoots(requested)
	selection := msg.Selection
	if selection.Total != len(requested) || selection.Visible+selection.Hidden != selection.Total {
		selection = listsurface.SelectionCounts{Total: len(requested), Visible: len(requested)}
	}
	impact := calculateTaskDeleteImpact(roots, selection)
	m.taskMutation.armDelete(roots, requested, impact)
}

// canonicalizeTaskDeleteRoots drops refs with a nil task/project or blank ID and
// removes duplicate roots by project-qualified identity, preserving first-seen
// order. Descendant resolution and nested-root collapsing happen later (in
// confirmTaskDelete and the application service); this only shapes the
// confirmation summary.
func canonicalizeTaskDeleteRoots(refs []TaskProjectRef) []TaskProjectRef {
	seen := make(map[taskref.TaskIdentity]bool, len(refs))
	roots := make([]TaskProjectRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Task == nil || ref.Project == nil || ref.Task.ID == "" {
			continue
		}
		identity := ref.Identity()
		if seen[identity] {
			continue
		}
		seen[identity] = true
		roots = append(roots, ref)
	}
	return roots
}

// collapseNestedTaskDeleteRoots removes a selected task when an ancestor in the
// same project is already selected. The ancestor carries that entire subtree,
// so these are the exact effective roots used by both impact accounting and the
// application transaction.
func collapseNestedTaskDeleteRoots(refs []TaskProjectRef) []TaskProjectRef {
	selected := make(map[taskref.TaskIdentity]bool, len(refs))
	for _, ref := range refs {
		selected[ref.Identity()] = true
	}
	roots := make([]TaskProjectRef, 0, len(refs))
	for _, ref := range refs {
		ancestorSelected := false
		seen := make(map[string]bool)
		parentID := ref.Task.ParentID
		for parentID != nil && *parentID != "" && !seen[*parentID] {
			seen[*parentID] = true
			if selected[taskref.TaskIdentity{ProjectID: ref.Project.ID, TaskID: *parentID}] {
				ancestorSelected = true
				break
			}
			parent := ref.Project.GetTaskByID(*parentID)
			if parent == nil {
				break
			}
			parentID = parent.ParentID
		}
		if !ancestorSelected {
			roots = append(roots, ref)
		}
	}
	return roots
}

func calculateTaskDeleteImpact(roots []TaskProjectRef, selection listsurface.SelectionCounts) taskDeleteImpact {
	rootSet := make(map[taskref.TaskIdentity]bool, len(roots))
	projects := make(map[string]*models.Project)
	for _, root := range roots {
		rootSet[root.Identity()] = true
		projects[root.Project.ID] = root.Project
	}
	removed := make(map[taskref.TaskIdentity]*models.Task)
	for projectID, project := range projects {
		for _, task := range project.Tasks {
			identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
			if taskDeleteRootAncestor(identity, task, project, rootSet) {
				removed[identity] = task
			}
		}
	}
	impact := taskDeleteImpact{
		selectedRoots: len(roots),
		totalTasks:    len(removed),
		selection:     selection,
	}
	impact.descendants = max(impact.totalTasks-impact.selectedRoots, 0)
	tasks := make([]*models.Task, 0, len(removed))
	for _, task := range removed {
		tasks = append(tasks, task)
	}
	impact.storedAttachments, impact.linkedAttachments = countAttachmentImpact(tasks)
	return impact
}

// countAttachmentImpact splits the attachments a deletion destroys from the ones
// it merely forgets. Every surface that warns about a delete reads this, so a
// stored file and a link can never be counted differently in two places.
func countAttachmentImpact(tasks []*models.Task) (stored, linked int) {
	for _, task := range tasks {
		for _, attachment := range task.Attachments {
			if attachment.IsURL || attachment.IsCodeRef {
				linked++
			} else {
				stored++
			}
		}
	}
	return stored, linked
}

func attachmentImpactNote(stored, linked int) string {
	return fmt.Sprintf(
		"attachments: %d stored file(s) deleted · %d URL/code reference(s) removed",
		stored,
		linked,
	)
}

func taskDeleteRootAncestor(
	identity taskref.TaskIdentity,
	task *models.Task,
	project *models.Project,
	roots map[taskref.TaskIdentity]bool,
) bool {
	seen := make(map[string]bool)
	for task != nil && !seen[task.ID] {
		seen[task.ID] = true
		identity.TaskID = task.ID
		if roots[identity] {
			return true
		}
		if task.ParentID == nil || *task.ParentID == "" {
			return false
		}
		task = project.GetTaskByID(*task.ParentID)
	}
	return false
}

func taskDeleteConfirmDetail(roots []TaskProjectRef, impact taskDeleteImpact) *chrome.ConfirmDetail {
	items := make([]string, 0, min(len(roots), 5))
	for _, root := range roots {
		if len(items) == 5 {
			break
		}
		items = append(items, fmt.Sprintf("%s:#%d %s", root.Project.Slug, root.Task.ShortID, root.Task.Title))
	}
	attachmentImpact := attachmentImpactNote(impact.storedAttachments, impact.linkedAttachments)
	return &chrome.ConfirmDetail{
		Items:      items,
		TotalCount: len(roots),
		CascadeNote: fmt.Sprintf(
			"%d selected root(s) · %d deduplicated descendant(s) · %d task(s) total · %d selected (%d visible · %d hidden) · %s",
			impact.selectedRoots,
			impact.descendants,
			impact.totalTasks,
			impact.selection.Total,
			impact.selection.Visible,
			impact.selection.Hidden,
			attachmentImpact,
		),
		UndoHint: true,
	}
}

// taskDeletePrompt renders "Delete task #N?" for a single root or
// "Delete N tasks?" for several, matching the canonical deletion grammar.
func taskDeletePrompt(roots []TaskProjectRef) string {
	if len(roots) == 1 {
		return fmt.Sprintf("Delete task #%d?", roots[0].Task.ShortID)
	}
	return fmt.Sprintf("Delete %d tasks?", len(roots))
}

// handleTaskDeleteConfirmKey gives the shell-owned task-delete confirmation the
// same modal key priority as project archive: while it is active it consumes
// every key (confirm on y/Y, cancel on n/N/Esc, swallow the rest) so nothing
// leaks into the underlying view. It returns handled=false only when no delete
// confirmation is pending.
func (m *AppModel) handleTaskDeleteConfirmKey(key string) (bool, tea.Cmd) {
	switch m.taskMutation.resolveDeleteKey(key) {
	case chrome.ConfirmAnswerConfirmed:
		return true, m.confirmTaskDelete()
	case chrome.ConfirmAnswerCanceled, chrome.ConfirmAnswerSwallowed:
		return true, nil
	case chrome.ConfirmAnswerUnarmed:
		return false, nil
	}
	return false, nil
}

// confirmTaskDelete executes the canonical deletion transaction for the pending
// roots. It runs ONE BulkCoordinator.DeleteLocators for both single- and multi-root
// deletion; the application service expands each subtree and folds canonical
// Today/Next cleanup into the checked project transaction. The
// returned command is decorated so the committed batch clears the requested
// selection IDs after the commit (finalizeStaleSelectionBatchResult passes a TransactionError
// through untouched, leaving state intact on failure). An artifact-build error
// aborts before any mutation and preserves the pending selection's task data.
func (m *AppModel) confirmTaskDelete() tea.Cmd {
	pending := m.taskMutation.takeDelete()
	roots := pending.roots
	selected := pending.selected
	if len(roots) == 0 {
		return nil
	}
	m.domain.service.WithPlanningArtifacts(coordinator.StructuralPlanningArtifacts(m.domain.runtime, m.domain.manager))

	locators := taskref.TaskLocatorsFromTaskRefs(roots)
	cmd := m.secondaryFeatures.Bulk.DeleteLocators(locators)
	if len(selected) == 0 {
		selected = roots
	}
	return coordinator.FinalizeStaleSelectionBatchResult(cmd, taskref.TaskLocatorsFromTaskRefs(selected))
}
