package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ZebraLine applies a subtle background tint to odd-indexed rows when zebra
// striping is enabled. It is a no-op on even rows, disabled config, or when
// the line is already selected/dimmed/flashed (caller handles those cases).
func ZebraLine(line string, width int, rowIndex int) string {
	if !zebraEnabled || rowIndex%2 == 0 {
		return line
	}
	return ZebraRowStyle.Width(width).Render(line)
}

// TaskLine holds the configuration for rendering a single task line.
// Callers set the fields they need and call Render() to produce the line string.
// The core badge sequence (status → type → priority → title) is always rendered.
type TaskLine struct {
	Task       *models.Task
	Project    *models.Project // optional; enables hero attribute (blocked-by-deps) selection
	IsSelected bool
	MaxTitle   int // maximum title width; clamped to minimum 10

	// Optional elements
	Queued          bool          // show "Q " queue indicator in accent color (else "  "); superseded by DailyPlanned
	DailyPlanned    bool          // show "w " daily-plan indicator in accent color; takes precedence over Queued
	Active          bool          // show * active marker in cursor prefix
	Pinned          bool          // show · pinned marker in cursor prefix
	Prefix          string        // after queue indicator (e.g., star indicator "★ ")
	PreBadges       string        // after cursor, before status badge (e.g., project name, checkbox, rank)
	PostTitle       string        // after title (e.g., tags, score badge, section)
	DueStr          string        // right-aligned due date string (pre-styled by caller)
	ViewWidth       int           // total view width for due date right-alignment
	StatusBadge     string        // override computed status badge (pre-styled by caller)
	HideLowPriority bool          // only show priority badge for priority ≤ 2
	Flash           bool          // cursor flash highlight active
	SearchQuery     string        // when set, highlight matching substring in title
	RowIndex        int           // visual row index for zebra striping (0-based)
	Layout          *ColumnLayout // when non-nil, use fixed-width badge columns
}

// CrossProjectTaskLineOpts configures optional elements for a cross-project task line.
// Views that aggregate tasks from multiple projects use this to render a standard
// [project-name] badge plus optional tags/score in a consistent format.
type CrossProjectTaskLineOpts struct {
	ShowScore       bool          // show composite score badge (requires cfg != nil)
	ShowTags        bool          // show inline tag string
	ShowRecurrence  bool          // show ↻ indicator with streak count for recurring tasks
	ShowDescription bool          // show description-indicator icon when task has a non-empty description
	MaxProjectLen   int           // project name truncation; 0 defaults to 12
	Queued          bool          // show "Q " queue indicator in accent color (else "  "); superseded by DailyPlanned
	DailyPlanned    bool          // show "w " daily-plan indicator in accent color; takes precedence over Queued
	Prefix          string        // prepended before the project badge (e.g. checkbox, date string)
	StatusBadge     string        // pre-styled status badge override
	DueStr          string        // right-aligned context string, already styled by caller
	HideLowPriority bool          // omit priority badge when priority > 2
	Flash           bool          // cursor flash highlight active
	SearchQuery     string        // when set, highlight matching substring in title
	BadgeColor      color.Color   // when set, overrides the project identity color on the [project-name] badge
	RowIndex        int           // visual row index for zebra striping (0-based)
	Layout          *ColumnLayout // when non-nil, use fixed-width badge columns
}

// RenderCrossProjectTaskLine renders a standard task line for views that aggregate
// tasks from multiple projects. It builds the [project] badge, optionally appends
// tags and a score badge, and delegates to TaskLine.Render().
//
// Use opts.Prefix to prepend content before the project badge (e.g. a checkbox or
// a pre-styled date string). Use opts.DueStr for the right-aligned context column.
// For non-standard badge arrangements, construct TaskLine directly instead.
func RenderCrossProjectTaskLine(
	task *models.Task,
	proj *models.Project,
	runtime *config.RuntimeState,
	isCursor bool,
	width, maxTitleLen int,
	opts CrossProjectTaskLineOpts,
) string {
	maxProjLen := opts.MaxProjectLen
	if maxProjLen <= 0 {
		maxProjLen = 12
	}

	projectName := ""
	if proj != nil {
		projectName = proj.Name
	}
	badgeStyle := projectColorStyle(proj)
	if opts.BadgeColor != nil {
		badgeStyle = lipgloss.NewStyle().Foreground(opts.BadgeColor)
	}
	projectBadge := badgeStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(projectName, maxProjLen)))
	if opts.Layout != nil && opts.Layout.HasProject {
		projectBadge = PadBadge(projectBadge, opts.Layout.ProjectWidth)
	}
	preBadges := opts.Prefix + projectBadge + " "

	// Build indicator groups
	structureGroup := renderProgressFraction(task)

	recurStr := ""
	if opts.ShowRecurrence && task.IsRecurring() {
		recurStr = "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		recurStr = MutedStyle.Render(recurStr)
	}
	lifecycleGroup := JoinGroupParts(recurStr)

	tagsStr := ""
	if opts.ShowTags {
		tagsStr = renderTruncatedTags(task.Tags, 2, 8)
	}
	scoreBadge := ""
	if opts.ShowScore && runtime != nil && proj != nil {
		if profile := runtime.Automation().GetProjectScoringProfile(proj.ID); profile != nil {
			scoreBadge = RenderScoreBadge(task, profile)
		}
	}
	staleBadge := renderStaleIndicator(runtime.Settings(), task)
	descStr := ""
	if opts.ShowDescription {
		descStr = renderDescriptionIndicator(task)
	}
	issueStr := renderGitHubIssueIndicator(task)
	contentGroup := JoinGroupParts(descStr, issueStr, tagsStr, scoreBadge, staleBadge)

	postTitle := JoinIndicatorGroups(structureGroup, lifecycleGroup, contentGroup)
	if postTitle != "" {
		postTitle = " " + postTitle
	}

	return TaskLine{
		Task:            task,
		Project:         proj,
		IsSelected:      isCursor,
		MaxTitle:        maxTitleLen,
		Queued:          opts.Queued,
		DailyPlanned:    opts.DailyPlanned,
		Active:          TaskInFocusQueue(runtime.Planning(), task.ID),
		Pinned:          task.Pinned,
		PreBadges:       preBadges,
		PostTitle:       postTitle,
		StatusBadge:     opts.StatusBadge,
		DueStr:          opts.DueStr,
		ViewWidth:       width,
		HideLowPriority: opts.HideLowPriority,
		Flash:           opts.Flash,
		SearchQuery:     opts.SearchQuery,
		RowIndex:        opts.RowIndex,
		Layout:          opts.Layout,
	}.Render()
}

// DimLine strips all ANSI styling and re-renders as dim text.
// Call after full line assembly, before selection styling.
// The caller is responsible for placing a "◌" indicator in the cursor prefix
// position so that DimLine does not alter the line width.
func DimLine(line string) string {
	stripped := ansi.Strip(line)
	return DimmedTaskStyle.Render(stripped)
}

// DimSelectedLine strips all ANSI styling and re-renders as dim+selected text.
// Used when a dimmed task is under the cursor to keep it visually distinct from
// normal selected rows.
func DimSelectedLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return DimmedSelectedStyle.Width(width).Render(stripped)
}

// SelectedLine strips all ANSI styling and re-renders with the unified
// selection style (colored background, high-contrast foreground, bold).
// Call after full line assembly so the entire row gets consistent styling.
func SelectedLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return SelectedStyle.Width(width).Render(stripped)
}

// SelectedFlashLine is like SelectedLine but uses the flash background.
func SelectedFlashLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return SelectedStyle.Background(ColorFlashBg).Width(width).Render(stripped)
}

// MutationFlashLine renders the line with a status-specific flash background.
func MutationFlashLine(line string, width int, bg color.Color) string {
	stripped := ansi.Strip(line)
	return SelectedStyle.Background(bg).Width(width).Render(stripped)
}

// Render assembles the task line string from the configured fields.
func (tl TaskLine) Render() string {
	cursorPrefix := RenderCursorPrefix(tl.IsSelected, tl.Active, tl.Pinned, tl.Task.Dimmed)
	segments := tl.buildTaskLineSegments()
	segments = tl.applyHeroDemotion(segments)

	line := tl.composeTaskLine(cursorPrefix, tl.queueIndicator(), segments)
	line = tl.appendDueString(line, segments.dueStr)
	if tl.ViewWidth > 0 {
		line = truncateToVisualWidth(line, tl.ViewWidth)
	}
	return tl.applyRowStyles(line)
}

type taskLineSegments struct {
	statusIcon      string
	typeIcon        string
	prioritySegment string
	title           string
	dueStr          string
}

func (tl TaskLine) queueIndicator() string {
	switch {
	case tl.DailyPlanned:
		return ActiveStyle.Render("w") + " "
	case tl.Queued:
		return ActiveStyle.Render("q") + " "
	default:
		return "  "
	}
}

func (tl TaskLine) buildTaskLineSegments() taskLineSegments {
	priorityBadge := renderPriorityBadge(tl.Task.Priority)
	if tl.HideLowPriority && tl.Task.Priority > 2 {
		priorityBadge = ""
	}

	segments := taskLineSegments{
		statusIcon: tl.statusBadge(),
		typeIcon:   tl.typeBadge(),
		title:      tl.renderTitle(),
		dueStr:     tl.DueStr,
	}
	segments.statusIcon = tl.padStatusBadge(segments.statusIcon)
	segments.typeIcon = tl.padTypeBadge(segments.typeIcon)
	segments.prioritySegment = tl.prioritySegment(priorityBadge)
	return segments
}

func (tl TaskLine) statusBadge() string {
	if tl.StatusBadge != "" {
		return tl.StatusBadge
	}
	return GetStateBadge(tl.Task.State())
}

func (tl TaskLine) typeBadge() string {
	return GetTypeBadge(string(tl.Task.Type), tl.Task.Type.Icon())
}

func (tl TaskLine) renderTitle() string {
	maxTitleLen := max(tl.MaxTitle-3, 10)
	title := truncateToVisualWidth(tl.Task.Title, maxTitleLen)
	titleStyle := TaskTitleStyleFor(tl.Task, TaskTitleStyleOptions{
		CursorSelected: tl.IsSelected,
	})
	if tl.SearchQuery != "" {
		return HighlightSubstring(title, tl.SearchQuery, titleStyle)
	}
	return titleStyle.Render(title)
}

func (tl TaskLine) padStatusBadge(statusIcon string) string {
	if tl.Layout != nil && tl.Layout.HasStatus {
		return PadBadge(statusIcon, tl.Layout.StatusWidth)
	}
	return statusIcon
}

func (tl TaskLine) padTypeBadge(typeIcon string) string {
	if tl.Layout != nil && tl.Layout.HasType {
		return PadBadge(typeIcon, tl.Layout.TypeWidth)
	}
	return typeIcon
}

func (tl TaskLine) prioritySegment(priorityBadge string) string {
	if tl.Layout != nil && tl.Layout.HasPriority {
		return PadBadge(priorityBadge, tl.Layout.PriorityWidth) + " "
	}
	if priorityBadge != "" {
		return priorityBadge + " "
	}
	return ""
}

func (tl TaskLine) applyHeroDemotion(segments taskLineSegments) taskLineSegments {
	hero := pickHeroAttribute(tl.Task, tl.Project)
	if hero == HeroNone {
		return segments
	}

	if !statusIsHero(hero) {
		segments.statusIcon = demoteBadge(segments.statusIcon)
	}
	segments.typeIcon = demoteBadge(segments.typeIcon)
	if !priorityIsHero(hero) {
		segments.prioritySegment = demotedTaskLineSegment(segments.prioritySegment)
	}
	if segments.dueStr != "" && !dueIsHero(hero) {
		segments.dueStr = demoteBadge(segments.dueStr)
	}
	return segments
}

func demotedTaskLineSegment(segment string) string {
	trimmed := strings.TrimRight(segment, " ")
	if trimmed == "" {
		// Segment is pure alignment padding (e.g. PadBadge of an empty badge
		// when the column layout reserved width for sibling rows). Preserve
		// the original width so vertical alignment survives demotion.
		return segment
	}
	return demoteBadge(trimmed) + " "
}

func (tl TaskLine) composeTaskLine(cursorPrefix, queueIndicator string, segments taskLineSegments) string {
	return fmt.Sprintf(
		"%s%s%s%s%s %s %s%s%s",
		cursorPrefix,
		queueIndicator,
		tl.Prefix,
		tl.PreBadges,
		segments.statusIcon,
		segments.typeIcon,
		segments.prioritySegment,
		segments.title,
		tl.postTitleSuffix(),
	)
}

func (tl TaskLine) postTitleSuffix() string {
	return appendParkedRowSnippet(tl.PostTitle, tl.Task, tl.ViewWidth)
}

func (tl TaskLine) appendDueString(line, dueStr string) string {
	if dueStr == "" {
		return line
	}
	if tl.ViewWidth <= 0 {
		return line + " " + dueStr
	}

	leftWidth := max(tl.ViewWidth-lipgloss.Width(dueStr)-1, 1)
	line = truncateToVisualWidth(line, leftWidth)
	padding := strings.Repeat(" ", max(leftWidth-lipgloss.Width(line), 0))
	return line + padding + " " + dueStr
}

func (tl TaskLine) applyRowStyles(line string) string {
	line = tl.applyDimmedStyle(line)
	if tl.IsSelected && !tl.Task.Dimmed {
		return tl.applySelectedStyle(line)
	}
	if tl.Task.Dimmed || tl.IsSelected || tl.Flash {
		return line
	}
	return tl.applyUnselectedStyle(line)
}

func (tl TaskLine) applyDimmedStyle(line string) string {
	switch {
	case tl.Task.Dimmed && tl.IsSelected:
		return DimSelectedLine(line, tl.ViewWidth)
	case tl.Task.Dimmed:
		return DimLine(line)
	default:
		return line
	}
}

func (tl TaskLine) applySelectedStyle(line string) string {
	if tl.ViewWidth > 0 && tl.Flash {
		return SelectedFlashLine(line, tl.ViewWidth)
	}
	if tl.ViewWidth > 0 {
		return SelectedLine(line, tl.ViewWidth)
	}
	return SelectedLine(line, 0)
}

func (tl TaskLine) applyUnselectedStyle(line string) string {
	switch {
	case tl.Task.IsInState(models.StateWaiting):
		return BlockedRowStyle.Width(tl.ViewWidth).Render(line)
	case tl.Task.IsOverdue():
		return OverdueRowStyle.Width(tl.ViewWidth).Render(line)
	default:
		return ZebraLine(line, tl.ViewWidth, tl.RowIndex)
	}
}
