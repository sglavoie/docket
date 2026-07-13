package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskMetadataAlreadyShown lists row-context flags that allow the metadata
// strip to omit fields the user can already see in the current row or view.
// Only flag a field as already shown when the row presents it in exactly the
// same form the strip would; partial duplication (e.g. an inline date that
// omits the time) should leave the flag false.
type TaskMetadataAlreadyShown struct {
	Priority bool
	Project  bool
	Section  bool
	Due      bool
	Score    bool
	Stale    bool
	Updated  bool
}

// TaskMetadataStripOpts configures renderTaskMetadataStrip.
type TaskMetadataStripOpts struct {
	Task         *models.Task
	Project      *models.Project
	Config       *config.Config
	Width        int // max visual width; <= 0 means unbounded
	AlreadyShown TaskMetadataAlreadyShown

	// ColumnConfig, when non-nil, gates optional strip parts through the
	// caller's column-visibility resolution. A nil value preserves the
	// pre-Session-2 behavior where every available indicator participates.
	ColumnConfig *ColumnConfig
}

// metadataGroup labels a strip part for ordering and visual separation.
// Same-group parts are joined with " "; different groups are joined with
// IndicatorGroupSep.
type metadataGroup int

const (
	metadataGroupProjectSection metadataGroup = iota
	metadataGroupLifecycle
	metadataGroupScore
	metadataGroupProgress
	metadataGroupDependencies
	metadataGroupTags
	metadataGroupContent
	metadataGroupFooter
)

// Drop-priority constants. Lower values are kept longer when the strip
// overflows the configured width; higher values are dropped first.
const (
	dropCritical = 0 // never drop unless we must (priority, due)
	dropHigh     = 2
	dropMedium   = 4
	dropLow      = 6
	dropLowest   = 8
)

// metadataPart is one rendered fragment of the strip.
type metadataPart struct {
	group        metadataGroup
	text         string // pre-rendered (may contain ANSI styling); must be non-empty
	dropPriority int    // higher = dropped earlier under overflow
}

// renderTaskMetadataStrip renders the one-line metadata strip describing the
// selected task. Returns an empty string when there is no task, when no
// metadata is available, or when no part survives overflow trimming.
func renderTaskMetadataStrip(opts TaskMetadataStripOpts) string {
	if opts.Task == nil {
		return ""
	}
	parts := buildMetadataParts(opts)
	if len(parts) == 0 {
		return ""
	}
	if opts.Width > 0 {
		parts = applyMetadataStripOverflow(parts, opts.Width)
		if len(parts) == 0 {
			return ""
		}
	}
	rendered := joinMetadataParts(parts)
	if opts.Width > 0 && lipgloss.Width(rendered) > opts.Width {
		rendered = truncateToVisualWidth(rendered, opts.Width)
	}
	return rendered
}

// buildMetadataParts collects strip fragments in display order, skipping any
// field flagged as already shown by the row context. Optional column groups
// are also gated through opts.ColumnConfig when non-nil so the strip honors
// the same visibility resolution used by the column picker.
func buildMetadataParts(opts TaskMetadataStripOpts) []metadataPart {
	parts := make([]metadataPart, 0, 12)

	parts = appendProjectSectionParts(parts, opts)
	parts = appendLifecycleParts(parts, opts)
	parts = appendScorePart(parts, opts)
	parts = appendProgressParts(parts, opts)
	parts = appendDependencyPart(parts, opts)
	parts = appendTagsPart(parts, opts)
	parts = appendContentParts(parts, opts)
	parts = appendFooterParts(parts, opts)

	return parts
}

// columnVisible reports whether the named optional column should participate
// in the metadata strip. A nil ColumnConfig is treated as fully permissive so
// existing callers that have not opted into column gating keep their behavior.
func columnVisible(cc *ColumnConfig, col string) bool {
	if cc == nil {
		return true
	}
	return cc.IsVisible(col)
}

func priorityVisibleInStrip(cc *ColumnConfig, priority int) bool {
	if priority < 1 || priority > 5 {
		return false
	}
	if cc == nil {
		return true
	}
	return cc.IsVisible(ColPriority) && cc.ShouldShowPriority(priority)
}

func formatSnoozeDate(snoozedUntil time.Time) string {
	if snoozedUntil.Year() != time.Now().Year() {
		return snoozedUntil.Format("Jan 2 '06")
	}
	return snoozedUntil.Format("Jan 2")
}

func appendProjectSectionParts(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	if !opts.AlreadyShown.Project && opts.Project != nil && opts.Project.Name != "" {
		text := MutedStyle.Render(fmt.Sprintf("[%s]", opts.Project.Name))
		parts = append(parts, metadataPart{
			group:        metadataGroupProjectSection,
			text:         text,
			dropPriority: dropHigh,
		})
	}
	if bucket := taskSectionBucket(opts.Task); !opts.AlreadyShown.Section && bucket != "" {
		text := MutedStyle.Render(bucket)
		parts = append(parts, metadataPart{
			group:        metadataGroupProjectSection,
			text:         text,
			dropPriority: dropHigh,
		})
	}
	return parts
}

func appendLifecycleParts(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	task := opts.Task
	if !opts.AlreadyShown.Priority && priorityVisibleInStrip(opts.ColumnConfig, task.Priority) {
		parts = append(parts, metadataPart{
			group:        metadataGroupLifecycle,
			text:         renderPriorityBadge(task.Priority),
			dropPriority: dropCritical,
		})
	}
	if !opts.AlreadyShown.Due && task.DueDate != nil && columnVisible(opts.ColumnConfig, ColDueDate) {
		var dueStr string
		if opts.Config != nil {
			dueStr = FormatRelativeDueTask(task, true, opts.Config.GetTimeFormat())
		} else {
			dueStr = FormatRelativeDue(task.DueDate, true)
		}
		if dueStr != "" {
			parts = append(parts, metadataPart{
				group:        metadataGroupLifecycle,
				text:         dueStr,
				dropPriority: dropCritical,
			})
		}
	}
	if task.SnoozedUntil != nil && columnVisible(opts.ColumnConfig, ColSnooze) {
		parts = append(parts, metadataPart{
			group:        metadataGroupLifecycle,
			text:         MutedStyle.Render("zzz " + formatSnoozeDate(*task.SnoozedUntil)),
			dropPriority: dropMedium,
		})
	}
	if task.IsRecurring() && columnVisible(opts.ColumnConfig, ColRecurrence) {
		recurStr := "↻"
		if task.Recurrence.Streak > 0 {
			recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
		}
		parts = append(parts, metadataPart{
			group:        metadataGroupLifecycle,
			text:         MutedStyle.Render(recurStr),
			dropPriority: dropMedium,
		})
	}
	return parts
}

func appendScorePart(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	if opts.AlreadyShown.Score || opts.Config == nil || opts.Project == nil ||
		!columnVisible(opts.ColumnConfig, ColScore) {
		return parts
	}
	profile := opts.Config.GetProjectScoringProfile(opts.Project.ID)
	if profile == nil {
		return parts
	}
	badge := RenderScoreBadge(opts.Task, profile)
	if badge == "" {
		return parts
	}
	return append(parts, metadataPart{
		group:        metadataGroupScore,
		text:         badge,
		dropPriority: dropMedium,
	})
}

func appendProgressParts(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	if columnVisible(opts.ColumnConfig, ColProgress) {
		if progress := renderProgressFraction(opts.Task); progress != "" {
			parts = append(parts, metadataPart{
				group:        metadataGroupProgress,
				text:         progress,
				dropPriority: dropMedium,
			})
		}
	}
	if columnVisible(opts.ColumnConfig, ColChecklist) {
		if checklist := renderChecklistIndicator(opts.Task); checklist != "" {
			parts = append(parts, metadataPart{
				group:        metadataGroupProgress,
				text:         checklist,
				dropPriority: dropMedium,
			})
		}
	}
	return parts
}

func appendDependencyPart(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	if opts.Project == nil || !columnVisible(opts.ColumnConfig, ColDependencies) {
		return parts
	}
	deps := renderDependencyIndicators(opts.Task, opts.Project)
	if deps == "" {
		return parts
	}
	return append(parts, metadataPart{
		group:        metadataGroupDependencies,
		text:         deps,
		dropPriority: dropLow,
	})
}

func appendTagsPart(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	if !columnVisible(opts.ColumnConfig, ColTags) {
		return parts
	}
	tags := renderTruncatedTags(opts.Task.Tags, 3, 12)
	if tags == "" {
		return parts
	}
	return append(parts, metadataPart{
		group:        metadataGroupTags,
		text:         tags,
		dropPriority: dropLow,
	})
}

func appendContentParts(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	task := opts.Task
	contentTexts := []struct {
		text   string
		column string
	}{
		{renderDescriptionIndicator(task), ColDescription},
		{renderCommentIndicator(task), ColComments},
		{renderDecisionIndicator(task), ColDecisions},
		{renderAttachmentIndicator(task), ColAttachments},
		{renderGitHubIssueIndicator(task), ""}, // no column toggle
	}
	for _, item := range contentTexts {
		if item.text == "" {
			continue
		}
		if item.column != "" && !columnVisible(opts.ColumnConfig, item.column) {
			continue
		}
		parts = append(parts, metadataPart{
			group:        metadataGroupContent,
			text:         item.text,
			dropPriority: dropLowest - 1,
		})
	}
	return parts
}

func appendFooterParts(parts []metadataPart, opts TaskMetadataStripOpts) []metadataPart {
	if opts.Task.EstimatedDays > 0 {
		text := MutedStyle.Render(fmt.Sprintf("~%gd", opts.Task.EstimatedDays))
		parts = append(parts, metadataPart{
			group:        metadataGroupFooter,
			text:         text,
			dropPriority: dropLowest,
		})
	}
	if !opts.AlreadyShown.Stale {
		if stale := renderStaleIndicator(opts.Config, opts.Task); stale != "" {
			parts = append(parts, metadataPart{
				group:        metadataGroupFooter,
				text:         stale,
				dropPriority: dropLowest,
			})
		}
	}
	if !opts.AlreadyShown.Updated {
		updatedAt := opts.Task.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = opts.Task.CreatedAt
		}
		if !updatedAt.IsZero() {
			parts = append(parts, metadataPart{
				group:        metadataGroupFooter,
				text:         MutedStyle.Render(formatRelativeTime(updatedAt)),
				dropPriority: dropLowest + 1,
			})
		}
	}
	return parts
}

// joinMetadataParts produces the final strip string. Parts within the same
// group are space-joined; group transitions get IndicatorGroupSep.
func joinMetadataParts(parts []metadataPart) string {
	if len(parts) == 0 {
		return ""
	}
	var sb strings.Builder
	prevGroup := parts[0].group
	sb.WriteString(parts[0].text)
	for _, p := range parts[1:] {
		if p.group == prevGroup {
			sb.WriteString(" ")
		} else {
			sb.WriteString(IndicatorGroupSep)
			prevGroup = p.group
		}
		sb.WriteString(p.text)
	}
	return sb.String()
}

// applyMetadataStripOverflow drops parts until the rendered string fits in
// width. Parts with higher dropPriority are dropped first; ties break by
// dropping later (right-most) parts first so left-anchored context survives.
// The final remaining part may still exceed width; the caller is responsible
// for truncating the rendered string.
func applyMetadataStripOverflow(parts []metadataPart, width int) []metadataPart {
	for len(parts) > 0 {
		if lipgloss.Width(joinMetadataParts(parts)) <= width {
			return parts
		}
		dropIdx := pickMetadataDropIndex(parts)
		if dropIdx < 0 {
			return parts
		}
		parts = append(parts[:dropIdx], parts[dropIdx+1:]...)
	}
	return parts
}

// pickMetadataDropIndex returns the index of the part to drop next: highest
// dropPriority wins; ties go to the right-most part.
func pickMetadataDropIndex(parts []metadataPart) int {
	if len(parts) == 0 {
		return -1
	}
	best := 0
	for i := 1; i < len(parts); i++ {
		if parts[i].dropPriority > parts[best].dropPriority {
			best = i
			continue
		}
		if parts[i].dropPriority == parts[best].dropPriority && i > best {
			best = i
		}
	}
	return best
}
