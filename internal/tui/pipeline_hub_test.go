package tui

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

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
	// All six pipeline stage names should appear in the new cockpit vocabulary.
	for _, stage := range []string{"CAPTURE", "TRIAGE", "PLAN", "WORK", "REVIEW", "SUPPORT"} {
		if !strings.Contains(plain, stage) {
			t.Errorf("expected output to contain %q, got:\n%s", stage, plain)
		}
	}
	// Legacy vocabulary must not leak back into the hub header.
	for _, legacy := range []string{"ORGANIZE", "EXECUTE"} {
		if strings.Contains(plain, legacy) {
			t.Errorf("expected legacy stage %q to be removed from hub, got:\n%s", legacy, plain)
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

func TestPipelineHubModel_HandledKeysIncludesArrowNavigation(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
	keys := m.HandledKeys()

	for _, key := range []string{"j", "down", "k", "up"} {
		if !slices.Contains(keys, key) {
			t.Errorf("HandledKeys missing %q", key)
		}
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

func TestCountInbox_CountsProcessableRootTasksOnly(t *testing.T) {
	future := time.Now().AddDate(0, 0, 1)
	parentID := "root"

	tests := []struct {
		name string
		task *models.Task
		want int
	}{
		{
			name: "root inbox todo",
			task: &models.Task{Section: models.SectionInbox, Status: models.StatusTodo},
			want: 1,
		},
		{
			name: "completed root inbox",
			task: &models.Task{Section: models.SectionInbox, Status: models.StatusDone},
			want: 0,
		},
		{
			name: "snoozed root inbox",
			task: &models.Task{Section: models.SectionInbox, Status: models.StatusTodo, SnoozedUntil: &future},
			want: 0,
		},
		{
			name: "child inbox task",
			task: &models.Task{ParentID: &parentID, Section: models.SectionInbox, Status: models.StatusTodo},
			want: 0,
		},
		{
			name: "root with inbox child",
			task: &models.Task{
				ID:       "root",
				Section:  models.SectionInbox,
				Status:   models.StatusTodo,
				Children: []*models.Task{{ParentID: &parentID, Section: models.SectionInbox, Status: models.StatusTodo}},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countInbox(tt.task); got != tt.want {
				t.Errorf("countInbox = %d, want %d", got, tt.want)
			}
		})
	}
}

// simulateKey sends a key press to the model (in-place update).
func simulateKey(m *PipelineHubModel, key string) {
	updated, _ := m.Update(syntheticKeyStr(key))
	*m = *updated.(*PipelineHubModel)
}
