package outliner

import (
	"fmt"
	"image/color"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
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
	cursorPrefix, prefixStr, foldIndicator string
	statusIcon, prioritySegment, pinMarker string
	indicators, relationMarker, matchCue   string
	// projectLabel names the task's project where no heading above it does: the
	// ranked list, which draws no group headers, and the one row a merged
	// singleton group draws in place of its header. It is empty everywhere else.
	projectLabel string
}

func (s taskRowSegments) titleWidth(rowWidth int) int {
	return max(rowWidth-outlinerNonTitleWidth(
		s.cursorPrefix, s.prefixStr, s.foldIndicator,
		s.statusIcon, s.prioritySegment, s.projectLabel, s.pinMarker,
		s.indicators, s.relationMarker, s.matchCue,
	), 1)
}

// gutter is the leading structural region the cursor highlight owns: the left
// margin, which is also where a selected row wears its mark, the tree connectors
// and the fold arrow. It is a
// concatenation only — the seam changes nothing about how wide each piece is, so
// titleWidth stays the single title-budget authority.
func (s taskRowSegments) gutter() string {
	return s.cursorPrefix + s.prefixStr + s.foldIndicator
}

// titlePrefix is every column drawn before the title: the structural gutter and
// the leading badges. It is the row's own arithmetic rather than a second copy of
// it, so the inline editor opens its input in the column the title occupied and
// sizes it to the columns the title actually had. A widened title budget reaches
// the editor by construction, which is the whole point of naming this once.
func (s taskRowSegments) titlePrefix() string {
	return s.gutter() + s.statusIcon + " " + s.prioritySegment + s.projectLabel + s.pinMarker
}

// content is everything from the state badge on, the region that keeps its own
// colors under the cursor. The space after the badge is the one outlinerNonTitleWidth
// counts.
// The match cue closes the row, behind the indicators, as it closes a note row:
// the indicators are the row's own facts and keep one column across a result
// list, while the cue is a remark about the search and comes and goes per row.
func (s taskRowSegments) content(title string) string {
	return fmt.Sprintf("%s %s%s%s%s%s%s%s",
		s.statusIcon,
		s.prioritySegment,
		s.projectLabel,
		s.pinMarker,
		title,
		s.relationMarker,
		s.indicators,
		s.matchCue,
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
	return theme.RelationMarkerStyle.Render("  " + marker)
}

// rankedProjectLabelMaxSlug bounds the slug a ranked row spells: a fifth of the
// row, never under twelve columns and never over twenty-four, so a long slug is
// cut at the floor and spelled out where the row has the room. It is the one
// source of that cap; the render and ComputeOutlinerRowLayout both read it.
func rankedProjectLabelMaxSlug(rowWidth int) int {
	return min(max(rowWidth/5, 12), 24)
}

// showsRankedProjectLabel reports whether every task row leads its title with
// the project: the ranked list across several projects, the one cross-project
// projection whose rows no group header stands over.
func (m *OutlinerModel) showsRankedProjectLabel() bool {
	return m.ShowsRankedList() && m.groupsRowsByProject()
}

// rowLeadsWithProjectLabel reports whether one task row leads its title with
// the project: every row of the ranked list, or the row leading a merged
// singleton group, which no header stands over either.
func (m *OutlinerModel) rowLeadsWithProjectLabel(row OutlinerRow) bool {
	return row.LeadsMergedGroup || m.showsRankedProjectLabel()
}

// rankedProjectSlug is the bare slug a ranked row names, bounded to the label's
// ceiling so the render and ComputeOutlinerRowLayout measure one string.
func (m *OutlinerModel) rankedProjectSlug(row OutlinerRow, rowWidth int) string {
	if row.ProjectID == "" {
		return ""
	}
	slug, _ := m.projectSlugForRow(row)
	return textfit.TruncateToVisualWidth(slug, rankedProjectLabelMaxSlug(rowWidth))
}

// taskProjectLabel is the `[slug]` the project header would have drawn, in the
// same bold project color, padded to the projection's widest label so every
// labeled title starts in one column.
func (m *OutlinerModel) taskProjectLabel(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout) string {
	if !m.rowLeadsWithProjectLabel(row) {
		return ""
	}
	slug := m.rankedProjectSlug(row, rowWidth)
	if slug == "" {
		return ""
	}
	_, project := m.projectSlugForRow(row)
	label := theme.ProjectColorStyle(project).Bold(true).Render("[" + slug + "]")
	if colLayout != nil {
		label += strings.Repeat(" ", max(colLayout.ProjectWidth-lipgloss.Width(label), 0))
	}
	return label + " "
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
	foldIndicator := m.renderTaskFoldIndicator(row, colLayout, bg)
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
	// The state badge is not in this: its symbol says the workflow state, which is
	// identity, so it is never demoted with lower-priority attention signals.
	// Inside the Review lane a flag-only review demotes priority though the row
	// drops the bare word, since the lane heading is what names it there.
	hero := chrome.PickHeroAttribute(task, now)
	if hero != chrome.HeroNone && !chrome.PriorityIsHero(hero) && prioritySegment != "" {
		if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
			prioritySegment = chrome.DemoteBadge(trimmed) + " "
		}
	}
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefixStr, foldIndicator := renderTaskPrefix(compactOutlinerTreePrefix(row.Prefix, prefixBudget), foldIndicator, bg, m.treeConnectorStyle(row))

	segments := taskRowSegments{
		cursorPrefix:    m.renderTaskCursorColumn(row, paint),
		prefixStr:       prefixStr,
		foldIndicator:   foldIndicator,
		statusIcon:      statusIcon,
		prioritySegment: prioritySegment,
		projectLabel:    m.taskProjectLabel(row, rowWidth, colLayout),
		pinMarker:       m.taskPinnedMarker(row),
		indicators:      indicatorSegments.render(),
		relationMarker:  m.taskRelationMarker(row),
		matchCue:        taskMatchSourceCue(row),
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
	return segments
}

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
	title := m.renderTaskTitle(row, titleStyle, rowWidth, segments)

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

// rowDrawsFold reports whether a task row has anything `h`/`l` could fold: a
// child the projection shows, or a checklist. It is the one predicate the glyph
// and the projection-wide fold column both read.
func rowDrawsFold(row OutlinerRow) bool {
	return row.Task != nil && (row.HasVisibleChildren || len(row.Task.Checklist) > 0)
}

// renderTaskFoldIndicator reports the fold `h`/`l` would act on next. Children
// answer first because the ladder collapses the checklist before them, so a task
// whose only foldable content is its checklist is the one that reports the
// checklist's own state. A row with nothing to fold keeps the column blank when
// the projection holds one that has, so every title starts in one column; a nil
// layout reserves nothing.
func (m *OutlinerModel) renderTaskFoldIndicator(row OutlinerRow, colLayout *chrome.ColumnLayout, bg color.Color) string {
	task := row.Task
	if task == nil {
		return ""
	}
	if !rowDrawsFold(row) {
		if colLayout != nil && colLayout.HasFold {
			return gutterSpace(bg) + gutterSpace(bg)
		}
		return ""
	}
	identity := m.rowIdentity(row)
	expanded := m.IsChecklistExpanded(identity)
	if row.HasVisibleChildren {
		expanded = m.IsExpanded(identity)
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
	// The deadline and the review are chips carrying a column of padding each
	// side; the census is bare text, so it is given the leading column a chip
	// would have brought and sits the same distance from whatever precedes it.
	census := segments.dependencies
	if census != "" {
		census = " " + census
	}
	rendered := listsurface.JoinGroupParts(
		segments.review,
		segments.due,
		census,
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
	// The Review lane already names a flag-only review, so a row there drops the
	// bare word and keeps the tiers that carry a day.
	if row.PromotedTag == reviewLaneKey && !task.CheckInIsDueAt(now) {
		segments.review = ""
	}
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
// the word review keeps the signals distinct even when both occur on one row,
// and a late one is amber rather than the teal of a deadline.
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
	return theme.ReviewLateStyle.Render(fmt.Sprintf("review +%dd", daysLate))
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

// selectionMark is what a member of the visual selection wears in the row's
// leading column. It is one column and takes the cursor column rather than one
// of its own, so entering a selection moves no title; it is a glyph rather than
// a color, so it reads reversed under the cursor's bar and in a profile carrying
// no color at all.
const selectionMark = "✓"

// renderTaskCursorColumn draws the row's leading column: the selection mark on
// a member of the visual selection, a blank on every other row, and the cursor's
// bar over whichever it is.
func (m *OutlinerModel) renderTaskCursorColumn(row OutlinerRow, paint cursorPaint) string {
	if m.MultiSelectMode && m.isRowSelected(row) {
		return paint.bar(theme.SelectionCheckStyle, selectionMark)
	}
	return paint.bar(lipgloss.NewStyle(), theme.RenderCursorPrefix())
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

func renderTaskPrefix(prefix, foldIndicator string, bg color.Color, style lipgloss.Style) (string, string) {
	// A leaf connector already communicates its tree position with ├──/└──.
	// Do not extend it into the task-content gutter.
	return gutterPiece(bg, style, prefix), foldIndicator
}

// treeConnectorStyle paints a row's tree connectors in its project's color, so
// the tree reads as belonging to the header above it; an uncolored or
// unresolved project keeps the muted default ProjectColorStyle falls back to.
func (m *OutlinerModel) treeConnectorStyle(row OutlinerRow) lipgloss.Style {
	return theme.ProjectColorStyle(m.ResolveProjectForRow(row))
}

func (m *OutlinerModel) renderTaskTitle(row OutlinerRow, titleStyle lipgloss.Style, rowWidth int, segments taskRowSegments) string {
	availableWidth := segments.titleWidth(rowWidth)
	title := row.Task.Title
	if lipgloss.Width(title) > availableWidth {
		title = m.compactTitleForRow(title)
	}
	title = textfit.TruncateToVisualWidth(title, availableWidth)
	highlight, caseSensitive := m.titleHighlightQuery()
	if highlight == "" {
		return titleStyle.Render(title)
	}
	positions := query.SearchSubstringPositions(title, highlight, caseSensitive)
	// A title the matcher admitted as a subsequence holds no contiguous run to
	// paint, and a title hit earns no cue, so without this the row would explain
	// itself nowhere. The positions are the matcher's own, as Peek's excerpt
	// reads them, and only for a row the projection attributed to its title.
	if len(positions) == 0 && row.MatchSource == MatchSourceTitle {
		_, positions = fuzzy.NewMatcher(caseSensitive).ScoreTermsWithPositions(title, highlight)
	}
	if len(positions) == 0 {
		return titleStyle.Render(title)
	}
	return textfit.HighlightText(title, positions, titleStyle)
}

// compactTitleForRow is the row's display form of a title too wide for it: a
// URL keeps its host and first path segment, and a path under the home
// directory keeps its last segment. It runs before the cut, so a search
// highlight paints the compacted text the row draws. It is the row's alone —
// Peek, the editors, the form, and exports read the stored title whole.
func (m *OutlinerModel) compactTitleForRow(title string) string {
	return compactRowTitle(title, m.homeDir)
}

// compactRowTitle compacts each space-separated word of title that is an
// http(s) URL or an absolute path under home, leaving the rest as written.
// Punctuation wrapped around a word, such as a closing parenthesis or a full
// stop, stays outside the compacted form.
func compactRowTitle(title, home string) string {
	words := strings.Split(title, " ")
	for i, word := range words {
		core := strings.TrimLeft(word, "(<[\"'")
		lead := word[:len(word)-len(core)]
		core = strings.TrimRight(core, ".,;:!?)>]\"'")
		trail := word[len(lead)+len(core):]
		if core == "" {
			continue
		}
		if compact, ok := compactTitleURL(core); ok {
			words[i] = lead + compact + trail
		} else if compact, ok := compactTitleHomePath(core, home); ok {
			words[i] = lead + compact + trail
		}
	}
	return strings.Join(words, " ")
}

// compactTitleURL renders an http(s) URL as its host, without `www.`, then its
// first path segment, then `/…` when anything follows it.
func compactTitleURL(word string) (string, bool) {
	u, err := url.Parse(word)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	compact := strings.TrimPrefix(u.Host, "www.")
	var segments []string
	for _, segment := range strings.Split(u.EscapedPath(), "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) > 0 {
		compact += "/" + segments[0]
	}
	if len(segments) > 1 || u.RawQuery != "" || u.Fragment != "" {
		compact += "/…"
	}
	return compact, true
}

// compactTitleHomePath renders an absolute path under home as `~/…/<last>`,
// or as `~/<last>` when the last segment sits directly in home.
func compactTitleHomePath(word, home string) (string, bool) {
	if home == "" {
		return "", false
	}
	sep := string(filepath.Separator)
	if word == home || word == home+sep {
		return "~", true
	}
	rest, ok := strings.CutPrefix(word, home+sep)
	if !ok {
		return "", false
	}
	var segments []string
	for _, segment := range strings.Split(rest, sep) {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	switch len(segments) {
	case 0:
		return "~", true
	case 1:
		return "~" + sep + segments[0], true
	default:
		return "~" + sep + "…" + sep + segments[len(segments)-1], true
	}
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

func outlinerNonTitleWidth(cursorPrefix, prefixStr, foldIndicator, statusIcon, prioritySegment, projectLabel, pinMarker, indicators, relationMarker, matchCue string) int {
	return lipgloss.Width(cursorPrefix) +
		lipgloss.Width(prefixStr) + lipgloss.Width(foldIndicator) +
		lipgloss.Width(statusIcon) + 1 +
		lipgloss.Width(prioritySegment) + lipgloss.Width(projectLabel) + lipgloss.Width(pinMarker) +
		lipgloss.Width(indicators) +
		lipgloss.Width(relationMarker) + lipgloss.Width(matchCue)
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
	// (a long tree prefix) can assemble wider than rowWidth.
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
