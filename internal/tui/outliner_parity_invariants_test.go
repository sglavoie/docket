package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// parityKeyCase describes one cross-project parity binding and the status
// invariant that must hold when the cursor sits on an unscoped non-task row
// (section header or separator). Each invoker resolves and executes the binding through the same
// keymap builder the production view uses, so the test fails if any future
// edit drops the cursor-project / cursor-task guard or stops setting the
// shared status constant.
type parityKeyCase struct {
	name       string
	invoke     func(t *testing.T, m *OutlinerModel) tea.Msg
	wantStatus string
}

// crossParityKeyCases aggregates the keybindings whose cross-project no-task
// path was wired through sessions 2-4. Session 2 covers navigation, session
// 3 covers capture/creation, session 4 covers mutation/project operations.
func crossParityKeyCases() []parityKeyCase {
	return []parityKeyCase{
		// Session 2 — navigation parity (primary view entries).
		{
			name:       "B (ProjectBlockers)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "B") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "W (InboxProcessor)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "W") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "} (CreateNote)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "}") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "{ (OpenNotes)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "{") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "ctrl+l (EditLastModified)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "ctrl+l") },
			wantStatus: statusNoProjectUnderCursor,
		},
		// Session 2 — goto-prefix entries.
		{
			name:       "gh (ViewTaskActivity)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeGotoChild(t, m, "h") },
			wantStatus: statusNoTaskSelected,
		},
		{
			name:       "gl (ProjectRecentActivity)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeGotoChild(t, m, "l") },
			wantStatus: statusNoProjectUnderCursor,
		},
		// Session 3 — capture/creation parity.
		{
			name:       "i (QuickCapture)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "i") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "I (BulkCapture)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "I") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name: "tn (handleTaskPrefix new task)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg {
				_, cmd := m.handleTaskPrefix("n", nil)
				if cmd == nil {
					return nil
				}
				return cmd()
			},
			wantStatus: statusNoTaskSelected,
		},
		{
			name: "tN (handleTaskPrefix new task with template)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg {
				_, cmd := m.handleTaskPrefix("N", nil)
				if cmd == nil {
					return nil
				}
				return cmd()
			},
			wantStatus: statusNoTaskSelected,
		},
		// Session 4 — mutation/project operation parity (utility entries).
		{
			name:       "ctrl+a (ArchiveProject)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+a") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "ctrl+v (PasteClipboard)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+v") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "ctrl+s (SectionManager)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+s") },
			wantStatus: statusNoProjectUnderCursor,
		},
		{
			name:       "ctrl+e (ExportProject)",
			invoke:     func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+e") },
			wantStatus: statusNoProjectUnderCursor,
		},
	}
}

// TestCrossOutlinerParity_SectionHeaderInvariant aggregates every cross-project
// parity key and asserts that placing the cursor on an unscoped section header
// emits no message and sets the shared status constant. The
// table-driven shape lets a regression in any single binding surface as one
// targeted subtest failure rather than a diffuse keymap symptom.
func TestCrossOutlinerParity_SectionHeaderInvariant(t *testing.T) {
	for _, tc := range crossParityKeyCases() {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _, _, _ := crossOutlinerWithConfig(t)
			m.cursor = 0 // header row

			if msg := tc.invoke(t, m); msg != nil {
				t.Fatalf("expected nil message on header row, got %T", msg)
			}
			if m.statusMessage != tc.wantStatus {
				t.Fatalf("status = %q, want %q", m.statusMessage, tc.wantStatus)
			}
		})
	}
}

// TestCrossOutlinerParity_SeparatorInvariant mirrors the header invariant
// for separator rows. Separators appear only in cross-project mode, between
// adjacent projects, so the no-message + status-constant contract guards
// against drift where a future edit silently swallows the keypress without
// telling the user why.
func TestCrossOutlinerParity_SeparatorInvariant(t *testing.T) {
	for _, tc := range crossParityKeyCases() {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _, _, _ := crossOutlinerWithConfig(t)
			m.cursor = 2 // separator row

			if msg := tc.invoke(t, m); msg != nil {
				t.Fatalf("expected nil message on separator row, got %T", msg)
			}
			if m.statusMessage != tc.wantStatus {
				t.Fatalf("status = %q, want %q", m.statusMessage, tc.wantStatus)
			}
		})
	}
}

// TestCrossOutlinerParity_EmptyRowsInvariant covers the edge case where the
// rows slice is empty (newly opened cross-project view before the first
// loadedProjects rebuild, or an empty filter result). Every parity binding
// must return nil and set the shared status constant rather than panic on an
// out-of-range cursor.
func TestCrossOutlinerParity_EmptyRowsInvariant(t *testing.T) {
	for _, tc := range crossParityKeyCases() {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _, _, _ := crossOutlinerWithConfig(t)
			m.rows = nil
			m.cursor = 0

			if msg := tc.invoke(t, m); msg != nil {
				t.Fatalf("expected nil message on empty rows, got %T", msg)
			}
			if m.statusMessage != tc.wantStatus {
				t.Fatalf("status = %q, want %q", m.statusMessage, tc.wantStatus)
			}
		})
	}
}

// TestCrossOutlinerParity_CursorRoutingMatrix aggregates the project-routing
// happy paths from sessions 2-3 into one matrix. Each row drives a parity
// key against a two-project scaffold, places the cursor on each project's
// task, and asserts the emitted message carries that project. A regression
// where cross-mode dispatch falls back to currentProject or the first
// loaded project surfaces here as one targeted subtest failure per key.
func TestCrossOutlinerParity_CursorRoutingMatrix(t *testing.T) {
	type routingCase struct {
		name      string
		invoke    func(t *testing.T, m *OutlinerModel) tea.Msg
		extract   func(tea.Msg) *models.Project
		extractOK func(tea.Msg) bool
	}
	cases := []routingCase{
		{
			name:    "B (ProjectBlockers)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "B") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ShowBlockersMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ShowBlockersMsg)
				return ok
			},
		},
		{
			name:    "W (InboxProcessor)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "W") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ShowInboxProcessorMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ShowInboxProcessorMsg)
				return ok
			},
		},
		{
			name:    "} (CreateNote)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "}") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ShowNoteCreatorMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ShowNoteCreatorMsg)
				return ok
			},
		},
		{
			name:    "ctrl+l (EditLastModified)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "ctrl+l") },
			extract: func(msg tea.Msg) *models.Project { return msg.(EditTaskMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(EditTaskMsg)
				return ok
			},
		},
		{
			name:    "gl (ProjectRecentActivity)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeGotoChild(t, m, "l") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ShowProjectRecentActivityMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ShowProjectRecentActivityMsg)
				return ok
			},
		},
		{
			name:    "i (QuickCapture)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "i") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ShowQuickCaptureMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ShowQuickCaptureMsg)
				return ok
			},
		},
		{
			name:    "I (BulkCapture)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "I") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ShowBulkCaptureMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ShowBulkCaptureMsg)
				return ok
			},
		},
		{
			name:    "ctrl+a (ArchiveProject)",
			invoke:  func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+a") },
			extract: func(msg tea.Msg) *models.Project { return msg.(ProjectArchiveRequestMsg).Project },
			extractOK: func(msg tea.Msg) bool {
				_, ok := msg.(ProjectArchiveRequestMsg)
				return ok
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, projA, projB, _, _ := crossOutlinerWithConfig(t)

			m.cursor = 1 // task in projA
			msgA := tc.invoke(t, m)
			if !tc.extractOK(msgA) {
				t.Fatalf("cursor=projA: unexpected message type %T", msgA)
			}
			if got := tc.extract(msgA); got != projA {
				t.Fatalf("cursor=projA: project = %v, want %v", got, projA)
			}

			m.cursor = 4 // task in projB
			msgB := tc.invoke(t, m)
			if !tc.extractOK(msgB) {
				t.Fatalf("cursor=projB: unexpected message type %T", msgB)
			}
			if got := tc.extract(msgB); got != projB {
				t.Fatalf("cursor=projB: project = %v, want %v", got, projB)
			}
		})
	}
}
