package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestCustomContextEditorProjectRowsDistinguishCursorFromSelection(t *testing.T) {
	cfg := newTestConfig()
	alpha := testutil.NewTestProject("Alpha")
	alpha.Context = "work"
	beta := testutil.NewTestProject("Beta")
	beta.Context = "home"
	setTestProjects(cfg, []*models.Project{alpha, beta})
	cfg.Machine().SetCustomContext("Work", []string{alpha.ID})

	m := NewCustomContextEditorModel(testRuntimeAt(cfg.DataDir(), cfg), "Work")
	m.projectListMode = true
	m.cursor = 1
	m.SetSize(100, 30)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "  [x] Alpha") {
		t.Fatalf("expected selected non-cursor project to show checked state without cursor marker:\n%s", plain)
	}
	if !strings.Contains(plain, IconCursorRight+" [ ] Beta") {
		t.Fatalf("expected cursor project to show cursor marker separately from checked state:\n%s", plain)
	}
	if strings.Count(plain, IconCursorRight+" ") != 1 {
		t.Fatalf("expected exactly one visible project cursor marker, got %d:\n%s", strings.Count(plain, IconCursorRight+" "), plain)
	}
}
