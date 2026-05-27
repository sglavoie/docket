package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/workflow"
)

// TestExpandedViewConfirmDialogReceivesYesKey verifies that when a delete
// confirmation dialog is opened from the expanded (narrow-screen) preview,
// the "y" key reaches the confirm dialog handler and deletes the rule.
// Regression: previously the expanded-view key handler ran before the confirm
// dispatch, swallowing y/n and leaving the user stuck.
func TestExpandedViewConfirmDialogReceivesYesKey(t *testing.T) {
	cfg := config.NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true, Source: "user"},
	}
	mgr := config.NewManager(filepath.Join(t.TempDir(), "tt.json"))

	m := NewWorkflowRulesModel(cfg, mgr)
	m.SetSize(60, 24) // narrow screen so expanded view is allowed
	m.expandedView = true

	if len(m.filteredRules) != 1 {
		t.Fatalf("expected 1 filtered rule, got %d", len(m.filteredRules))
	}

	// Open the delete confirm dialog via the expanded-view "d" key.
	cmd := m.handleExpandedViewKey("d")
	if cmd != nil {
		t.Fatalf("expected nil cmd from d in expanded view, got %v", cmd)
	}
	if !m.confirm.IsActive() {
		t.Fatal("expected confirm dialog to be active after pressing d")
	}

	// Now press "y": this must dispatch to the confirm dialog, not be
	// swallowed by handleExpandedViewKey.
	_, _ = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	if m.confirm.IsActive() {
		t.Fatal("confirm dialog should be dismissed after pressing y")
	}
	if len(cfg.WorkflowRules) != 0 {
		t.Fatalf("expected rule to be deleted, %d rules remain", len(cfg.WorkflowRules))
	}
	if m.expandedView {
		t.Error("expandedView should be reset to false after confirmed delete")
	}
}

// TestExpandedViewConfirmDialogReceivesCancelKey verifies the cancel path:
// pressing "n" from expanded view dismisses the dialog without deleting.
func TestExpandedViewConfirmDialogReceivesCancelKey(t *testing.T) {
	cfg := config.NewConfig()
	cfg.WorkflowRules = []workflow.WorkflowRule{
		{ID: "r1", Name: "Rule 1", Enabled: true, Source: "user"},
	}
	mgr := config.NewManager(filepath.Join(t.TempDir(), "tt.json"))

	m := NewWorkflowRulesModel(cfg, mgr)
	m.SetSize(60, 24)
	m.expandedView = true

	_ = m.handleExpandedViewKey("d")
	if !m.confirm.IsActive() {
		t.Fatal("expected confirm dialog active")
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})

	if m.confirm.IsActive() {
		t.Error("confirm dialog should be dismissed after pressing n")
	}
	if len(cfg.WorkflowRules) != 1 {
		t.Errorf("rule should NOT be deleted on cancel, got %d rules", len(cfg.WorkflowRules))
	}
}
