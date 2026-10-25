package outliner

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// reviewAdvanceExhaustedStatus is what a review pass says when it has run out of
// visible work. It is the acknowledgement's own answer to the keystroke rather
// than an advisory, so it is a status line and not an AdvisoryLine.
const reviewAdvanceExhaustedStatus = "No further visible review candidates"

// PlanReviewAdvance turns an ordinary relocation capture into a review
// acknowledgement's: the successors the cursor may land on are recorded now,
// from the rows the user is looking at, because the acknowledgement itself is
// what makes the source stop being a candidate and a list rebuilt after the
// commit can no longer be asked "who was next".
//
// The sequence is ordered and deduplicated by identity, with the source seeded
// into the seen set. Every row builder as it stands draws an identity once — a
// promoted lane moves a task rather than copying it — so neither guard fires
// today. Both are here because the sequence is consumed after a rebuild the
// planning code cannot see, and landing on a second copy of a row the user has
// already answered is not advancing.
//
// Candidacy is taskOwedAttention: the automatic question, so a pass
// visits what raised its hand — a flag, or a check-in whose day has come — and
// not everything a person could choose to acknowledge. An early review is still
// allowed by name on the task under the cursor; it is only the advancing that
// skips a schedule whose day is still ahead. Acknowledging always takes the
// source out of that set, so a pass ends rather than circling.
//
// Eligibility is read here and again when the cursor lands. Recording it twice
// is not redundancy: this reading is what keeps an ineligible row out of the
// sequence, and the later one is what drops a candidate the commit or another
// surface changed while the transaction was in flight.
func (m *OutlinerModel) PlanReviewAdvance(capture *OutlinerRelocationCapture) {
	if m == nil || capture == nil {
		return
	}
	capture.ReviewAdvance = true
	now := time.Now()
	seen := map[taskref.TaskIdentity]bool{capture.Task: true}
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		if !identity.Valid() || seen[identity] {
			continue
		}
		seen[identity] = true
		if !taskOwedAttention(row.Task, m.reviewRowProject(row), now) {
			continue
		}
		capture.ReviewSuccessors = append(capture.ReviewSuccessors, identity)
	}
}

// taskOwedAttention is the attention question every review reader asks:
// models.TaskNeedsAttention over work that is not effectively hidden. It reads
// no facet, so the Review lane, its count, and a review pass skip hidden work
// even where fH has put it on screen, since hidden work is never owed attention.
func taskOwedAttention(task *models.Task, project *models.Project, now time.Time) bool {
	return models.TaskOwesReviewAttention(project, task, now)
}

// reviewRowProject resolves the project a row's task is read inside, the way
// TaskIdentityForRow does, so the ancestor walk sees the task's own tree.
func (m *OutlinerModel) reviewRowProject(row OutlinerRow) *models.Project {
	if m.Project != nil {
		return m.Project
	}
	return m.crossResolveProject(row.ProjectID)
}

// finishReviewAdvance seats the cursor on the first recorded successor that is
// still both visible and still asking for attention, and reports exhaustion when
// none is.
// It writes m.Cursor and nothing else: ComputeOutlinerViewport stays the single
// window authority, folds are left as they were, and no project is loaded to
// find a candidate the projection does not already hold. There is no wrap —
// a pass that reaches the bottom is over, not restarted.
//
// Every caller sits behind FinishOutlinerRelocation's projection guard, so mode,
// scope, search, filters, sort, zoom, folds, and focus are already known to be
// the ones the sequence was recorded against. The mode's own cursor-advance
// catalog flag is deliberately not consulted: that flag governs following a task
// that left its lane, while a review pass is a sweep the user asked for by name
// and works the same in every mode.
//
// Nothing is pushed onto jump history. An acknowledgement moves the cursor, not
// the task, so there is no vacated origin for `Ctrl+O` to return to.
func (m *OutlinerModel) finishReviewAdvance(capture *OutlinerRelocationCapture) tea.Cmd {
	if m == nil || capture == nil {
		return nil
	}
	now := time.Now()
	for _, successor := range capture.ReviewSuccessors {
		index, ok := m.RowIndexForTask(successor)
		if !ok {
			continue
		}
		row := m.Rows[index]
		if row.Task == nil || !taskOwedAttention(row.Task, m.reviewRowProject(row), now) {
			continue
		}
		m.Cursor = index
		return nil
	}
	// The source is preferred over whatever the reload's nearest-row
	// reconciliation chose, because the user is still looking at the task they
	// just acknowledged and it is usually still on screen. When it is not — a
	// flagged row leaving the Review lane, say — that reconciliation already ran
	// and its answer stands.
	if index, ok := m.RowIndexForTask(capture.Task); ok {
		m.Cursor = index
	}
	return func() tea.Msg { return status.InfoStatusMsg(reviewAdvanceExhaustedStatus) }
}
