package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
)

// TestPipelineHubColumnVocabularyMatchesCockpitModel pins the new six-stage
// vocabulary on the model's column slice so future drift away from the
// Capture/Triage/Plan/Work/Review/Support map fails fast.
func TestPipelineHubColumnVocabularyMatchesCockpitModel(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)

	want := []string{"Capture", "Triage", "Plan", "Work", "Review", "Support"}
	if len(m.columns) != len(want) {
		t.Fatalf("expected %d columns, got %d", len(want), len(m.columns))
	}
	for i, name := range want {
		if m.columns[i].name != name {
			t.Errorf("column[%d] = %q, want %q", i, m.columns[i].name, name)
		}
	}
}

// TestPipelineHubSupportColumnExposesCrossCuttingViews verifies the new
// Support column wires Blockers, Notes, Workflow Rules, and Notifications —
// the four cross-cutting tools that were previously absent from the hub.
func TestPipelineHubSupportColumnExposesCrossCuttingViews(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)

	var support pipelineColumn
	for _, col := range m.columns {
		if col.name == "Support" {
			support = col
			break
		}
	}
	if support.name == "" {
		t.Fatalf("expected Support column to exist; got columns %v", columnNames(m.columns))
	}

	wantViews := map[ViewType]bool{
		ViewGlobalBlockers:     false,
		ViewGlobalNotes:        false,
		ViewWorkflowRules:      false,
		ViewNotificationCenter: false,
	}
	for _, entry := range support.entries {
		if _, ok := wantViews[entry.viewType]; ok {
			wantViews[entry.viewType] = true
		}
	}
	for vt, seen := range wantViews {
		if !seen {
			t.Errorf("Support column missing entry for view %v", vt)
		}
	}
}

// TestPipelineHubWorkColumnAbsorbsExecuteEntries verifies that with a current
// project, the Work column carries both Today and the project-specific
// entries that used to live in the dropped Execute column.
func TestPipelineHubWorkColumnAbsorbsExecuteEntries(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("Alpha")
	m := NewPipelineHubModel(testRuntimeAt(cfg.DataDir(), cfg), nil, nil, project)

	var work pipelineColumn
	for _, col := range m.columns {
		if col.name == "Work" {
			work = col
			break
		}
	}
	if work.name == "" {
		t.Fatalf("expected Work column to exist; got columns %v", columnNames(m.columns))
	}

	labels := make([]string, 0, len(work.entries))
	outlinerRoutes := 0
	for _, e := range work.entries {
		labels = append(labels, e.label)
		if e.viewType == ViewOutliner {
			outlinerRoutes++
		}
	}

	if !contains(labels, "Today") {
		t.Errorf("Work column missing 'Today' entry; labels=%v", labels)
	}
	// The current project entry is the sole route to its Outliner — the retired
	// ProjectTree left the Outliner as the only per-project surface, so a
	// separate "Outliner" tile would be a duplicate.
	if !contains(labels, "Alpha") {
		t.Errorf("Work column missing current-project 'Alpha' entry; labels=%v", labels)
	}
	if contains(labels, "Outliner") {
		t.Errorf("Work column should not carry a duplicate 'Outliner' entry; labels=%v", labels)
	}
	if outlinerRoutes != 1 {
		t.Errorf("Work column should expose exactly one route to the Outliner, got %d; labels=%v", outlinerRoutes, labels)
	}
	if !contains(labels, "Kanban") {
		t.Errorf("Work column missing 'Kanban' entry; labels=%v", labels)
	}
}

// TestPipelineHubRecommendationStripFirstRun verifies the empty-cockpit case:
// no projects + no tasks surfaces the first-run "Create your first project"
// strip above the column grid, matching the Today cockpit and Dashboard.
func TestPipelineHubRecommendationStripFirstRun(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
	m.SetSize(120, 30)

	plain := stripANSI(m.View().Content)
	if !strings.Contains(plain, "Create your first project") {
		t.Fatalf("expected first-run strip CTA in pipeline hub view, got:\n%s", plain)
	}
	stripIdx := strings.Index(plain, "Create your first project")
	headerIdx := strings.Index(plain, "CAPTURE")
	if headerIdx < 0 || stripIdx >= headerIdx {
		t.Fatalf("expected strip (idx=%d) above column headers (idx=%d):\n%s", stripIdx, headerIdx, plain)
	}
}

// TestPipelineHubRecommendationStripWorkOnTask verifies the strip surfaces a
// planned-today task fed through the same engine the cockpit uses. Confirms
// the hub speaks the same Next Action language as Today.
func TestPipelineHubRecommendationStripWorkOnTask(t *testing.T) {
	cfg := newTestConfigAt(t.TempDir())
	project := testutil.NewTestProject("Hub Plan")
	project.Path = cfg.ProjectPath(project.Slug)
	planned, err := models.NewTask("planned-1", 1, models.TaskTypeTask, "Ship pipeline polish")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	planned.WorkflowState = models.StateInProgress
	project.Tasks = []*models.Task{planned}
	setTestProjects(cfg, []*models.Project{project})
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	todayKey := util.StartOfDay(time.Now()).Format(util.DateFormat)
	cfg.Planning().DailyPlan = []config.DailyPlanItem{{
		TaskID:      planned.ID,
		ProjectID:   project.ID,
		PlannedDate: todayKey,
	}}

	m := NewPipelineHubModel(testRuntimeAt(cfg.DataDir(), cfg), nil, nil, project)
	m.SetSize(140, 30)
	m.setOpenLoops(OpenLoops{TodayTotal: 1, HasTasks: true})

	plain := stripANSI(m.View().Content)
	if !strings.Contains(plain, "Next: Ship pipeline polish") {
		t.Fatalf("expected WorkOnTask strip title in hub, got:\n%s", plain)
	}
	if !strings.Contains(plain, "planned today") {
		t.Fatalf("expected planned-today reason badge in hub strip, got:\n%s", plain)
	}
}

// TestPipelineHubRecommendationStripTriageInboxFromOpenLoops verifies the
// inbox-only fallback: when no tasks are loaded but OpenLoops carries an
// inbox count, the strip surfaces a direct "Triage inbox (N items)" CTA.
func TestPipelineHubRecommendationStripTriageInboxFromOpenLoops(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("Inboxy")
	setTestProjects(cfg, []*models.Project{project})
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
	m.SetSize(140, 30)
	m.setOpenLoops(OpenLoops{Inbox: 3, HasTasks: true})

	plain := stripANSI(m.View().Content)
	if !strings.Contains(plain, "Triage inbox") {
		t.Fatalf("expected triage CTA in hub strip, got:\n%s", plain)
	}
	if !strings.Contains(plain, "3 items") {
		t.Fatalf("expected inbox count in hub strip, got:\n%s", plain)
	}
}

// TestPipelineHubRecommendationStripSuppressedWhenNarrow confirms the strip
// inherits the renderer's NextActionStripMinWidth gate so narrow terminals do
// not see truncated CTAs. The column grid still renders.
func TestPipelineHubRecommendationStripSuppressedWhenNarrow(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
	m.SetSize(NextActionStripMinWidth-1, 24)

	plain := stripANSI(m.View().Content)
	if strings.Contains(plain, "Create your first project") || strings.Contains(plain, "Next: ") || strings.Contains(plain, "Triage inbox") {
		t.Fatalf("expected suppressed strip below NextActionStripMinWidth, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Pipeline Overview") {
		t.Fatalf("expected title still rendered at narrow width, got:\n%s", plain)
	}
}

// TestPipelineHubOpenLoopsAwareInterface verifies that PipelineHubModel
// implements openLoopsAware so AppModel.View injects the latest cross-project
// counters before each render — without this the inbox-only / first-run
// strip variants would never surface.
func TestPipelineHubOpenLoopsAwareInterface(t *testing.T) {
	var m any = &PipelineHubModel{}
	if _, ok := m.(openLoopsAware); !ok {
		t.Fatal("expected *PipelineHubModel to implement openLoopsAware")
	}
}

// TestPipelineHubSupportEntryDispatchesShowMsg confirms the Support column's
// Blockers entry dispatches the correct global message so the new column is
// fully navigable, not just decorative.
func TestPipelineHubSupportEntryDispatchesShowMsg(t *testing.T) {
	cfg := newTestConfig()
	m := NewPipelineHubModel(testRuntimeAt("", cfg), nil, nil, nil)
	m.SetSize(140, 30)

	// Move to Support column (index 5)
	for i := 0; i < 5; i++ {
		simulateKey(m, "l")
	}
	if m.columns[m.activeCol].name != "Support" {
		t.Fatalf("expected to land on Support column, got %q", m.columns[m.activeCol].name)
	}
	cmd := m.openSelectedEntry()
	if cmd == nil {
		t.Fatal("expected Support→Blockers entry to dispatch a command")
	}
	msg := cmd()
	if _, ok := msg.(ShowGlobalBlockersMsg); !ok {
		t.Fatalf("expected ShowGlobalBlockersMsg, got %T", msg)
	}
}

func columnNames(cols []pipelineColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.name
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
