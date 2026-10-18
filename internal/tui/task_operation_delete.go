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
	"github.com/sglavoie/tt/internal/util"
)

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

// attachmentImpactParts names what a deletion does to attachments, and nothing
// when it touches none: a zero count warns about a consequence that is not
// coming.
func attachmentImpactParts(stored, linked int) []string {
	var parts []string
	if stored > 0 {
		parts = append(parts, util.Count(stored, "stored file")+" deleted")
	}
	if linked > 0 {
		parts = append(parts, util.Count(linked, "link")+" removed")
	}
	return parts
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
	return &chrome.ConfirmDetail{
		Items:       items,
		TotalCount:  len(roots),
		CascadeNote: taskDeleteCascadeNote(impact),
		UndoHint:    true,
	}
}

// taskDeleteCascadeNote warns about what the listed titles do not already say:
// the subtasks going with them, the total when it differs from one, the
// selected tasks the view is not drawing, and the attachments. A plain task
// with none of those deletes with no warning line at all.
func taskDeleteCascadeNote(impact taskDeleteImpact) string {
	var parts []string
	if impact.descendants > 0 {
		parts = append(parts, util.Count(impact.descendants, "subtask")+" also deleted")
	}
	if impact.totalTasks > 1 {
		parts = append(parts, util.Count(impact.totalTasks, "task")+" total")
	}
	if impact.selection.Hidden > 0 {
		parts = append(parts, fmt.Sprintf("%d selected (%d visible · %d hidden)",
			impact.selection.Total, impact.selection.Visible, impact.selection.Hidden))
	}
	parts = append(parts, attachmentImpactParts(impact.storedAttachments, impact.linkedAttachments)...)
	return strings.Join(parts, " · ")
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
//
// This is also the seam that knows the gesture's cardinality, so it is the one
// that names the committed focus target: the batch delete is the only route for
// both one and many roots, and neither the coordinator nor the removed-locator
// evidence can tell a row the user pointed at from a descendant that went with
// it. A gesture naming several tasks stamps nothing and stays uncorrelated.
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
	cmd = coordinator.StampCommittedFocusTarget(cmd, soleTaskDeleteTarget(selected))
	return coordinator.FinalizeStaleSelectionBatchResult(cmd, taskref.TaskLocatorsFromTaskRefs(selected))
}

// soleTaskDeleteTarget names the one task a deletion gesture pointed at, read
// off the requested refs rather than the collapsed roots: a selection holding a
// parent and its own child collapses to one root, but the user named two tasks
// and that is a multi-target gesture. Any other cardinality returns the zero
// locator, which leaves the batch unstamped and the transaction uncorrelated.
func soleTaskDeleteTarget(requested []TaskProjectRef) app.TaskLocator {
	if len(requested) != 1 {
		return app.TaskLocator{}
	}
	return requested[0].Locator()
}
