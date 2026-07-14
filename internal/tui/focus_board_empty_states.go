package tui

import (
	"fmt"
	"strings"
)

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
		return m.withOnboardingGuide(
			SuccessStyle.Render(fmt.Sprintf("  Today cleared — %d %s done.", completed, suffix)) +
				"\n\n" + HelpStyle.Render("  "+cockpitActionsHint(actionPlanMore(), actionQuickCapture(), actionCloseView())),
		)
	}

	switch {
	case !m.hasAnyProjectsForRecommendation() && !m.openLoops.HasTasks:
		return m.withOnboardingGuide(renderEmptyState(
			"  Capture your first task to get started.",
			"  a add · p new project · ? help",
		))
	case m.openLoops.Inbox > 0:
		n := m.openLoops.Inbox
		suffix := "items"
		if n == 1 {
			suffix = "item"
		}
		return m.withOnboardingGuide(renderEmptyState(
			fmt.Sprintf("  %d %s waiting in your inbox.", n, suffix),
			"  "+cockpitActionsHint(actionTriageInbox(), actionQuickCapture()),
		))
	default:
		return m.withOnboardingGuide(renderEmptyState(
			"  Plan today from your backlog.",
			"  "+cockpitActionsHint(actionPlanToday(), actionQuickCapture(), actionShowHelp()),
		))
	}
}

func (m *FocusBoardModel) withOnboardingGuide(body string) string {
	if m == nil || m.runtime == nil {
		return body
	}
	progress := m.runtime.Settings().GetOnboardingProgress()
	status := onboardingStatus(progress)
	if status == "" {
		return body
	}
	if m.height <= 18 {
		next := "create(n)"
		switch {
		case !progress.ProjectCreated:
		case !progress.TaskCaptured:
			next = "capture(a)"
		case !progress.InboxTriaged:
			next = "triage(ctrl+i)"
		case !progress.TaskPlanned:
			next = "plan(;q)"
		case !progress.TaskCompleted:
			next = "complete(x)"
		}
		status = fmt.Sprintf("Setup %d/5 · Next: %s", progress.CompletedCount(), next)
	}
	width := max(m.width-4, 20)
	lines := wrapText(status, width)
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return InfoStatusStyle.Render(strings.Join(lines, "\n")) + "\n\n" + body
}
