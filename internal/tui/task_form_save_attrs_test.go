package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// ── buildDraftTaskAttrs ─────────────────────────────────────────────────────

func TestTaskForm_Save_BuildAttrs_BugType(t *testing.T) {
	m := newBugTaskForm()
	m.draft.bugAttrs.Severity = models.BugSeverityMajor
	m.draft.bugAttrs.Reproducible = models.BugReproducibleYes
	bug, idea, task, story, epic := m.buildDraftTaskAttrs()
	if bug == nil {
		t.Fatal("expected bugAttrs to be non-nil")
	}
	if bug.Severity != models.BugSeverityMajor {
		t.Errorf("expected Severity=major, got %v", bug.Severity)
	}
	if idea != nil || task != nil || story != nil || epic != nil {
		t.Error("other attrs should be nil for Bug type")
	}
}

func TestTaskForm_Save_BuildAttrs_EmptyBug_ReturnsNil(t *testing.T) {
	m := newBugTaskForm()
	bug, _, _, _, _ := m.buildDraftTaskAttrs()
	if bug != nil {
		t.Error("expected bugAttrs=nil when bug attrs are empty")
	}
}
