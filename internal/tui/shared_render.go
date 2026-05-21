package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// IndicatorGroupSep is the dim separator rendered between indicator groups.
var IndicatorGroupSep = MutedStyle.Render(" │ ")

// JoinIndicatorGroups joins non-empty group strings with a dim "·" separator.
// Empty groups are skipped so no orphan separators appear.
func JoinIndicatorGroups(groups ...string) string {
	var nonEmpty []string
	for _, g := range groups {
		if g != "" {
			nonEmpty = append(nonEmpty, g)
		}
	}
	return strings.Join(nonEmpty, IndicatorGroupSep)
}

// JoinGroupParts joins non-empty parts within one indicator group with a space.
func JoinGroupParts(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, " ")
}

// FormatCyclePosition returns a " [i/n]" suffix if projectID appears in the
// configured cycle list, or "" otherwise.
func FormatCyclePosition(cfg *config.Config, projectID string) string {
	cycleProjects := cfg.GetCycleProjects()
	for i, p := range cycleProjects {
		if p.ID == projectID {
			return fmt.Sprintf(" [%d/%d]", i+1, len(cycleProjects))
		}
	}
	return ""
}

// EmptyStateFiltered is the standard message shown when active filters produce no results.
const EmptyStateFiltered = "No tasks match filters. Press 'F' then 'x' to clear."

// EmptyStateNoTasks is the standard message shown when a view has no tasks at all.
const EmptyStateNoTasks = "No tasks yet. Press 'i' to add one."

// EmptyStateClosed is the standard message shown when the closed/done tab has no tasks.
const EmptyStateClosed = "No closed tasks."

// EmptyStateNoMatch is the standard message shown when a tab filter produces no results.
const EmptyStateNoMatch = "No tasks match this tab."

// EmptyStateNoMessages is shown when a message log has no entries.
const EmptyStateNoMessages = "No messages yet."

// EmptyStateSearchPrompt is shown in a search view when no query has been entered.
const EmptyStateSearchPrompt = "Type to search across all tasks…"

// EmptyStateCrossProject is shown when a cross-project search finds no tasks.
const EmptyStateCrossProject = "No tasks found across projects."

// EmptyStateNoAlerts is shown in the alert center when no alerts are active.
const EmptyStateNoAlerts = "No alerts — everything looks good!"

// SectionHeaderOpts configures the unified section header renderer.
type SectionHeaderOpts struct {
	Name        string
	Count       int
	Collapsed   bool
	AccentColor lipgloss.Style // applied to the name portion
	Width       int            // if > 0, trailing ─ fill is added
	Prefix      string         // inserted between indicator and name (e.g. "@ ")
}

// renderSectionHeader renders a standardized collapsible section header:
//
//	▾ [prefix]Name (count) ─────
//
// AccentColor is applied to the name; indicator and count use MutedStyle.
// When Width > 0 a trailing horizontal rule fills the remaining line width.
// Callers are responsible for applying selection/zebra highlights on the
// returned string.
func renderSectionHeader(opts SectionHeaderOpts) string {
	indicator := IconExpanded
	if opts.Collapsed {
		indicator = IconCollapsed
	}
	indPart := MutedStyle.Render(indicator)
	namePart := opts.AccentColor.Bold(true).Render(opts.Prefix + opts.Name)
	countPart := MutedStyle.Render(fmt.Sprintf("(%d)", opts.Count))

	line := "  " + indPart + " " + namePart + " " + countPart

	if opts.Width > 0 {
		visible := lipgloss.Width(line)
		if opts.Width > visible+2 {
			line += " " + MutedStyle.Render(strings.Repeat("─", opts.Width-visible-1))
		}
	}
	return line
}

// renderTruncatedTags renders a compact inline tag string with per-tag truncation
// and overflow count. Shows at most maxTags tags, each truncated to maxTagLen chars.
// Returns an empty string if there are no tags.
func renderTruncatedTags(tags []string, maxTags, maxTagLen int) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	for i, tag := range tags {
		if i >= maxTags {
			break
		}
		display := truncateToVisualWidth(tag, maxTagLen)
		parts = append(parts, "#"+display)
	}
	result := TagInlineStyle.Render(strings.Join(parts, " "))
	if len(tags) > maxTags {
		result += MutedStyle.Render(fmt.Sprintf(" +%d", len(tags)-maxTags))
	}
	return result
}

func renderDescriptionIndicator(task *models.Task) string {
	if len(task.Description) == 0 {
		return ""
	}
	return MutedStyle.Render(IconDescription())
}

// renderCommentIndicator renders a compact comment count indicator.
// Shows unresolved/total when some are resolved, otherwise just total.
func renderCommentIndicator(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	unresolved := task.UnresolvedCommentCount()
	if unresolved < len(task.Comments) {
		return MutedStyle.Render(fmt.Sprintf("%s %d/%d", IconComment(), unresolved, len(task.Comments)))
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconComment(), len(task.Comments)))
}

// renderDecisionIndicator renders a compact decision count indicator.
// Shows unresolved/total when some are resolved, otherwise just total.
func renderDecisionIndicator(task *models.Task) string {
	if len(task.Decisions) == 0 {
		return ""
	}
	unresolved := task.UnresolvedDecisionCount()
	if unresolved < len(task.Decisions) {
		return MutedStyle.Render(fmt.Sprintf("%s %d/%d", IconDecision(), unresolved, len(task.Decisions)))
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconDecision(), len(task.Decisions)))
}

// renderAttachmentIndicator renders a compact attachment count indicator.
func renderAttachmentIndicator(task *models.Task) string {
	if !task.HasAttachments() {
		return ""
	}
	return MutedStyle.Render(fmt.Sprintf("%s %d", IconAttachment(), task.AttachmentCount()))
}

// renderChecklistIndicator renders a compact checklist progress indicator.
func renderChecklistIndicator(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	completed, total := checklistCounts(task)
	return MutedStyle.Render(fmt.Sprintf("%s%d/%d", IconChecklist(), completed, total))
}

// renderChecklistIndicatorCompact renders the smallest checklist progress form
// (no icon, plain "X/Y") suitable for very narrow containers like Kanban cards.
func renderChecklistIndicatorCompact(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	completed, total := checklistCounts(task)
	return MutedStyle.Render(fmt.Sprintf("%d/%d", completed, total))
}

// renderChecklistBar renders a checklist progress bar of the form "[▓▓▓░░] 3/5".
// When the available width is too small to render a meaningful bar, falls back
// to renderChecklistIndicator. Returns "" when the checklist is empty.
func renderChecklistBar(task *models.Task, width int) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	const minBarWidth = 12
	if width < minBarWidth {
		return renderChecklistIndicator(task)
	}
	completed, total := checklistCounts(task)
	percent := float64(completed) / float64(total) * 100
	bar := "[" + RenderProgressBar(percent, 5) + "]"
	fraction := fmt.Sprintf("%d/%d", completed, total)
	if completed == total {
		return bar + " " + SuccessStyle.Render(fraction)
	}
	return bar + " " + MutedStyle.Render(fraction)
}

// checklistCounts returns (completed, total) for a task's checklist.
func checklistCounts(task *models.Task) (int, int) {
	completed := 0
	for _, item := range task.Checklist {
		if item.Done {
			completed++
		}
	}
	return completed, len(task.Checklist)
}

// renderProgressFraction renders an inline progress bar + fraction for parent tasks.
// Returns an empty string if the task has no children.
func renderProgressFraction(task *models.Task) string {
	if !task.HasProgressChildren() {
		return ""
	}
	completed := 0
	for _, child := range task.Children {
		if child.IsComplete() {
			completed++
		}
	}
	total := len(task.Children)
	percent := float64(completed) / float64(total) * 100
	bar := "[" + RenderProgressBar(percent, 5) + "]"
	fraction := fmt.Sprintf("%d/%d", completed, total)
	if completed == total {
		return bar + " " + SuccessStyle.Render(fraction)
	}
	return bar + " " + MutedStyle.Render(fraction)
}

// renderDependencyIndicators renders compact dependency arrow indicators.
func renderDependencyIndicators(task *models.Task, project *models.Project) string {
	var parts []string
	if len(task.DependsOn) > 0 {
		parts = append(parts, DependencyStyle.Render(fmt.Sprintf("→%d", len(task.DependsOn))))
	}
	if blockedBy := project.GetBlockedByTasks(task); len(blockedBy) > 0 {
		parts = append(parts, DependencyStyle.Render(fmt.Sprintf("←%d", len(blockedBy))))
	}
	return strings.Join(parts, " ")
}

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
	PostTitle       string        // after title (e.g., tags, score badge, custom section)
	DueStr          string        // right-aligned due date string (pre-styled by caller)
	ViewWidth       int           // total view width for due date right-alignment
	StatusBadge     string        // override computed status badge (pre-styled by caller)
	StageBadge      string        // pre-styled stage badge (from GetStageBadge)
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
	BadgeColor      color.Color   // when set, overrides MutedStyle on the [project-name] badge
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
	cfg *config.Config,
	isCursor bool,
	width, maxTitleLen int,
	opts CrossProjectTaskLineOpts,
) string {
	maxProjLen := opts.MaxProjectLen
	if maxProjLen <= 0 {
		maxProjLen = 12
	}

	badgeStyle := MutedStyle
	if opts.BadgeColor != nil {
		badgeStyle = lipgloss.NewStyle().Foreground(opts.BadgeColor)
	}
	projectBadge := badgeStyle.Render(fmt.Sprintf("[%s]", truncateToVisualWidth(proj.Name, maxProjLen)))
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
	if opts.ShowScore && cfg != nil {
		if profile := cfg.GetProjectScoringProfile(proj.ID); profile != nil {
			scoreBadge = RenderScoreBadge(task, profile)
		}
	}
	staleBadge := ""
	if cfg != nil {
		if threshold := cfg.GetStaleTaskDays(); threshold > 0 {
			effectiveUpdate := task.UpdatedAt
			if effectiveUpdate.IsZero() {
				effectiveUpdate = task.CreatedAt
			}
			if !effectiveUpdate.IsZero() {
				days := int(time.Since(effectiveUpdate).Hours() / 24)
				if days >= threshold {
					staleBadge = MutedStyle.Render("stale")
				}
			}
		}
	}
	descStr := ""
	if opts.ShowDescription && len(task.Description) > 0 {
		descStr = MutedStyle.Render(IconDescription())
	}
	contentGroup := JoinGroupParts(descStr, tagsStr, scoreBadge, staleBadge)

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
		Active:          task.Active,
		Pinned:          task.Pinned,
		PreBadges:       preBadges,
		PostTitle:       postTitle,
		StatusBadge:     opts.StatusBadge,
		StageBadge:      GetStageBadge(task.Stage),
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
	// Standard 5-char prefix: [dim(1)][active(1)][pin(1)] + [T/Q(2)]
	cursorPrefix := RenderCursorPrefix(tl.IsSelected, tl.Active, tl.Pinned, tl.Task.Dimmed)

	queueIndicator := "  "
	if tl.DailyPlanned {
		queueIndicator = ActiveStyle.Render("w") + " "
	} else if tl.Queued {
		queueIndicator = ActiveStyle.Render("q") + " "
	}

	statusIcon := tl.StatusBadge
	if statusIcon == "" {
		statusIcon = GetStatusBadge(string(tl.Task.Status), tl.Task.Status.Icon())
	}
	typeIcon := GetTypeBadge(string(tl.Task.Type), tl.Task.Type.Icon())
	priorityBadge := renderPriorityBadge(tl.Task.Priority)
	if tl.HideLowPriority && tl.Task.Priority > 2 {
		priorityBadge = ""
	}

	// Cursor prefix (3 chars) + queue indicator (2 chars) reduce title space.
	maxTitleLen := max(tl.MaxTitle-3, 10)
	title := truncateToVisualWidth(tl.Task.Title, maxTitleLen)
	if tl.SearchQuery != "" {
		title = HighlightSubstring(title, tl.SearchQuery, GetTitlePriorityStyle(tl.Task.Priority))
	}
	if tl.IsSelected && !tl.Task.Dimmed {
		title = SelectedTaskTitleStyle.Render(title)
	} else {
		title = GetTitlePriorityStyle(tl.Task.Priority).Render(title)
	}

	// Fixed-width badge columns when Layout is set, variable otherwise.
	if tl.Layout != nil && tl.Layout.HasStatus {
		statusIcon = PadBadge(statusIcon, tl.Layout.StatusWidth)
	}
	if tl.Layout != nil && tl.Layout.HasType {
		typeIcon = PadBadge(typeIcon, tl.Layout.TypeWidth)
	}

	var prioritySegment string
	if tl.Layout != nil && tl.Layout.HasPriority {
		prioritySegment = PadBadge(priorityBadge, tl.Layout.PriorityWidth) + " "
	} else if priorityBadge != "" {
		prioritySegment = priorityBadge + " "
	}

	// Build stage segment. When Layout reserves a stage column, pad the
	// rendered badge (or an empty placeholder) so the title column stays
	// aligned across rows.
	stageBadge := tl.StageBadge
	var stageSegment string
	if tl.Layout != nil && tl.Layout.HasStage {
		if stageBadge != "" {
			stageSegment = PadBadge(stageBadge, tl.Layout.StageWidth) + " "
		} else {
			stageSegment = EmptyBadge(tl.Layout.StageWidth) + " "
		}
	} else if stageBadge != "" {
		stageSegment = stageBadge + " "
	}

	// Von Restorff: when a hero attribute applies, demote the non-hero badges
	// so the single most urgent attribute reads at full saturation.
	hero := pickHeroAttribute(tl.Task, tl.Project)
	if hero != HeroNone && !tl.Task.Dimmed {
		if !statusIsHero(hero) {
			statusIcon = demoteBadge(statusIcon)
		}
		typeIcon = demoteBadge(typeIcon)
		if !priorityIsHero(hero) && prioritySegment != "" {
			if trimmed := strings.TrimRight(prioritySegment, " "); trimmed != "" {
				prioritySegment = demoteBadge(trimmed) + " "
			}
		}
		if stageSegment != "" && stageBadge != "" {
			stageSegment = demoteBadge(strings.TrimRight(stageSegment, " ")) + " "
		}
	}

	dueStr := tl.DueStr
	if hero != HeroNone && !tl.Task.Dimmed && dueStr != "" && !dueIsHero(hero) {
		dueStr = demoteBadge(dueStr)
	}
	line := fmt.Sprintf("%s%s%s%s%s %s %s%s%s%s", cursorPrefix, queueIndicator, tl.Prefix, tl.PreBadges, statusIcon, typeIcon, prioritySegment, stageSegment, title, tl.PostTitle)
	if dueStr != "" {
		if tl.ViewWidth > 0 {
			leftWidth := max(tl.ViewWidth-lipgloss.Width(dueStr)-1, 1)
			line = truncateToVisualWidth(line, leftWidth)
			line += strings.Repeat(" ", max(leftWidth-lipgloss.Width(line), 0)) + " " + dueStr
		} else {
			line += " " + dueStr
		}
	}

	// Truncate to prevent terminal wrapping.
	if tl.ViewWidth > 0 {
		line = truncateToVisualWidth(line, tl.ViewWidth)
	}

	if tl.Task.Dimmed && !tl.IsSelected {
		line = DimLine(line)
	} else if tl.Task.Dimmed && tl.IsSelected {
		line = DimSelectedLine(line, tl.ViewWidth)
	}

	if tl.IsSelected && (tl.Task == nil || !tl.Task.Dimmed) {
		if tl.ViewWidth > 0 {
			if tl.Flash {
				line = SelectedFlashLine(line, tl.ViewWidth)
			} else {
				line = SelectedLine(line, tl.ViewWidth)
			}
		} else {
			line = SelectedLine(line, 0)
		}
	} else if !tl.Task.Dimmed && !tl.IsSelected && !tl.Flash {
		if tl.Task.Status == models.StatusBlocked {
			line = BlockedRowStyle.Width(tl.ViewWidth).Render(line)
		} else if tl.Task.IsOverdue() {
			line = OverdueRowStyle.Width(tl.ViewWidth).Render(line)
		} else {
			line = ZebraLine(line, tl.ViewWidth, tl.RowIndex)
		}
	}

	return line
}
