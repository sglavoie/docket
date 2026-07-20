package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

var ansiStripper = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI color/style escape codes from s.
func stripANSI(s string) string {
	return ansiStripper.ReplaceAllString(s, "")
}

func TestPipelineHubModel_NoProjects(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
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
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
	if got := m.StatusBarLabel(); got != "Pipeline" {
		t.Errorf("StatusBarLabel() = %q, want %q", got, "Pipeline")
	}
}

func TestPipelineHubModel_HandledKeysIncludesArrowNavigation(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
	keys := m.HandledKeys()

	for _, key := range []string{"j", "down", "k", "up"} {
		if !slices.Contains(keys, key) {
			t.Errorf("HandledKeys missing %q", key)
		}
	}
}

func TestPipelineHubModel_Navigation(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
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
	cfg := newTestConfig()

	// Test at various sizes including edge cases
	for _, size := range [][2]int{{0, 0}, {10, 5}, {80, 24}, {200, 50}} {
		m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
		m.SetSize(size[0], size[1])
		_ = m.View()
	}
}

func TestPipelineHubModel_CountsAppearInView(t *testing.T) {
	// The Next ("Plan") count derives from StateNext membership, not the passed
	// queue-ID map, so seed three StateNext tasks on disk for loadPipelineCounts
	// to load. Today's count remains the config-owned daily-plan list length.
	dataDir := t.TempDir()
	cfg := newTestConfigAt(dataDir)
	project := testutil.NewTestProject("pipeline-counts")
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	for _, title := range []string{"Next A", "Next B", "Next C"} {
		task := mustNewTestTask(project, title)
		task.ApplyWorkflowState(models.StateNext)
		project.Tasks = append(project.Tasks, task)
	}
	project.BuildTaskTree()

	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("mkdir project path: %v", err)
	}
	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	defer func() { storage.DefaultRepository = oldRepo }()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	setTestProjects(cfg, []*models.Project{project})

	dailyPlanIDs := map[string]bool{"id4": true}
	m := NewPipelineHubModel(testRuntimeAt(dataDir, cfg), nil, dailyPlanIDs, nil)
	m.SetSize(160, 40)

	plain := stripANSI(m.View().Content)
	// Next (StateNext) count = 3, daily plan count = 1
	if !strings.Contains(plain, "(3)") {
		t.Errorf("expected StateNext count (3) in output:\n%s", plain)
	}
	if !strings.Contains(plain, "(1)") {
		t.Errorf("expected daily plan count (1) in output:\n%s", plain)
	}
}

func TestCountInbox_CountsProcessableTasksAtEveryDepth(t *testing.T) {
	future := time.Now().AddDate(0, 0, 1)
	parentID := "root"

	tests := []struct {
		name string
		task *models.Task
		want int
	}{
		{
			name: "root inbox todo",
			task: &models.Task{WorkflowState: models.StateInbox},
			want: 1,
		},
		{
			name: "completed root inbox",
			task: &models.Task{WorkflowState: models.StateDone},
			want: 0,
		},
		{
			name: "snoozed root inbox",
			task: &models.Task{WorkflowState: models.StateInbox, SnoozedUntil: &future},
			want: 0,
		},
		{
			name: "child inbox task",
			task: &models.Task{ParentID: &parentID, WorkflowState: models.StateInbox},
			want: 1,
		},
		{
			name: "root with inbox child",
			task: &models.Task{
				ID:            "root",
				WorkflowState: models.StateInbox,
				Children:      []*models.Task{{ParentID: &parentID, WorkflowState: models.StateInbox}},
			},
			want: 2,
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
