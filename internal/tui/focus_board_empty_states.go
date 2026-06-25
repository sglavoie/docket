package tui

import "fmt"

// renderTodayEmptyStateBody returns the body shown on the Today tab when no
// rows are visible. Replaces the previous passive copy with action-oriented
// variants that match the cockpit's "what should I do next" reading order:
//
//   - completedToday > 0   → "Today cleared" review-completion celebration
//   - no projects, no tasks → first-run capture prompt
//   - openLoops.Inbox > 0  → inbox-triage prompt
//   - otherwise            → plan-from-backlog prompt
//
// The Next Action strip carries the primary CTA above; this body restates the
// same intent in long form so users see the keybinding for the next move and
// the global Peak-End "Today cleared" banner (set by maybeFireTodayCleared)
// keeps firing independently.
func (m *FocusBoardModel) renderTodayEmptyStateBody() string {
	if completed := m.completedTodayCount(); completed > 0 {
		suffix := "task"
		if completed != 1 {
			suffix = "tasks"
		}
		return SuccessStyle.Render(fmt.Sprintf("  Today cleared — %d %s done.", completed, suffix)) +
			"\n\n" + HelpStyle.Render("  "+cockpitActionsHint(actionPlanMore(), actionQuickCapture(), actionCloseView()))
	}

	switch {
	case !m.hasAnyProjectsForRecommendation() && !m.openLoops.HasTasks:
		return renderEmptyState(
			"  Capture your first task to get started.",
			"  a add · p new project · ? help",
		)
	case m.openLoops.Inbox > 0:
		n := m.openLoops.Inbox
		suffix := "items"
		if n == 1 {
			suffix = "item"
		}
		return renderEmptyState(
			fmt.Sprintf("  %d %s waiting in your inbox.", n, suffix),
			"  "+cockpitActionsHint(actionTriageInbox(), actionQuickCapture()),
		)
	default:
		return renderEmptyState(
			"  Plan today from your backlog.",
			"  "+cockpitActionsHint(actionPlanToday(), actionQuickCapture(), actionShowHelp()),
		)
	}
}
