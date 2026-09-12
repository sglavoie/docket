package outliner

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The relationship trail is the way back out of a hop the picker took. It is a
// stack of its own rather than a reuse of the jump history or the scope frames:
// a hop replaces the whole projection — scope, mode, facets, sort, focus, zoom,
// and the live lens — and only one restoration point can hand all of that back
// at once.
//
// A frame holds presentation and identity and nothing else. The checkpoint is
// Session 3's and by contract holds no task or project pointer; the lens frame
// travels as the pointer the model was already holding, so its identity,
// fingerprint, baseline, and population provenance come along without being
// copied and without a second reading that could disagree. The trail itself is
// deliberately not inside the checkpoint: a lens frame carries a checkpoint of
// its own, and a trail nested in one would embed itself.

// relationReturnFrame is one hop of that trail.
type relationReturnFrame struct {
	presentation OutlinerPresentationCheckpoint
	// lens is the live lens the hop suspended, or nil when none was live. It is
	// suspended rather than dismissed: nothing machine-local was written on the way
	// out, so putting it back needs no persistence either.
	lens *activeLensFrame
	// from is the task the hop was taken from, so the return parks the cursor on
	// the row the picker was opened over rather than on a restored row index that
	// a rebuild may have moved.
	from taskref.TaskIdentity
}

// HasRelationTrail reports an outstanding one-hop return.
func (m *OutlinerModel) HasRelationTrail() bool {
	return m != nil && len(m.relationTrail) > 0
}

// ClearRelationTrail drops every outstanding return, reporting whether there was
// one. A global reset discards the trail whatever else it finds to clear: the
// destination may look like the launch view already, and a trail left standing
// behind it would make the next Esc jump somewhere the view no longer names.
func (m *OutlinerModel) ClearRelationTrail() bool {
	if m == nil || len(m.relationTrail) == 0 {
		return false
	}
	m.relationTrail = nil
	return true
}

// PopRelationReturn is one hop back: the view the jump left, the lens it
// suspended, and the cursor on the task the picker was opened over. The picker
// is not reopened — the return is to a view, and a list of relationships is not
// part of one.
func (m *OutlinerModel) PopRelationReturn() bool {
	if m == nil || len(m.relationTrail) == 0 {
		return false
	}
	frame := m.relationTrail[len(m.relationTrail)-1]
	m.relationTrail = m.relationTrail[:len(m.relationTrail)-1]
	m.closeRelationPicker()
	// The lens is installed before the presentation, because the checkpoint owns
	// the narrowing query the frame reads and restoring it second would write an
	// empty query over the lens that was just put back.
	m.Lens.setActive(frame.lens)
	m.RestoreOutlinerPresentationCheckpoint(frame.presentation)
	// The scope the frame names may have been archived, parked, or deleted while
	// the user was away. Reinstating it as written would leave the return standing
	// in a project the workspace no longer holds, with no rows and no cursor, so
	// the repair the reload path already owns is run over the restored axes. The
	// notice is transient because a return raises no Cmd of its own: the Esc
	// cascade that drove it has no channel to publish one on.
	if notice, repaired := m.reconcileProjectScope(); repaired {
		m.SetTransientStatus(notice)
	}
	if !m.MoveCursorToIdentity(frame.from) {
		// The checkpoint restore already seated the nearest surviving row, which is
		// the fallback a vanished source is owed; the clamp only keeps that landing
		// inside a list a rebuild may have shortened.
		m.ClampCursor()
	}
	m.rebaseReloadTargetAfterRelocation()
	return true
}

// jumpToRelation opens the target's project and reveals the task, publishing a
// return frame only once that succeeded. It reports a refusal, or the empty
// string. A refusal restores the presentation it captured, so a hop that cannot
// land leaves neither the view nor the trail changed.
func (m *OutlinerModel) jumpToRelation(task *models.Task, project *models.Project) string {
	if m == nil || task == nil || project == nil {
		return relationStaleTargetRefusal
	}
	if m.Project != nil && m.Project.ID != project.ID {
		return "This view holds one project and cannot open " + project.Name
	}
	frame := relationReturnFrame{
		presentation: m.CaptureOutlinerPresentationCheckpoint(),
		lens:         m.Lens.takeActive(),
		from:         m.RelationPicker.Subject,
	}
	// forgetActiveLensFrame is the suspension seam: it drops the frame, the
	// application still reading a population for it, the query edit, and the
	// narrowing that frame installed, and it touches the machine-local resume
	// pointer not at all. Neither RestoreLensBaseline nor ClearActiveLensWithoutRestore
	// would do: both commit that pointer, and a hop is not a lens being put down.
	m.forgetActiveLensFrame()
	if !m.revealRelationTarget(task, project) {
		m.Lens.setActive(frame.lens)
		m.RestoreOutlinerPresentationCheckpoint(frame.presentation)
		return "That task is not reachable in " + project.Name
	}
	m.relationTrail = append(m.relationTrail, frame)
	// A read started before the hop is carrying the origin's cursor. It stays in
	// flight, because the hop resolved inside the population it is about to
	// refresh, but it is re-aimed at the target: a result honoring the old capture
	// would land the user back where they pressed the key.
	m.rebaseReloadTargetAfterRelocation()
	return ""
}

// revealRelationTarget opens the target's project in All and puts the cursor on
// it, reporting whether it got there.
//
// The project population filters are deliberately left alone: they decide which
// projects are in memory, the hop resolves strictly inside that loaded set, and
// widening them would make a jump reload the workspace. Only the row facets are
// replaced, by the at most two predicates the target itself needs.
func (m *OutlinerModel) revealRelationTarget(task *models.Task, project *models.Project) bool {
	return m.revealTaskForNavigation(task, project, true)
}

// revealTaskForNavigation exposes a hidden task in All. A relationship hop
// also enters the destination project's scope; a parent jump stays in the
// current scope, because its parent belongs to the same project as the child.
func (m *OutlinerModel) revealTaskForNavigation(task *models.Task, project *models.Project, scopeProject bool) bool {
	identity := taskref.For(task, project)
	if !identity.Valid() {
		return false
	}
	m.PeekMode = false
	m.ClearSearch()
	for m.PopFocusLevel() {
	}
	m.ExitSectionZoom()
	m.Sort.Reset()
	m.TreeSort.Reset()
	clauses := make([]models.QueryClause, 0, 2)
	if task.IsTerminalState() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, task.State().String())
	}
	if task.IsDeferred() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldDeferred,
			models.QueryOperatorPresent, "")
	}
	m.ReplaceFilterClauses(query.FilterQueryTaskFields(), clauses)
	if scopeProject {
		// A relationship hop retains an explicitly focused parked project: its
		// return trail needs that project still loaded when Esc restores the origin.
		m.pushProjectScopeRetainingParkedFocus(project.ID)
	}
	m.SelectOutlinerMode(chrome.ModeAll)
	m.RebuildRows()
	if index, found := m.RowIndexForTask(identity); found {
		m.Cursor = index
		return true
	}
	return m.expandFoldsRevealing(task, identity)
}

// expandFoldsRevealing opens the folds hiding one task and puts every other one
// back. It runs only after a first attempt failed, so an unrelated fold is
// disturbed only for as long as the search takes.
//
// It opens every section and reads the row's own headers off the rebuilt list
// rather than spelling their keys from the task: All groups by lane, a lane is
// not derivable from a task alone, and a surface that grew a lane would leave a
// spelled key naming nothing.
func (m *OutlinerModel) expandFoldsRevealing(task *models.Task, identity taskref.TaskIdentity) bool {
	saved := copyBoolMap(m.Folds.CollapsedSections)
	m.Folds.clearSections()
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.Folds.setNode(taskref.TaskIdentity{ProjectID: identity.ProjectID, TaskID: ancestor.ID}, true)
	}
	m.RebuildRows()
	index, found := m.RowIndexForTask(identity)
	if !found {
		m.Folds.setSections(saved)
		m.RebuildRows()
		return false
	}
	restored := copyBoolMap(saved)
	for _, key := range m.enclosingCollapseKeys(index) {
		delete(restored, key)
	}
	m.Folds.setSections(restored)
	m.RebuildRows()
	if index, found = m.RowIndexForTask(identity); !found {
		// Nothing between the row and its two headers can fold it, so reaching here
		// means the rows moved under a fold this pass did not read. The open list is
		// what the hop promised, and it is kept.
		m.Folds.clearSections()
		m.RebuildRows()
		if index, found = m.RowIndexForTask(identity); !found {
			m.Folds.setSections(saved)
			m.RebuildRows()
			return false
		}
	}
	m.Cursor = index
	return true
}

// enclosingCollapseKeys names the section and project-group headers one row sits
// under, nearest first. It stops at the first of each: a header further up
// cannot fold the row without folding one of those two first.
func (m *OutlinerModel) enclosingCollapseKeys(index int) []string {
	keys := make([]string, 0, 2)
	haveSection, haveProject := false, false
	for i := index - 1; i >= 0 && (!haveSection || !haveProject); i-- {
		row := m.Rows[i]
		switch {
		// The focused-project header folds nothing, so it can hide nothing and is
		// not one of the two headers this walk has to reopen.
		case row.IsProjectHeader && row.IsCollapsibleHeader() && !haveProject:
			haveProject = true
		case row.IsSectionHeader && !haveSection:
			haveSection = true
		default:
			continue
		}
		keys = append(keys, row.EffectiveCollapseKey())
	}
	return keys
}
