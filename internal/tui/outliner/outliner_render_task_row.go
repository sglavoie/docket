package outliner

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// taskRowSegments holds every rendered piece of a task row except the title, so
// the render path and the viewport measurement path derive the title budget from
// one formula instead of two that can drift apart.
type taskRowSegments struct {
	cursorPrefix, prefixStr, foldIndicator string
	statusIcon, prioritySegment, pinMarker string
	indicators, relationMarker, matchCue   string
	attachmentMarker, descriptionMarker    string
	// projectLabel names the task's project where no heading above it does: the
	// ranked list, which draws no group headers, and the one row a merged
	// singleton group draws in place of its header. Context follows that label,
	// or stands alone where a project header already names the project.
	projectLabel string
	projectFirst bool
}

func (s taskRowSegments) titleWidth(rowWidth int) int {
	return max(rowWidth-outlinerNonTitleWidth(
		s.cursorPrefix, s.prefixStr, s.foldIndicator,
		s.statusIcon, s.prioritySegment, s.projectLabel, s.pinMarker,
		s.indicators, s.relationMarker, s.attachmentMarker, s.descriptionMarker, s.matchCue,
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
	return s.gutter() + s.contentPrefix()
}

// contentPrefix keeps rendering and the inline editor on the same title column.
func (s taskRowSegments) contentPrefix() string {
	if s.projectFirst {
		return s.projectLabel + s.statusIcon + " " + s.prioritySegment + s.pinMarker
	}
	return s.statusIcon + " " + s.prioritySegment + s.projectLabel + s.pinMarker
}

// content keeps semantic colors under the cursor; the match cue closes the row.
func (s taskRowSegments) content(title string) string {
	return s.contentPrefix() + title + s.descriptionMarker + s.attachmentMarker + s.relationMarker + s.indicators + s.matchCue
}

// taskDescriptionMarker keeps nonblank description presence visible even when
// optional indicators yield to the title. Peek carries the description itself.
func taskDescriptionMarker(task *models.Task) string {
	if task == nil || strings.TrimSpace(task.Description) == "" {
		return ""
	}
	return theme.MutedStyle.Render(" ▤")
}

// taskAttachmentMarker reserves a presence cue even when the row sheds its
// optional indicators. Counts and attachment details remain in Peek.
func taskAttachmentMarker(task *models.Task) string {
	if task == nil || !task.HasAttachments() {
		return ""
	}
	return theme.MutedStyle.Render(" 📎")
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
	// One unstyled space, then the chip, whose own padding supplies the second
	// space the marker always led with.
	return " " + theme.RelationMarkerStyle.Render(marker)
}

// rankedProjectLabelMaxSlug bounds the slug a ranked row spells: an eighth of
// the row, never under twelve columns and never over twenty-four, so a narrow
// row keeps its title and a wide one spells a long slug out. It is the one
// source of that cap; the render and ComputeOutlinerRowLayout both read it.
func rankedProjectLabelMaxSlug(rowWidth int) int {
	return min(max(rowWidth/8, 12), 24)
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
	return m.abbreviatedProjectSlug(slug, rankedProjectLabelMaxSlug(rowWidth))
}

// taskIdentityLabel puts context directly after the project, before alignment
// padding. Rows under a project header carry only their own context.
func (m *OutlinerModel) taskIdentityLabel(row OutlinerRow, rowWidth int) string {
	label := ""
	if m.rowLeadsWithProjectLabel(row) {
		if slug := m.rankedProjectSlug(row, rowWidth); slug != "" {
			_, project := m.projectSlugForRow(row)
			label = theme.ProjectLabel(project, slug)
		}
	}
	if row.Task != nil {
		if context := strings.TrimSpace(row.Task.Context); context != "" {
			if label != "" {
				label += " "
			}
			context = textfit.TruncateToVisualWidth(context, max(rowWidth/3, 1))
			label += theme.TaskContextBadgeStyle(row.Task).Render("[" + context + "]")
		}
	}
	return label
}

func (m *OutlinerModel) taskProjectLabel(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout) string {
	label := m.taskIdentityLabel(row, rowWidth)
	if colLayout != nil {
		width := colLayout.ContextWidth
		if m.rowLeadsWithProjectLabel(row) {
			width = colLayout.ProjectWidth
		}
		label += strings.Repeat(" ", max(width-lipgloss.Width(label), 0))
	}
	if row.LeadsMergedGroup {
		_, project := m.projectSlugForRow(row)
		label = m.projectHeaderPinMarker(project) + label
	}
	if label == "" {
		return ""
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
	// Reserve a recognizable phrase before spending columns on supporting
	// metadata. Short titles need only their actual width. Deadlines retain the
	// smaller emergency floor below, so gaining title space cannot hide urgency.
	target := min(lipgloss.Width(row.Task.Title), min(32, max(12, rowWidth/3)))
	return m.buildTaskRowSegmentsFor(row, rowWidth, colLayout, paint, target, indicatorDropLadder)
}

// buildTaskRowSegmentsFor is buildTaskRowSegments with the title width the
// shedding defends named by the caller: the match cue and then the indicators
// along indicatorDropLadder go while the title budget is under titleTarget.
func (m *OutlinerModel) buildTaskRowSegmentsFor(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout, paint cursorPaint, titleTarget int, ladder []func(*taskIndicatorSegments)) taskRowSegments {
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
	prefix := compactOutlinerTreePrefix(row.Prefix, prefixBudget)
	if row.LeadsMergedGroup {
		// The merged row replaces a project header, so its label starts in
		// that header's column. Keep the task's fold arrow in the same gutter.
		prefix = ""
		if foldIndicator == "" {
			foldIndicator = gutterSpace(bg) + gutterSpace(bg)
		}
	}
	prefixStr, foldIndicator := renderTaskPrefix(prefix, foldIndicator, bg, m.treeConnectorStyle(row))

	segments := taskRowSegments{
		cursorPrefix:      m.renderTaskCursorColumn(row, paint),
		prefixStr:         prefixStr,
		foldIndicator:     foldIndicator,
		statusIcon:        statusIcon,
		prioritySegment:   prioritySegment,
		projectLabel:      m.taskProjectLabel(row, rowWidth, colLayout),
		projectFirst:      row.LeadsMergedGroup,
		pinMarker:         m.taskPinnedMarker(row),
		attachmentMarker:  taskAttachmentMarker(task),
		descriptionMarker: taskDescriptionMarker(task),
		indicators:        indicatorSegments.render(),
		relationMarker:    m.taskRelationMarker(row),
		matchCue:          taskMatchSourceCue(row),
	}

	// Preserve a readable title before secondary metadata on narrow/deep rows.
	// Queue membership, multi-select state, tree position, and workflow state
	// remain; lower-priority badges are progressively removed as space tightens.
	// The match cue goes first: it explains a result the title itself cannot
	// show, which is worth nothing once that title is unreadable. It is also the
	// only segment a row wears solely because a search is live, so shedding it
	// leaves an ordinary row's degradation exactly as it was.
	if segments.titleWidth(rowWidth) < titleTarget {
		segments.matchCue = ""
	}
	for index, dropSegment := range ladder {
		target := titleTarget
		if index == len(indicatorDropLadder)-1 {
			// The deadline is the last rung, and yields only when even a short
			// title would be unreadable. The preview's ladder never reaches it.
			target = min(target, 8)
		}
		if segments.titleWidth(rowWidth) >= target {
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

// RenderOutlinerPreviewTaskRow draws one task row for a read-only glance at a
// project — Select Project's preview pane — where the title is what the row is
// read for and nothing below it can be opened: no cursor stands on it and no
// Peek answers it. It is RenderOutlinerTaskRow with the shedding target raised
// from a readable title to the whole one, so indicatorDropLadder empties, in its
// own order, before a title is cut — except the deadline, which is the one
// indicator a glance at a project is for: an overdue date outranks the end of a
// title, so the preview cuts the title rather than drop it.
func (m *OutlinerModel) RenderOutlinerPreviewTaskRow(
	row OutlinerRow,
	rowWidth, rowIndex int,
	colLayout *chrome.ColumnLayout,
) string {
	task := row.Task
	titleStyle := m.renderTaskTitleStyle(task)
	paint := m.cursorPaintFor(false).overTaskRow()
	titleTarget := min(8, max(rowWidth/4, 4))
	if task != nil {
		titleTarget = max(titleTarget, lipgloss.Width(task.Title))
	}
	segments := m.buildTaskRowSegmentsFor(row, rowWidth, colLayout, paint, titleTarget, previewIndicatorDropLadder)
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
