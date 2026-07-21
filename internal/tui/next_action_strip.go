package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// NextActionStripMinWidth is the smallest render width that still produces a
// strip. Below this width the strip is suppressed so the caller can reclaim
// the row for content; the shortest possible message ("All caught up") plus a
// single key hint still fits at this width.
const NextActionStripMinWidth = 30

// nextActionStripGap is the minimum number of spaces preserved between the
// left segment (kind + title + project + reasons) and the right segment
// (key hints). Below this gap the right segment is dropped, then reasons,
// then project, then the title is truncated.
const nextActionStripGap = 2

// NextActionStripOpts configures rendering of the Next Action strip — a
// single-line "what should I do next" component drawn above Today's content.
type NextActionStripOpts struct {
	// Action is the deterministic recommendation produced by the engine. Pass
	// the zero value when no recommendation is available; the strip then
	// falls back to OpenLoops to decide what to render.
	Action RecommendedAction

	// Width is the available horizontal budget. Below NextActionStripMinWidth
	// the strip renders empty.
	Width int

	// HasProjects reports whether any project exists in the cockpit. Used to
	// distinguish first-run AllDone ("create your first project") from
	// steady-state AllDone ("all caught up").
	HasProjects bool

	// OpenLoops carries cached cross-project counters from AppModel. Acts as a
	// supplemental signal when the engine's input is incomplete (e.g., inbox
	// tasks tracked by the counter but not loaded into Projects).
	OpenLoops OpenLoops
}

// nextActionStripCopy is the resolved kind plus a small variant tag used for
// copy/key-hint selection. The variant tag lets a single Kind branch into
// distinct surfaces (e.g., AllDone splits into "first-run" vs "caught-up").
type nextActionStripCopy struct {
	kind    RecommendationKind
	variant string
}

const (
	stripVariantDefault   = ""
	stripVariantFirstRun  = "first-run"
	stripVariantCaughtUp  = "caught-up"
	stripVariantInboxOnly = "inbox-only"
)

// renderNextActionStrip composes the Next Action strip for the Today cockpit.
// Always renders one terminal row of styled text. Returns empty when Width is
// below NextActionStripMinWidth so callers can reclaim the row.
func renderNextActionStrip(opts NextActionStripOpts) string {
	if opts.Width < NextActionStripMinWidth {
		return ""
	}

	resolved := resolveNextActionStripCopy(opts)
	if resolved.kind == RecommendationKindNone {
		return ""
	}

	glyph := stripGlyphFor(resolved.kind, resolved.variant)
	title := stripTitleFor(resolved, opts)
	project := stripProjectFor(resolved, opts.Action)
	reasons := stripReasonsFor(resolved, opts.Action.Reasons)
	hints := stripHintsFor(resolved, opts)

	return assembleNextActionStripRow(opts.Width, glyph, title, project, reasons, hints, resolved)
}

// resolveNextActionStripCopy turns the engine output plus supplemental
// OpenLoops counters into the kind + variant the renderer should display.
//
// The engine cannot tell "nothing exists yet" (first-run) from "everything is
// done" — both surface as AllDone (see session-3 handoff). The renderer
// applies cockpit-level context to choose between the two and to pick up
// inbox-only signals that the engine missed because the projects slice was
// incomplete.
func resolveNextActionStripCopy(opts NextActionStripOpts) nextActionStripCopy {
	switch opts.Action.Kind {
	case RecommendationKindWorkOnTask,
		RecommendationKindReviewBlocked,
		RecommendationKindTriageInbox,
		RecommendationKindPlanToday:
		return nextActionStripCopy{kind: opts.Action.Kind, variant: stripVariantDefault}
	case RecommendationKindAllDone, RecommendationKindNone:
		switch {
		case opts.OpenLoops.Inbox > 0:
			return nextActionStripCopy{kind: RecommendationKindTriageInbox, variant: stripVariantInboxOnly}
		case !opts.HasProjects && !opts.OpenLoops.HasTasks:
			return nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantFirstRun}
		default:
			return nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantCaughtUp}
		}
	}
	return nextActionStripCopy{kind: RecommendationKindNone}
}

func stripGlyphFor(kind RecommendationKind, variant string) string {
	switch kind {
	case RecommendationKindWorkOnTask, RecommendationKindPlanToday, RecommendationKindTriageInbox:
		return "▸"
	case RecommendationKindReviewBlocked:
		return "⊘"
	case RecommendationKindAllDone:
		if variant == stripVariantFirstRun {
			return "✦"
		}
		return "✓"
	default:
		return "·"
	}
}

func stripTitleFor(c nextActionStripCopy, opts NextActionStripOpts) string {
	switch c.kind {
	case RecommendationKindWorkOnTask:
		return "Next: " + safeTaskTitle(opts.Action.Task, "an unscheduled task")
	case RecommendationKindReviewBlocked:
		return "Review blocked: " + safeTaskTitle(opts.Action.Task, "a parked task")
	case RecommendationKindTriageInbox:
		if n := opts.OpenLoops.Inbox; n > 0 {
			suffix := "items"
			if n == 1 {
				suffix = "item"
			}
			return fmt.Sprintf("Triage inbox (%d %s)", n, suffix)
		}
		return "Triage inbox"
	case RecommendationKindPlanToday:
		return "Plan today's work"
	case RecommendationKindAllDone:
		switch c.variant {
		case stripVariantFirstRun:
			return "Create your first project"
		case stripVariantCaughtUp:
			return "All caught up for today"
		default:
			return "All done"
		}
	default:
		return ""
	}
}

func safeTaskTitle(task *models.Task, fallback string) string {
	if task == nil {
		return fallback
	}
	if title := strings.TrimSpace(task.Title); title != "" {
		return title
	}
	return fallback
}

// stripProjectFor returns the project badge (e.g., "[Project Name]") for
// kinds that pin a task to a single project. Empty when the kind does not
// carry a task.
func stripProjectFor(c nextActionStripCopy, action RecommendedAction) string {
	if action.Project == nil {
		return ""
	}
	switch c.kind {
	case RecommendationKindWorkOnTask, RecommendationKindReviewBlocked:
		name := truncateToVisualWidth(action.Project.Name, 18)
		if name == "" {
			return ""
		}
		return "[" + name + "]"
	default:
		return ""
	}
}

// stripReasonsFor returns the comma-separated reason badges (already styled).
// Reasons are emitted in the order the engine produces them; duplicates are
// preserved for emphasis, matching the engine contract documented in
// session-3 expansion notes. Empty when no reasons apply.
func stripReasonsFor(c nextActionStripCopy, reasons []RecommendationReason) string {
	if len(reasons) == 0 {
		return ""
	}
	// AllDone / PlanToday do not surface reason badges; the title already
	// tells the user what to do.
	if c.kind == RecommendationKindAllDone || c.kind == RecommendationKindPlanToday {
		return ""
	}
	labels := make([]string, 0, len(reasons))
	for _, r := range reasons {
		if !r.IsValid() {
			continue
		}
		labels = append(labels, styleReasonLabel(r))
	}
	if len(labels) == 0 {
		return ""
	}
	return strings.Join(labels, " · ")
}

func styleReasonLabel(r RecommendationReason) string {
	switch r {
	case ReasonOverdue, ReasonBlocked:
		return OverdueStyle.Render(r.Label())
	case ReasonDueToday, ReasonCarriedOver, ReasonHighPriority:
		return WarningIconStyle.Render(r.Label())
	case ReasonPlannedToday, ReasonRecentlyActive:
		return SuccessStyle.Render(r.Label())
	default:
		return MutedStyle.Render(r.Label())
	}
}

// stripHintsFor returns the right-aligned key hint cluster for the kind. The
// strings inside are styled with HelpStyle so the row reads as a help line
// without overpowering the title.
func stripHintsFor(c nextActionStripCopy, opts NextActionStripOpts) string {
	hints := primaryHintsFor(c, opts)
	if len(hints) == 0 {
		return ""
	}
	return HelpStyle.Render(strings.Join(hints, " · "))
}

// primaryHintsFor returns the strip's right-aligned key hint cluster. It uses
// the cockpit action model so the strip cannot advertise stale shortcuts after
// the Today key grammar changes.
func primaryHintsFor(c nextActionStripCopy, opts NextActionStripOpts) []string {
	switch c.kind {
	case RecommendationKindWorkOnTask:
		return nil // enter is already in the Planner footer
	case RecommendationKindReviewBlocked:
		return []string{"s state"}
	case RecommendationKindTriageInbox:
		return []string{": manage inbox"}
	case RecommendationKindPlanToday:
		return nil // tab is already in the Planner footer
	case RecommendationKindAllDone:
		// a/tab are already in the Planner footer.
		return nil
	default:
		return nil
	}
}

// assembleNextActionStripRow places glyph + title + project + reasons on the
// left and hints on the right, with degrade-on-narrow drop priority:
//  1. Drop hints
//  2. Drop reasons
//  3. Drop project badge
//  4. Truncate title with ellipsis
//
// All segments are pre-styled. The returned row is exactly one terminal line
// and never exceeds width visually.
func assembleNextActionStripRow(width int, glyph, title, project, reasons, hints string, c nextActionStripCopy) string {
	glyphStyled := stripGlyphStyleFor(c).Render(glyph)
	titleStyled := stripTitleStyle().Render(title)
	projectStyled := ""
	if project != "" {
		projectStyled = MutedStyle.Render(project)
	}

	segments := []string{glyphStyled, titleStyled}
	if projectStyled != "" {
		segments = append(segments, projectStyled)
	}
	if reasons != "" {
		segments = append(segments, MutedStyle.Render("·"), reasons)
	}
	left := strings.Join(segments, " ")

	right := hints

	// Iteratively drop optional segments until the row fits.
	for {
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(right)

		if rightW == 0 {
			if leftW <= width {
				return left
			}
			// Truncate the rendered left side as a last resort. Dense next-action
			// data is allowed to clip, so use the intentional audited helper.
			return truncateIntentional(left, width, "…")
		}

		if leftW+nextActionStripGap+rightW <= width {
			gap := width - leftW - rightW
			return left + strings.Repeat(" ", gap) + right
		}

		// Drop hints first.
		if right != "" {
			right = ""
			continue
		}
	}
}

// stripGlyphStyleFor returns the bold glyph style for a resolved strip kind.
// The foreground color mirrors the urgency tier so the glyph reads as a
// pre-attentive cue (red for blocked, yellow for triage, green for caught-up,
// accent for action/plan/first-run) — a visual-polish remap of the existing
// glyphs without changing the recommendation contract.
func stripGlyphStyleFor(c nextActionStripCopy) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(stripGlyphColorFor(c)).Bold(true)
}

func stripGlyphColorFor(c nextActionStripCopy) color.Color {
	switch c.kind {
	case RecommendationKindReviewBlocked:
		return ColorError
	case RecommendationKindTriageInbox:
		return ColorWarning
	case RecommendationKindAllDone:
		if c.variant == stripVariantCaughtUp {
			return ColorSuccess
		}
		return ColorAccent
	default:
		return ColorAccent
	}
}

func stripTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
}
