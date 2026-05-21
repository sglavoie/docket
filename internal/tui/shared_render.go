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
	staleBadge := renderStaleIndicator(cfg, task)
	descStr := ""
	if opts.ShowDescription {
		descStr = renderDescriptionIndicator(task)
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
