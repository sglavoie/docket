package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

var ansiStripper = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI color/style escape codes from s.
func stripANSI(s string) string {
	return ansiStripper.ReplaceAllString(s, "")
}

func TestPipelineHubModel_NoProjects(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
	m.SetSize(120, 30)

	plain := stripANSI(m.View().Content)
	if plain == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(plain, "Pipeline Overview") {
		t.Errorf("expected output to contain 'Pipeline Overview', got:\n%s", plain)
	}
	// All 5 stage names should appear.
	for _, stage := range []string{"CAPTURE", "ORGANIZE", "PLAN", "WORK", "EXECUTE", "REVIEW"} {
		if !strings.Contains(plain, stage) {
			t.Errorf("expected output to contain %q, got:\n%s", stage, plain)
		}
	}
}

func TestPipelineHubModel_StatusBarLabel(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
	if got := m.StatusBarLabel(); got != "Pipeline" {
		t.Errorf("StatusBarLabel() = %q, want %q", got, "Pipeline")
	}
}

func TestPipelineHubModel_Navigation(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
	m.SetSize(120, 30)

	// Initial state: col 0, row 0
	if m.activeCol != 0 || m.activeRow != 0 {
		t.Errorf("initial position: col=%d row=%d, want 0,0", m.activeCol, m.activeRow)
	}

	// Move right
	simulateKey(m, "l")
	if m.activeCol != 1 {
		t.Errorf("after l: col=%d, want 1", m.activeCol)
	}

	// Move right to last column
	simulateKey(m, "l")
	simulateKey(m, "l")
	simulateKey(m, "l")
	simulateKey(m, "l")
	if m.activeCol != 5 {
		t.Errorf("after 5×l: col=%d, want 5", m.activeCol)
	}

	// Wrap around right → back to 0
	simulateKey(m, "l")
	if m.activeCol != 0 {
		t.Errorf("after wrap right: col=%d, want 0", m.activeCol)
	}

	// Move down then up
	simulateKey(m, "j")
	if m.activeRow != 1 {
		t.Errorf("after j: row=%d, want 1", m.activeRow)
	}
	simulateKey(m, "k")
	if m.activeRow != 0 {
		t.Errorf("after k: row=%d, want 0", m.activeRow)
	}

	// k at top should stay at 0
	simulateKey(m, "k")
	if m.activeRow != 0 {
		t.Errorf("k at top: row=%d, want 0", m.activeRow)
	}
}

func TestPipelineHubModel_ViewDoesNotPanic(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}

	// Test at various sizes including edge cases
	for _, size := range [][2]int{{0, 0}, {10, 5}, {80, 24}, {200, 50}} {
		m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
		m.SetSize(size[0], size[1])
		_ = m.View()
	}
}

func TestPipelineHubModel_CountsAppearInView(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	queueIDs := map[string]bool{"id1": true, "id2": true, "id3": true}
	dailyPlanIDs := map[string]bool{"id4": true}
	m := NewPipelineHubModel(cfg, nil, queueIDs, dailyPlanIDs, nil)
	m.SetSize(160, 40)

	plain := stripANSI(m.View().Content)
	// Queue count = 3, daily plan count = 1
	if !strings.Contains(plain, "(3)") {
		t.Errorf("expected queue count (3) in output:\n%s", plain)
	}
	if !strings.Contains(plain, "(1)") {
		t.Errorf("expected daily plan count (1) in output:\n%s", plain)
	}
}

func TestCountInbox_RecursesIntoChildren(t *testing.T) {
	root := &models.Task{Section: models.SectionInbox, Status: models.StatusTodo}
	child := &models.Task{Section: models.SectionInbox, Status: models.StatusTodo}
	grandchild := &models.Task{Section: models.SectionInbox, Status: models.StatusTodo}
	completedChild := &models.Task{Section: models.SectionInbox, Status: models.StatusDone}
	nonInboxChild := &models.Task{Section: models.SectionNext, Status: models.StatusTodo}

	child.Children = []*models.Task{grandchild}
	root.Children = []*models.Task{child, completedChild, nonInboxChild}

	got := countInbox(root)
	if got != 3 {
		t.Errorf("countInbox = %d, want 3 (root + child + grandchild; completed and non-inbox excluded)", got)
	}
}

// simulateKey sends a key press to the model (in-place update).
func simulateKey(m *PipelineHubModel, key string) {
	updated, _ := m.Update(syntheticKeyStr(key))
	*m = *updated.(*PipelineHubModel)
}
