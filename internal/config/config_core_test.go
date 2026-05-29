package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.DataDir == "" {
		t.Error("DataDir should not be empty")
	}
	if cfg.Editor != "code" {
		t.Errorf("expected default Editor 'code', got '%s'", cfg.Editor)
	}
	if cfg.DefaultContext != "personal" {
		t.Errorf("expected default Context 'personal', got '%s'", cfg.DefaultContext)
	}
	if !cfg.FzfEnabled {
		t.Error("FzfEnabled should default to true")
	}
	if !cfg.GlowEnabled {
		t.Error("GlowEnabled should default to true")
	}
	if cfg.Projects == nil {
		t.Error("Projects should not be nil")
	}
	if cfg.SectionColors == nil {
		t.Error("NewConfig should initialize SectionColors map")
	}
	if cfg.Version != CurrentConfigVersion {
		t.Errorf("expected Version %d, got %d", CurrentConfigVersion, cfg.Version)
	}
}

func TestConfig_Projects(t *testing.T) {
	cfg := NewConfig()

	project1 := &models.Project{
		ID:   "p1",
		Slug: "project-one",
		Name: "Project One",
	}
	project2 := &models.Project{
		ID:   "p2",
		Slug: "project-two",
		Name: "Project Two",
	}

	cfg.AddProject(project1)
	cfg.AddProject(project2)

	if len(cfg.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(cfg.Projects))
	}

	found := cfg.GetProject("p1")
	if found == nil || found.ID != "p1" {
		t.Error("GetProject by ID failed")
	}

	found = cfg.GetProject("project-two")
	if found == nil || found.ID != "p2" {
		t.Error("GetProject by slug failed")
	}

	found = cfg.GetProject("Project One")
	if found != nil {
		t.Error("GetProject by name should return nil (only ID and slug are supported)")
	}

	found = cfg.GetProject("non-existent")
	if found != nil {
		t.Error("expected nil for non-existent project")
	}

	removed := cfg.RemoveProject("p1")
	if !removed {
		t.Error("RemoveProject should return true")
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project after removal, got %d", len(cfg.Projects))
	}

	removed = cfg.RemoveProject("non-existent")
	if removed {
		t.Error("RemoveProject should return false for non-existent")
	}
}

func TestConfig_CurrentProject(t *testing.T) {
	cfg := NewConfig()

	project := &models.Project{
		ID:   "p1",
		Slug: "test-project",
		Name: "Test Project",
	}
	cfg.AddProject(project)

	if cfg.GetCurrentProject() != nil {
		t.Error("expected nil current project initially")
	}

	if !cfg.SetCurrentProject("p1") {
		t.Error("SetCurrentProject should return true")
	}
	if cfg.CurrentProject != "p1" {
		t.Errorf("expected CurrentProject 'p1', got '%s'", cfg.CurrentProject)
	}

	current := cfg.GetCurrentProject()
	if current == nil || current.ID != "p1" {
		t.Error("GetCurrentProject failed")
	}

	if cfg.SetCurrentProject("non-existent") {
		t.Error("SetCurrentProject should return false for non-existent")
	}
	if cfg.CurrentProject != "p1" {
		t.Error("CurrentProject should not change on failed set")
	}

	cfg.RemoveProject("p1")
	if cfg.CurrentProject != "" {
		t.Error("CurrentProject should be cleared when project is removed")
	}
}

// TestRemoveProject_ClearsCurrentBySlug covers legacy configs where
// CurrentProject was stored as the project's slug rather than its ID.
// Without slug-aware clearing the field would dangle and GetCurrentProject
// would silently keep returning nil while leaving stale data on disk.
func TestRemoveProject_ClearsCurrentBySlug(t *testing.T) {
	cfg := NewConfig()
	project := &models.Project{
		ID:   "p1",
		Slug: "legacy-slug",
		Name: "Legacy",
	}
	cfg.AddProject(project)
	// Simulate legacy state: CurrentProject holds the slug, not the ID.
	cfg.CurrentProject = "legacy-slug"

	if !cfg.RemoveProject("p1") {
		t.Fatal("RemoveProject should return true")
	}
	if cfg.CurrentProject != "" {
		t.Errorf("CurrentProject should be cleared when removed project's slug matches; got %q", cfg.CurrentProject)
	}
}

func TestConfig_GetActiveArchivedProjects(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "a1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "a2", Name: "Active 2", Archived: false}
	archived := &models.Project{ID: "ar1", Name: "Archived 1", Archived: true}

	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.AddProject(archived)

	activeProjects := cfg.GetActiveProjects()
	if len(activeProjects) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(activeProjects))
	}

	archivedProjects := cfg.GetArchivedProjects()
	if len(archivedProjects) != 1 {
		t.Errorf("expected 1 archived project, got %d", len(archivedProjects))
	}
}

func TestConfig_EnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "tt-data", "nested")

	cfg := NewConfig()
	cfg.DataDir = dataDir

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir failed: %v", err)
	}

	info, err := os.Stat(dataDir)
	if os.IsNotExist(err) {
		t.Error("data directory was not created")
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestConfig_GetProjectPath(t *testing.T) {
	cfg := NewConfig()
	cfg.DataDir = "/home/user/.tt"

	path := cfg.GetProjectPath("my-project")
	expected := "/home/user/.tt/projects/my-project"

	if path != expected {
		t.Errorf("expected path '%s', got '%s'", expected, path)
	}
}

func TestRemoveProject_CleansDailyPlan(t *testing.T) {
	cfg := NewConfig()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "proj1", Name: "P1"})
	cfg.AddProject(&models.Project{ID: "p2", Slug: "proj2", Name: "P2"})

	cfg.DailyPlan = []DailyPlanItem{
		{TaskID: "t1", ProjectID: "p1", PlannedDate: "2026-04-13"},
		{TaskID: "t2", ProjectID: "p2", PlannedDate: "2026-04-13"},
		{TaskID: "t3", ProjectID: "p1", PlannedDate: "2026-04-14"},
	}

	cfg.RemoveProject("p1")

	if len(cfg.DailyPlan) != 1 {
		t.Fatalf("expected 1 daily plan item after removal, got %d", len(cfg.DailyPlan))
	}
	if cfg.DailyPlan[0].ProjectID != "p2" {
		t.Errorf("expected remaining item to be for p2, got %s", cfg.DailyPlan[0].ProjectID)
	}
}

func TestRemoveProject_CleansAllReferences(t *testing.T) {
	cfg := NewConfig()
	cfg.AddProject(&models.Project{ID: "p1", Slug: "proj1", Name: "P1"})
	cfg.CurrentProject = "p1"
	cfg.ProjectScoringProfile = map[string]string{"p1": "default"}
	cfg.ProjectViewStates = map[string]ProjectViewState{"p1": {}}
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{"p1": {}}
	cfg.FocusQueue = []FocusQueueItem{{TaskID: "t1", ProjectID: "p1"}}
	cfg.DailyPlan = []DailyPlanItem{{TaskID: "t1", ProjectID: "p1"}}
	cfg.RecentTaskVisits = []RecentTaskVisit{{TaskID: "t1", ProjectID: "p1"}}
	cfg.CycleProjects = []string{"p1"}
	cfg.ContextCycleProjects = map[string][]string{"personal": {"p1"}}
	cfg.CustomContexts = map[string][]string{"work": {"p1"}}
	cfg.ActionInboxProjectID = "p1"

	cfg.RemoveProject("p1")

	if cfg.CurrentProject != "" {
		t.Error("CurrentProject should be cleared")
	}
	if _, ok := cfg.ProjectScoringProfile["p1"]; ok {
		t.Error("ProjectScoringProfile should be cleaned")
	}
	if _, ok := cfg.ProjectViewStates["p1"]; ok {
		t.Error("ProjectViewStates should be cleaned")
	}
	if _, ok := cfg.WorkflowEvalStates["p1"]; ok {
		t.Error("WorkflowEvalStates should be cleaned")
	}
	if len(cfg.FocusQueue) != 0 {
		t.Errorf("FocusQueue should be empty, got %d", len(cfg.FocusQueue))
	}
	if len(cfg.DailyPlan) != 0 {
		t.Errorf("DailyPlan should be empty, got %d", len(cfg.DailyPlan))
	}
	if len(cfg.RecentTaskVisits) != 0 {
		t.Errorf("RecentTaskVisits should be empty, got %d", len(cfg.RecentTaskVisits))
	}
	if len(cfg.CycleProjects) != 0 {
		t.Errorf("CycleProjects should be empty, got %d", len(cfg.CycleProjects))
	}
	if len(cfg.ContextCycleProjects["personal"]) != 0 {
		t.Errorf("ContextCycleProjects should be empty, got %d", len(cfg.ContextCycleProjects["personal"]))
	}
	if len(cfg.CustomContexts["work"]) != 0 {
		t.Errorf("CustomContexts should be empty, got %d", len(cfg.CustomContexts["work"]))
	}
	if cfg.ActionInboxProjectID != "" {
		t.Error("ActionInboxProjectID should be cleared")
	}
}

func TestConfig_GetWeekEndDay(t *testing.T) {
	tests := []struct {
		stored string
		want   string
	}{
		{"", "sun"},
		{"sun", "sun"},
		{"sat", "sat"},
		{"fri", "fri"},
		{"invalid", "sun"},
		{"SAT", "sun"}, // must be lowercase
	}
	for _, tt := range tests {
		cfg := NewConfig()
		cfg.WeekEndDay = tt.stored
		if got := cfg.GetWeekEndDay(); got != tt.want {
			t.Errorf("GetWeekEndDay() with stored=%q = %q, want %q", tt.stored, got, tt.want)
		}
	}
}

func TestValidateWeekEndDay(t *testing.T) {
	valid := []string{"", "fri", "sat", "sun"}
	for _, v := range valid {
		if err := ValidateWeekEndDay(v); err != nil {
			t.Errorf("ValidateWeekEndDay(%q) unexpected error: %v", v, err)
		}
	}
	invalid := []string{"monday", "Sat", "sunday", "SAT", "FRI", "saturday"}
	for _, v := range invalid {
		if err := ValidateWeekEndDay(v); err == nil {
			t.Errorf("ValidateWeekEndDay(%q) expected error, got nil", v)
		}
	}
}

func TestConfig_GetTimeFormat(t *testing.T) {
	tests := []struct {
		stored string
		want   string
	}{
		{"", "15:04"},
		{"24h", "15:04"},
		{"12h", "3:04pm"},
		{"invalid", "15:04"},
	}
	for _, tt := range tests {
		cfg := NewConfig()
		cfg.TimeFormat = tt.stored
		if got := cfg.GetTimeFormat(); got != tt.want {
			t.Errorf("GetTimeFormat() with stored=%q = %q, want %q", tt.stored, got, tt.want)
		}
	}
}

func TestConfig_GetTimeFormat_NilReceiver(t *testing.T) {
	var cfg *Config
	if got := cfg.GetTimeFormat(); got != "15:04" {
		t.Errorf("GetTimeFormat() on nil receiver = %q, want %q", got, "15:04")
	}
}

func TestConfig_GetPopupTimeoutSeconds(t *testing.T) {
	cases := []struct {
		stored *int
		want   int
	}{
		{nil, 0},
		{intPtr(0), 0},
		{intPtr(5), 5},
		{intPtr(-1), 0},
		{intPtr(-1000), 0},
	}
	for _, tc := range cases {
		cfg := NewConfig()
		cfg.PopupTimeoutSeconds = tc.stored
		if got := cfg.GetPopupTimeoutSeconds(); got != tc.want {
			storedDesc := "nil"
			if tc.stored != nil {
				storedDesc = fmt.Sprintf("%d", *tc.stored)
			}
			t.Errorf("stored=%s: expected %d, got %d", storedDesc, tc.want, got)
		}
	}
}

func TestValidateTimeFormat(t *testing.T) {
	valid := []string{"", "12h", "24h"}
	for _, v := range valid {
		if err := ValidateTimeFormat(v); err != nil {
			t.Errorf("ValidateTimeFormat(%q) unexpected error: %v", v, err)
		}
	}
	invalid := []string{"12H", "24H", "12", "24", "hour", "AM", "12h24h"}
	for _, v := range invalid {
		if err := ValidateTimeFormat(v); err == nil {
			t.Errorf("ValidateTimeFormat(%q) expected error, got nil", v)
		}
	}
}
