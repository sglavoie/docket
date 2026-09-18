package outliner

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// taskRowSegments holds every rendered piece of a task row except the title, so
// the render path and the viewport measurement path derive the title budget from
// one formula instead of two that can drift apart.
type taskRowSegments struct {
	cursorPrefix, checkbox, prefixStr, foldIndicator string
	statusIcon, prioritySegment, pinMarker           string
	indicators, relationMarker, matchCue             string
	// projectField is the trailing project column: one separating space, the
	// compact label, and the blanks that carry it out to the projection's own
	// width. A row that names no project inside a projection that does reserves
	// the field blank, so the indicators after it stay in their column too.
	projectField string
	// cueColumns is the shared width of the title and the cues that travel with
	// it, which is what the project field is aligned by. Zero means this row is
	// packed left — no projection-wide column was measured, or this row is too
	// deeply indented to seat one — and the field follows its cues immediately.
	cueColumns int
}

// preTitleWidth is every column drawn before the title: the structural gutter,
// the badges, and the one space after the state badge.
func (s taskRowSegments) preTitleWidth() int {
	return lipgloss.Width(s.cursorPrefix) + lipgloss.Width(s.checkbox) +
		lipgloss.Width(s.prefixStr) + lipgloss.Width(s.foldIndicator) +
		lipgloss.Width(s.statusIcon) + 1 +
		lipgloss.Width(s.prioritySegment) + lipgloss.Width(s.pinMarker)
}

// postTitleWidth is every column drawn after the title: the cues that travel
// with it, the trailing project field, and the operational indicators.
func (s taskRowSegments) postTitleWidth() int {
	return lipgloss.Width(s.relationMarker) + lipgloss.Width(s.matchCue) +
		lipgloss.Width(s.projectField) + lipgloss.Width(s.indicators)
}

// titleWidth is the columns the title text itself may spend. Under a shared
// column it is that column's budget less the cues seated inside it, so the
// title is what gives way when a row wears more cues than its neighbors; with
// no column it is the row's own remainder, which is what a preview, a
// standalone render, and a projection naming no project all take.
func (s taskRowSegments) titleWidth(rowWidth int) int {
	if s.cueColumns > 0 {
		return max(s.cueColumns-lipgloss.Width(s.relationMarker)-lipgloss.Width(s.matchCue), 1)
	}
	return max(rowWidth-s.preTitleWidth()-s.postTitleWidth(), 1)
}

// naturalCueEnd is the column this row's title and cues end in when the row is
// packed left, which is where its project field would sit with no column to
// answer to. The projection's column is the maximum of these, so the column
// lands exactly where the longest title in view already ended.
func (s taskRowSegments) naturalCueEnd(rowWidth int, title string) int {
	return s.preTitleWidth() +
		min(lipgloss.Width(title), s.titleWidth(rowWidth)) +
		lipgloss.Width(s.relationMarker) + lipgloss.Width(s.matchCue)
}

// gutter is the leading structural region the cursor highlight owns: the left
// margin, the multi-select box, the tree connectors and the fold arrow. It is a
// concatenation only — the seam changes nothing about how wide each piece is, so
// titleWidth stays the single title-budget authority.
func (s taskRowSegments) gutter() string {
	return s.cursorPrefix + s.checkbox + s.prefixStr + s.foldIndicator
}

// titlePrefix is every column drawn before the title: the structural gutter and
// the leading badges. It is the row's own arithmetic rather than a second copy of
// it, so the inline editor opens its input in the column the title occupied and
// sizes it to the columns the title actually had. A widened title budget reaches
// the editor by construction, which is the whole point of naming this once.
// Nothing trailing belongs here: the project field is drawn past the title now,
// so the caret lands on the title's own first column.
func (s taskRowSegments) titlePrefix() string {
	return s.gutter() + s.statusIcon + " " + s.prioritySegment + s.pinMarker
}

// content is everything from the state badge on, the region that keeps its own
// colors under the cursor. The space after the badge is the one preTitleWidth
// counts. The title and its cues are padded out to the shared column as one
// run, so a row wearing a relation marker or a match cue seats its project
// field in the same column as the row beside it that wears neither.
func (s taskRowSegments) content(title string) string {
	cues := title + s.relationMarker + s.matchCue
	if s.cueColumns > 0 {
		if pad := s.cueColumns - lipgloss.Width(cues); pad > 0 {
			cues += strings.Repeat(" ", pad)
		}
	}
	return fmt.Sprintf("%s %s%s%s%s%s",
		s.statusIcon,
		s.prioritySegment,
		s.pinMarker,
		cues,
		s.projectField,
		s.indicators,
	)
}

// taskRelationMarker names the family the outline placed elsewhere: `‹` for a
// promoted root whose parent is not shown above it, `›N` for a row whose N open
// children are all seated somewhere else. It is reserved in taskRowSegments
// rather than offered the title's leftover columns, so it survives every width
// instead of vanishing exactly when the row is most crowded. Both marks share
// one segment so a row that is both keeps the reserve near three columns. The
// ancestor path itself, and the children, stay Peek's.
func (m *OutlinerModel) taskRelationMarker(row OutlinerRow) string {
	marker := ""
	if row.Breadcrumb != "" {
		marker = "‹"
	}
	// A row showing any child already says it is a parent through its connectors
	// and fold indicator; the count would only restate them.
	if !row.HasVisibleChildren {
		if open := m.openChildCounts[m.rowIdentity(row)]; open > 0 {
			marker += fmt.Sprintf("›%d", open)
		}
	}
	if marker == "" {
		return ""
	}
	return theme.MutedStyle.Render("  " + marker)
}

// projectLabelWidth is every column the compact project label may spend at the
// app's own minimum width, brackets included. The ceiling is what makes the
// label compact rather than a heading laid sideways: the heading it replaced
// had a row to itself and could spell the whole slug, while this rides in a
// column beside a title that still has to be readable. Twelve characters of
// slug is where the two stop trading against each other at 60 columns — long
// enough that an ordinary slug arrives whole and a long one is cut somewhere
// recognizable, and short enough that the widest titles the row is meant to
// carry still arrive whole.
const projectLabelWidth = 14

const (
	// projectLabelCeilingWidth is the widest the label may ever grow to. Past
	// this the column is spelling a name nobody is reading twice, and the
	// columns belong to the titles.
	projectLabelCeilingWidth = 24

	// projectLabelGrowthFloorWidth is the row width below which the label does
	// not grow at all. Growth is what a wide row can spare, never what an
	// ordinary one is charged: the fixture titles this product is measured
	// against still arrive whole at 120 columns, and they only do so because the
	// label spends nothing extra until the row is already wider than the work.
	projectLabelGrowthFloorWidth = 100

	// projectLabelGrowthStep is the columns of row width above that floor that
	// buy one column of label. A wide terminal is where a truncated slug is
	// least excusable, so the ceiling rises with the row — slowly, because the
	// label is one answer and the title is the work.
	projectLabelGrowthStep = 10
)

// projectLabelBudget is the compact label's ceiling at one row width, brackets
// included. It reads the row rather than the terminal because the row is what
// the label has to fit inside, so a scrollbar or a pane border narrows the
// label exactly as it narrows everything else on the row.
func projectLabelBudget(rowWidth int) int {
	grown := projectLabelWidth + max(rowWidth-projectLabelGrowthFloorWidth, 0)/projectLabelGrowthStep
	return min(grown, projectLabelCeilingWidth)
}

// taskProjectLabel names the project owning one cross-project task row.
// groupsRowsByProject is the same predicate appendProjectGroupHeader reads, so
// the label appears in exactly the projections that used to open a heading over
// the group: the identity moves onto the rows rather than being stated twice.
// Both single-project forms stay silent, because their chrome already names the
// project and a label there would only repeat it.
//
// It is reserved at every width, like the pin and relation markers, instead of
// joining indicatorDropLadder. A row whose project is unknown is a row whose
// title may mean two different things, and no deadline makes up for that; the
// ceiling above is how the reserve stays affordable at the narrow widths where
// the ladder is already running.
//
// It is the bare label rather than a placed one: taskProjectField is what lays
// it into the projection's column, so the row's one truncation rule stays here
// and the column's arithmetic stays there.
//
// A row seated under its own parent is silent for the same reason the pinned
// lane's rows carry no pin: parents are project-local, so the connectors above
// have already answered the question, and the columns a nested row can least
// afford go to its title. A promoted root whose parent is elsewhere is not
// seated under it, so it keeps the label.
func (m *OutlinerModel) taskProjectLabel(row OutlinerRow, rowWidth int) string {
	if !m.groupsRowsByProject() || row.ProjectID == "" {
		return ""
	}
	if row.Task != nil && row.Task.ParentID != nil && row.Breadcrumb == "" {
		return ""
	}
	project := m.ResolveProjectForRow(row)
	label := row.ProjectID
	if project != nil {
		label = grammar.ProjectLabel(project)
	}
	label = textfit.TruncateToVisualWidth(label, projectLabelBudget(rowWidth)-2)
	return theme.ProjectColorStyle(project).Render("[" + label + "]")
}

// taskProjectField lays one row's label into the projection's trailing column:
// the separating space the title is never allowed to eat, the label, and the
// blanks out to the widest label in view. A row naming no project inside a
// projection that names one reserves the field blank rather than dropping it,
// because the indicators sit after the field and a dropped field would leave
// them ragged. With no measured column — a preview, a standalone render, a
// projection where no row names a project — the field is exactly what the row
// itself carries.
func taskProjectField(label string, colLayout *chrome.ColumnLayout) string {
	width := 0
	if colLayout != nil {
		width = colLayout.ProjectWidth
	}
	if width <= 0 {
		if label == "" {
			return ""
		}
		return " " + label
	}
	return " " + label + strings.Repeat(" ", max(width-lipgloss.Width(label), 0))
}

// taskPinnedMarker names a pin the surrounding rows cannot: the Pinned lane's
// own rows carry the header instead, exactly as a row showing its child carries
// no child count.
func (m *OutlinerModel) taskPinnedMarker(row OutlinerRow) string {
	if row.Task == nil || !row.Task.Pinned || row.IsPinnedSection {
		return ""
	}
	return listsurface.PinnedMarker()
}

// matchSourceLabels is the one presentation vocabulary for search attribution:
// each source the projection can report maps to exactly one word. A title hit is
// absent deliberately — the highlighted title is already the explanation — as is
// matchSourceNone, which every ordinary row carries.
var matchSourceLabels = map[MatchSource]string{
	MatchSourceDescription: "description",
	MatchSourceResume:      "resume",
	MatchSourceChecklist:   "checklist",
	MatchSourceComment:     "comment",
	MatchSourceTag:         "tag",
	MatchSourceContext:     "context",
	MatchSourceAttachment:  "attachment",
	MatchSourceProject:     "project",
	MatchSourceDetails:     "details",
	MatchSourceNote:        "note",
}

// taskMatchSourceCue explains why a row whose title shows no match is in the
// results. It is one segment so ANSI-aware measurement charges it once, and it
// is muted so it reads as an annotation rather than competing with the semantic
// title color beside it.
func taskMatchSourceCue(row OutlinerRow) string {
	label, ok := matchSourceLabels[row.MatchSource]
	if !ok {
		return ""
	}
	return theme.MutedStyle.Render("  · " + label)
}

// buildTaskRowSegments assembles every rendered piece of a task row except the
// title. paint is how the row wears the cursor, and the zero value on an
// ordinary row; it changes no piece's width, so a caller measuring the title
// budget can pass the zero value.
func (m *OutlinerModel) buildTaskRowSegments(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout, paint cursorPaint) taskRowSegments {
	task := row.Task
	bg := paint.gutterBg()
	foldIndicator := m.renderTaskFoldIndicator(row, bg)
	statusIcon := m.renderTaskStatusIcon(task)
	if colLayout != nil && colLayout.HasStatus {
		statusIcon = theme.PadBadge(statusIcon, colLayout.StatusWidth)
	}
	prioritySegment := m.renderTaskPrioritySegment(task, colLayout)
	// One instant for the whole row. The ladder and the review segment ask the
	// same calendar-day question, so reading the clock twice could demote a badge
	// for a review the row does not name.
	now := time.Now()
	indicatorSegments := m.renderTaskIndicatorSegments(row, now)

	// Von Restorff: one row asks for attention with one badge. The ladder in
	// chrome.PickHeroAttribute decides whether priority is that badge, and the
	// priority badge quietens when something more time-sensitive is on the row.
	// The state badge is not in this: its symbol says the workflow state and its
	// semantic accent identifies the kind of work, so neither is demoted with
	// lower-priority attention signals.
	hero := chrome.PickHeroAttribute(task, now)
	if hero != chrome.HeroNone && !chrome.PriorityIsHero(hero) && prioritySegment != "" {
		if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
			prioritySegment = chrome.DemoteBadge(trimmed) + " "
		}
	}
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.Prefix, prefixBudget), foldIndicator, bg)

	segments := taskRowSegments{
		cursorPrefix:    paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix()),
		checkbox:        m.renderTaskCheckbox(row, bg),
		prefixStr:       prefixStr,
		foldIndicator:   foldIndicator,
		statusIcon:      statusIcon,
		prioritySegment: prioritySegment,
		pinMarker:       m.taskPinnedMarker(row),
		indicators:      indicatorSegments.render(),
		relationMarker:  m.taskRelationMarker(row),
		matchCue:        taskMatchSourceCue(row),
		projectField:    taskProjectField(m.taskProjectLabel(row, rowWidth), colLayout),
	}

	// Preserve a readable title before secondary metadata on narrow/deep rows.
	// Queue membership, multi-select state, tree position, and workflow state
	// remain; lower-priority badges are progressively removed as space tightens.
	titleTarget := min(8, max(rowWidth/4, 4))
	// The match cue goes first: it explains a result the title itself cannot
	// show, which is worth nothing once that title is unreadable. It is also the
	// only segment a row wears solely because a search is live, so shedding it
	// leaves an ordinary row's degradation exactly as it was.
	if segments.titleWidth(rowWidth) < titleTarget {
		segments.matchCue = ""
	}
	for _, dropSegment := range indicatorDropLadder {
		if segments.titleWidth(rowWidth) >= titleTarget {
			break
		}
		dropSegment(&indicatorSegments)
		segments.indicators = indicatorSegments.render()
	}

	// The shared column is applied after the ladder rather than inside it, so
	// the measurement pass and this one shed the same indicators from the same
	// row: the ladder answers what the row can afford at its own width, and the
	// column answers where the answer is drawn.
	//
	// The projection reserves trailing space for every row's indicators. Keep
	// the local width clamp for standalone callers, and let deeply indented rows
	// leave the column when aligning would make their title unreadable.
	if colLayout != nil && colLayout.ProjectColumn > 0 {
		columns := min(
			colLayout.ProjectColumn-segments.preTitleWidth(),
			segments.titleWidth(rowWidth)+
				lipgloss.Width(segments.relationMarker)+lipgloss.Width(segments.matchCue),
		)
		if columns >= minSharedCueColumns {
			segments.cueColumns = columns
		}
	}
	return segments
}

// minSharedCueColumns is the title-and-cue budget below which a row leaves the
// shared column rather than honoring it. A deeply nested row can start past
// where the projection's column sits, and a column that truncated its title to
// two glyphs to keep one label aligned would have spent the row on the answer
// nobody was reading.
const minSharedCueColumns = 8

// RenderOutlinerTaskRow draws one task row, and exactly one line of it. The
// detail behind the row is the dock's, below the list, so the renderer takes the
// row's own column budget and nothing else: there is no second width for a pane
// that no longer hangs off the row.
func (m *OutlinerModel) RenderOutlinerTaskRow(
	row OutlinerRow,
	selected bool,
	rowWidth, rowIndex int,
	colLayout *chrome.ColumnLayout,
) string {
	task := row.Task
	titleStyle := m.renderTaskTitleStyle(task)
	paint := m.cursorPaintFor(selected).overTaskRow()
	segments := m.buildTaskRowSegments(row, rowWidth, colLayout, paint)
	title := m.renderTaskTitle(task, titleStyle, rowWidth, segments)

	return applyTaskRowLineStyle(segments.gutter(), segments.content(title), task, paint, rowWidth, rowIndex)
}

// rowIdentity resolves a row's project-qualified task identity, supplying the
// scoped project ID that single-project rows omit.
func (m *OutlinerModel) rowIdentity(row OutlinerRow) taskref.TaskIdentity {
	identity := taskref.TaskIdentity{ProjectID: row.ProjectID}
	if row.Task != nil {
		identity.TaskID = row.Task.ID
	}
	if identity.ProjectID == "" && m.Project != nil {
		identity.ProjectID = m.Project.ID
	}
	return identity
}

// renderTaskFoldIndicator reports the fold `h`/`l` would act on next. Children
// answer first because the ladder collapses the checklist before them, so a task
// whose only foldable content is its checklist is the one that reports the
// checklist's own state.
func (m *OutlinerModel) renderTaskFoldIndicator(row OutlinerRow, bg color.Color) string {
	task := row.Task
	if task == nil {
		return ""
	}
	identity := m.rowIdentity(row)
	var expanded bool
	switch {
	case row.HasVisibleChildren:
		expanded = m.IsExpanded(identity)
	case len(task.Checklist) > 0:
		expanded = m.IsChecklistExpanded(identity)
	default:
		return ""
	}
	if expanded {
		return gutterPiece(bg, theme.MutedStyle, theme.IconExpanded) + gutterSpace(bg)
	}
	return gutterPiece(bg, theme.MutedStyle, theme.IconCollapsed) + gutterSpace(bg)
}

func (m *OutlinerModel) renderTaskStatusIcon(task *models.Task) string {
	return theme.GetTaskStateBadge(task)
}

// renderTaskTitleStyle resolves an Outliner title's color, which for every
// non-terminal task is ordinary text: attention travels on the badges and
// indicators beside the title, never on the title itself. It passes no
// CursorSelected either, because the cursor is marked by the gutter and the
// trailing fill, so the row under it reads exactly like the rows around it.
func (m *OutlinerModel) renderTaskTitleStyle(task *models.Task) lipgloss.Style {
	return theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
}

// taskIndicatorSegments is the whole of what a row says beside its title,
// badges, and structural marks. It is three segments rather than eleven because
// a row is scanned, not read: the deadline and the owed review are the two
// facts that change what a reader does next, and the reference census is the
// one cue that work the row cannot show may be unreadable or unfinished.
// Everything a row used to carry beside them — tags, context, elapsed ages,
// deferral dates, and the checklist, comment and attachment counts — is spelled
// in full in Peek, one keypress below the cursor.
type taskIndicatorSegments struct {
	review string
	due    string
	// dependencies is the trailing `~S/N` reading of outgoing soft references.
	// N is a count of what the task stores, not a reading of what those targets
	// are: a reference to closed, deleted, or unloaded work counts exactly like a
	// reference to open work, because nothing about the subject changes either
	// way. S and the color are the aggregate reading of the targets themselves,
	// one hop and never further; Peek remains the one place those targets are
	// named.
	dependencies string
}

func (segments taskIndicatorSegments) render() string {
	rendered := listsurface.JoinGroupParts(
		segments.review,
		segments.due,
		segments.dependencies,
	)
	if rendered == "" {
		return ""
	}
	return " " + rendered
}

// indicatorDropLadder orders which indicators a narrow row sheds, shallowest
// signal first. The reference census goes first because Peek names every target
// in full and no deadline or owed review depends on it; the owed review follows,
// because it says a task wants looking at while the deadline says when something
// is actually due. The deadline goes last: it is the final commitment width
// pressure may hide.
var indicatorDropLadder = []func(*taskIndicatorSegments){
	func(segments *taskIndicatorSegments) { segments.dependencies = "" },
	func(segments *taskIndicatorSegments) { segments.review = "" },
	func(segments *taskIndicatorSegments) { segments.due = "" },
}

// renderTaskIndicatorSegments promotes the row's three surviving operational
// signals: the deadline, the owed review, and the outgoing-reference census.
// Every other detail the row once abbreviated is Peek's, in full. The caller
// drops segments along indicatorDropLadder when preserving all three would leave
// an unreadably small title.
func (m *OutlinerModel) renderTaskIndicatorSegments(row OutlinerRow, now time.Time) taskIndicatorSegments {
	var segments taskIndicatorSegments
	task := row.Task
	if task == nil {
		return segments
	}
	segments.review = formatReviewIndicator(task, now)
	// A deadline remains a deadline in every lifecycle state. Check-ins have
	// their own schedule and indicator, so Waiting no longer reinterprets a due
	// date as a check-back or suppresses the ordinary due chip.
	if m.columnConfig.IsVisible(listsurface.ColDueDate) {
		segments.due = query.FormatDueDate(task)
	}
	// Deliberately ungated by a column: a reference count is not a column the
	// user can hide, and naming one would put a dependency back in the column
	// vocabulary the retired hard-dependency panels owned.
	segments.dependencies = renderDependencyIndicator(task, m.dependencyIndex())
	return segments
}

// renderDependencyIndicator renders the trailing outgoing-reference count as
// `~S/N`: N is every stored locator regardless of what it resolves to, so the
// number a row shows is exactly the number of tokens the edit grammar would
// round-trip and is independent of the task's own workflow state, and S is how
// many of them are settled. A wholly settled set collapses to the bare `~N`,
// and a trailing `?` says at least one locator points into a project this view
// did not load, so the shape now carries what only the color used to. The color
// is the census's own outlook, over the same index Peek resolves its detail
// lines against, so the row and Peek can never disagree. Nothing here adds or
// drops a reference.
//
// The index behind this is built once per row-rebuild pass rather than per
// reference per row per frame, which is strictly cheaper than the linear scan it
// replaced. It cannot go stale under a target's state change: changing a
// target's state is a mutation, and a mutation sets the force-rebuild flag that
// drops the index.
func renderDependencyIndicator(task *models.Task, index *app.DependencyIndex) string {
	if task == nil || len(task.Dependencies) == 0 {
		return ""
	}
	census := DependencyCensusFor(task, index)
	count := "~" + census.counts()
	if census.unreadable > 0 {
		count += "?"
	}
	return census.Outlook().style().Render(count)
}

// formatReviewIndicator announces a task that is asking to be looked at, which
// models.TaskNeedsAttention answers and this only spells: the stored review flag,
// or a scheduled check-in whose calendar day has arrived and whose task is
// eligible to raise it. A future schedule belongs in Peek, while a terminal task
// stays silent and a deferred one suppresses its schedule. The row and the
// attention ladder read one predicate, so a row can never demote its priority
// badge for a review it does not name.
//
// The three tiers share deadline emphasis without borrowing deadline meaning:
// the word review keeps the signals distinct even when both occur on one row.
// The bare `review` is the flag alone — someone marked this task, so there is no
// day to be late against — and it stays as long as the flag does, since `=` set
// it and `R` is what takes it off.
func formatReviewIndicator(task *models.Task, now time.Time) string {
	if !models.TaskNeedsAttention(task, now) {
		return ""
	}
	if !task.CheckInIsDueAt(now) {
		return theme.DueTodayStyle.Render("review")
	}
	daysLate := util.DaysBetween(*task.NextCheckIn, now)
	if daysLate == 0 {
		return theme.DueTodayStyle.Render("review today")
	}
	return theme.OverdueStyle.Render(fmt.Sprintf("review +%dd", daysLate))
}

const repeatedDeferralThreshold = 3

func taskHasRepeatedDeferrals(task *models.Task) bool {
	return task != nil && task.DeferCount >= repeatedDeferralThreshold
}

// taskWokeRecently reports whether a lapsed deferral is still inside the wake
// window. Peek's dates line is its one reader now that the row carries no
// deferral of its own, and it stays a named predicate because the pending and
// lapsed halves of a deferral are one ladder: `defer <date>` until the instant
// passes, then `woke <age>` for the window, then nothing.
func taskWokeRecently(task *models.Task, now time.Time) bool {
	if task == nil || task.DeferredUntil == nil || task.IsDeferredAt(now) {
		return false
	}
	return now.Sub(*task.DeferredUntil) < query.DeferWakeWindow
}

func (m *OutlinerModel) renderTaskPrioritySegment(task *models.Task, colLayout *chrome.ColumnLayout) string {
	priorityBadge := chrome.RenderPriorityBadge(task.Priority)
	if colLayout != nil && colLayout.HasPriority {
		return theme.PadBadge(priorityBadge, colLayout.PriorityWidth) + " "
	}
	if priorityBadge == "" {
		return ""
	}
	return priorityBadge + " "
}

func (m *OutlinerModel) renderTaskCheckbox(row OutlinerRow, bg color.Color) string {
	if !m.MultiSelectMode {
		return ""
	}
	if m.isRowSelected(row) {
		return gutterPiece(bg, theme.SelectionCheckStyle, "[x]") + gutterSpace(bg)
	}
	return gutterPiece(bg, theme.MutedStyle, "[ ]") + gutterSpace(bg)
}

func compactOutlinerTreePrefix(prefix string, maxWidth int) string {
	if maxWidth <= 0 || lipgloss.Width(prefix) <= maxWidth {
		return prefix
	}
	runes := []rune(prefix)
	tailWidth := max(maxWidth-1, 0)
	if tailWidth == 0 {
		return "…"
	}
	if tailWidth > len(runes) {
		tailWidth = len(runes)
	}
	return "…" + string(runes[len(runes)-tailWidth:])
}

func renderTaskPrefix(prefix, foldIndicator string, bg color.Color) (string, string) {
	// A leaf connector already communicates its tree position with ├──/└──.
	// Do not extend it into the task-content gutter.
	return gutterPiece(bg, theme.MutedStyle, prefix), foldIndicator
}

func (m *OutlinerModel) renderTaskTitle(task *models.Task, titleStyle lipgloss.Style, rowWidth int, segments taskRowSegments) string {
	availableWidth := segments.titleWidth(rowWidth)
	title := textfit.TruncateToVisualWidth(task.Title, availableWidth)
	highlight, caseSensitive := m.titleHighlightQuery()
	if highlight == "" {
		return titleStyle.Render(title)
	}
	positions := query.SearchSubstringPositions(title, highlight, caseSensitive)
	if len(positions) == 0 {
		return titleStyle.Render(title)
	}
	return textfit.HighlightText(title, positions, titleStyle)
}

// titleHighlightQuery names the free text the visible rows were narrowed by,
// with the case mode they were actually produced under. Ordinary search and a
// lens narrowing are the same act to a title: whichever one owns the current
// projection is what a highlight must read, so the two can never disagree.
func (m *OutlinerModel) titleHighlightQuery() (string, bool) {
	if query := m.searchTextQuery(); query != "" {
		return query, m.searchCaseSensitive()
	}
	if !m.HasActiveLens() {
		return "", false
	}
	return m.LensTextQuery(), m.LensCaseSensitive()
}

// taskRowTint names the row-level background a task earns, and a missed deadline
// is the only thing that earns one. It is the loudest mark the list has — a
// whole row repainted — so it is spent on the top of the attention ladder alone.
// Waiting held one until the ladder was consolidated: every Waiting row wore an
// alert background whether or not anything was owed on it, which made the
// quietest work in the workspace look like the most urgent. Waiting now reads
// through its state badge and its `waiting <age>` segment, and a Waiting task
// with a missed deadline is tinted for the deadline like any other task.
//
// It is returned rather than applied because the row the cursor is on wears the
// cursor's band instead: one row may carry one background, and on the row you
// are standing on the answer is where you are. Nothing about the deadline is
// lost to that — the row keeps its bolded due badge, its `!` hero emphasis and
// its overdue text color, which are the operational indicators urgency was
// always read from and the only ones that survive --no-color anyway. Move off
// the row and the tint is back.
func taskRowTint(task *models.Task) (lipgloss.Style, bool) {
	if task.IsOverdue() {
		return theme.OverdueRowStyle, true
	}
	return lipgloss.Style{}, false
}

func applyTaskRowLineStyle(gutter, content string, task *models.Task, paint cursorPaint, rowWidth, rowIndex int) string {
	if paint.marked {
		// Background precedence, stated once: the cursor's band owns the marked row
		// and neither the overdue tint nor the zebra stripe is applied over or under
		// it. outlinerCursorRow lays it under the content it was handed, so every
		// foreground that content carries reads on top of it unchanged.
		return outlinerCursorRow(gutter, content, rowWidth, paint)
	}
	tint, tinted := taskRowTint(task)

	// renderTaskTitle floors the title budget at 10 cols, so a deeply-nested row
	// (long tree prefix + multi-select checkbox) can assemble wider than rowWidth.
	// The row style below applies Width(rowWidth), which *wraps* an over-wide line
	// to a second terminal row — but computeOutlinerViewport budgets one line per
	// row, so the extra wrapped rows overflow terminal height. Clamp to rowWidth
	// (ANSI-aware) so every row stays exactly one line. outlinerCursorRow clamps
	// the cursor row the same way, against the columns its gutter leaves.
	line := textfit.TruncateToVisualWidth(gutter+content, rowWidth)
	if tinted {
		return tint.Width(rowWidth).Render(line)
	}
	return chrome.ZebraLine(line, rowWidth, rowIndex)
}

// PeekDocumentBodyForRow is one row's complete detail document, wrapped to the
// dock's own indent and width. It is the whole of it rather than the window on
// screen: the dock reaches the window through Lines, and this is the seam for
// naming a row's detail directly — what Peek can be scrolled to, independent of
// where the cursor is and of what the frame currently shows.
func (m *OutlinerModel) PeekDocumentBodyForRow(row OutlinerRow) []string {
	if m == nil || !rowSupportsPeek(row) {
		return nil
	}
	// The body is the whole document, so the geometry stamped onto it changes
	// nothing here; the frame's own is passed so this seam names no second window.
	return m.inlinePeekDocumentForRow(row, peekDockIndent, m.peekDockWidth(), m.peekDockGeometry()).Body
}
