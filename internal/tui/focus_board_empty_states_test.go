package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// Session 6 (Today Empty And Completion States) replaces the cockpit's passive
// empty-state copy with action-oriented variants that match the strip's CTA.
// The branches live in renderTodayEmptyStateBody and route on:
//
//   1. completedToday > 0   → Today-cleared celebration ("Today cleared — N done")
//   2. no projects + no tasks → first-run capture ("Capture your first task")
//   3. openLoops.Inbox > 0  → inbox triage ("N items waiting in your inbox.")
//   4. otherwise            → backlog plan ("Plan today from your backlog.")
//
// These tests lock the routing contract and the hint clusters so future
// sessions can layer further polish without losing the keybinding hints.

// emptyStateBody returns the rendered Today body for a focus-board model with
// no rows. It exercises the same render path the cockpit uses (View()) so the
// assertions cover the integrated empty-state branch.
func emptyStateBody(t *testing.T, m *FocusBoardModel) string {
	t.Helper()
	m.SetSize(120, 30)
	return ansi.Strip(m.View().Content)
}

// TestTodayEmptyStateFirstRunCopy covers the no-projects, no-tasks first-run
// state. The body must restate the strip's "Create your first project" CTA in
// the cockpit's reading register and surface the `a add` / `p new project`
// keybinding hints so the user knows how to act.
func TestTodayEmptyStateFirstRunCopy(t *testing.T) {
	m := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabToday)
	plain := emptyStateBody(t, m)

	for _, want := range []string{
		"Create your first project",               // strip
		"Capture your first task to get started.", // body
		"a add",
		": commands",
		"? help",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("first-run empty state missing %q in:\n%s", want, plain)
		}
	}

	// Routing guard: the inbox/backlog/cleared branches must not leak when no
	// tasks or projects exist.
	for _, forbidden := range []string{
		"waiting in your inbox",
		"Plan today from your backlog.",
		"Today cleared",
	} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("first-run empty state should not contain %q in:\n%s", forbidden, plain)
		}
	}
}

// TestTodayEmptyStateInboxOnlyCopyPlural covers the inbox-triage state when
// the openLoops counter reports multiple inbox items. The body must restate
// the strip's "Triage inbox (N items)" CTA with the inbox keybinding.
func TestTodayEmptyStateInboxOnlyCopyPlural(t *testing.T) {
	openLoops := OpenLoops{Inbox: 3, HasTasks: true}
	m := cockpitModel(nil, nil, nil, nil, openLoops, boardTabToday)
	plain := emptyStateBody(t, m)

	for _, want := range []string{
		"Triage inbox",                   // strip
		"3 items waiting in your inbox.", // body (plural)
		": open Manage Inbox",
		"a add",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("inbox-only empty state missing %q in:\n%s", want, plain)
		}
	}

	for _, forbidden := range []string{
		"Capture your first task",
		"Plan today from your backlog.",
		"Today cleared",
	} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("inbox-only empty state should not contain %q in:\n%s", forbidden, plain)
		}
	}
}

// TestTodayEmptyStateInboxOnlyCopySingular pins the singular suffix so the
// body reads naturally when exactly one inbox item is waiting.
func TestTodayEmptyStateInboxOnlyCopySingular(t *testing.T) {
	openLoops := OpenLoops{Inbox: 1, HasTasks: true}
	m := cockpitModel(nil, nil, nil, nil, openLoops, boardTabToday)
	plain := emptyStateBody(t, m)

	if !strings.Contains(plain, "1 item waiting in your inbox.") {
		t.Fatalf("expected singular inbox copy `1 item waiting in your inbox.` in:\n%s", plain)
	}
	if strings.Contains(plain, "1 items") {
		t.Fatalf("singular inbox copy should not pluralize `items` in:\n%s", plain)
	}
}

// TestTodayEmptyStateBacklogReadyCopy covers the case where tasks exist but
// nothing is planned or in the inbox. The body steers the user toward the
// Backlog tab so they can choose work for Today.
func TestTodayEmptyStateBacklogReadyCopy(t *testing.T) {
	project := cockpitProject(t, "Backlog")
	backlogTask := cockpitTask(t, project, "Refactor parser", models.StatusTodo, models.SectionNext)
	row := cockpitBoardRow(backlogTask, project)

	openLoops := OpenLoops{HasTasks: true}
	m := cockpitModel(nil, []boardRow{row}, nil, nil, openLoops, boardTabToday)
	plain := emptyStateBody(t, m)

	for _, want := range []string{
		"Plan today from your backlog.",
		"tab switch",
		"a add",
		"? help",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("backlog-ready empty state missing %q in:\n%s", want, plain)
		}
	}

	for _, forbidden := range []string{
		"Capture your first task",
		"waiting in your inbox",
		"Today cleared",
	} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("backlog-ready empty state should not contain %q in:\n%s", forbidden, plain)
		}
	}
}

// TestTodayEmptyStateInboxBeatsBacklog confirms the routing precedence when
// both inbox items and backlog tasks exist. Inbox triage is the higher-leverage
// action (it converts captured items into plannable work), so the body must
// surface the inbox CTA before the generic backlog one.
func TestTodayEmptyStateInboxBeatsBacklog(t *testing.T) {
	project := cockpitProject(t, "Backlog")
	backlogTask := cockpitTask(t, project, "Refactor parser", models.StatusTodo, models.SectionNext)
	row := cockpitBoardRow(backlogTask, project)

	openLoops := OpenLoops{Inbox: 2, HasTasks: true}
	m := cockpitModel(nil, []boardRow{row}, nil, nil, openLoops, boardTabToday)
	plain := emptyStateBody(t, m)

	if !strings.Contains(plain, "2 items waiting in your inbox.") {
		t.Fatalf("expected inbox CTA to win over backlog when both exist:\n%s", plain)
	}
	if strings.Contains(plain, "Plan today from your backlog.") {
		t.Fatalf("backlog CTA should be suppressed while inbox has items:\n%s", plain)
	}
}

// TestTodayEmptyStateStripPrecedesBody locks the reading order: the Next
// Action strip must render above the action-oriented empty-state body for
// every variant, so the user scans strip → body → hints top-to-bottom.
func TestTodayEmptyStateStripPrecedesBody(t *testing.T) {
	cases := []struct {
		name      string
		openLoops OpenLoops
		stripCue  string
		bodyCue   string
	}{
		{
			name:      "first-run",
			openLoops: OpenLoops{},
			stripCue:  "Create your first project",
			bodyCue:   "Capture your first task to get started.",
		},
		{
			name:      "inbox-only",
			openLoops: OpenLoops{Inbox: 2, HasTasks: true},
			stripCue:  "Triage inbox",
			bodyCue:   "2 items waiting in your inbox.",
		},
		{
			name:      "backlog-ready",
			openLoops: OpenLoops{HasTasks: true},
			stripCue:  "All caught up for today",
			bodyCue:   "Plan today from your backlog.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := cockpitModel(nil, nil, nil, nil, tc.openLoops, boardTabToday)
			plain := emptyStateBody(t, m)
			stripIdx := strings.Index(plain, tc.stripCue)
			if stripIdx < 0 {
				t.Fatalf("strip cue %q missing in:\n%s", tc.stripCue, plain)
			}
			bodyIdx := strings.Index(plain, tc.bodyCue)
			if bodyIdx < 0 {
				t.Fatalf("body cue %q missing in:\n%s", tc.bodyCue, plain)
			}
			if stripIdx >= bodyIdx {
				t.Fatalf("expected strip (idx %d) above body (idx %d) for %s", stripIdx, bodyIdx, tc.name)
			}
		})
	}
}

// TestTodayEmptyStateBodyDirectInvocation exercises the helper directly so
// future refactors that split View() into smaller pieces can keep the routing
// branches under a single test surface without re-running the whole cockpit.
func TestTodayEmptyStateBodyDirectInvocation(t *testing.T) {
	t.Run("first-run", func(t *testing.T) {
		m := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabToday)
		got := ansi.Strip(m.renderTodayEmptyStateBody())
		if !strings.Contains(got, "Capture your first task to get started.") {
			t.Fatalf("first-run body missing canonical copy: %q", got)
		}
	})

	t.Run("inbox-only", func(t *testing.T) {
		m := cockpitModel(nil, nil, nil, nil, OpenLoops{Inbox: 4, HasTasks: true}, boardTabToday)
		got := ansi.Strip(m.renderTodayEmptyStateBody())
		if !strings.Contains(got, "4 items waiting in your inbox.") {
			t.Fatalf("inbox body missing canonical copy: %q", got)
		}
	})

	t.Run("backlog-ready", func(t *testing.T) {
		project := cockpitProject(t, "Backlog")
		backlogTask := cockpitTask(t, project, "Sweep stale deps", models.StatusTodo, models.SectionNext)
		row := cockpitBoardRow(backlogTask, project)
		m := cockpitModel(nil, []boardRow{row}, nil, nil, OpenLoops{HasTasks: true}, boardTabToday)
		got := ansi.Strip(m.renderTodayEmptyStateBody())
		if !strings.Contains(got, "Plan today from your backlog.") {
			t.Fatalf("backlog body missing canonical copy: %q", got)
		}
	})
}

// TestTodayEmptyStatePreservesFooterHints confirms the cockpit footer's
// shortcut hints still render alongside the new empty-state body. Session 5
// pinned the same invariant for populated Today; session 6 extends the guard
// to the empty-state surface so muscle memory keeps working when the user
// hits Today with nothing planned.
func TestTodayEmptyStatePreservesFooterHints(t *testing.T) {
	m := cockpitModel(nil, nil, nil, nil, OpenLoops{Inbox: 2, HasTasks: true}, boardTabToday)
	plain := emptyStateBody(t, m)

	// The footer for the empty Today branch reads "a add · tab switch ·
	// Esc close · ? help" (see focus_board_render.go). The new body must
	// not displace these baseline hints.
	for _, want := range []string{"a add", "tab"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("footer hint %q missing alongside empty state body:\n%s", want, plain)
		}
	}
}
