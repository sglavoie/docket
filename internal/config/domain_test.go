package config

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNewWorkspaceCatalogDefaults(t *testing.T) {
	c := NewWorkspaceCatalog()
	if c.SchemaVersion != CurrentConfigVersion {
		t.Errorf("SchemaVersion = %d, want %d", c.SchemaVersion, CurrentConfigVersion)
	}
	if c.Projects == nil {
		t.Error("Projects should be non-nil")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("default catalog should validate: %v", err)
	}
}

func TestWorkspaceCatalogValidate(t *testing.T) {
	tests := []struct {
		name    string
		catalog WorkspaceCatalog
		wantErr bool
	}{
		{
			name:    "valid",
			catalog: WorkspaceCatalog{SchemaVersion: 3, Projects: []CatalogProject{{ID: "a", Slug: "a"}}, SystemInboxProjectID: "a"},
		},
		{
			name:    "non-positive schema version",
			catalog: WorkspaceCatalog{SchemaVersion: 0},
			wantErr: true,
		},
		{
			name:    "empty project id",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{Slug: "a"}}},
			wantErr: true,
		},
		{
			name:    "empty project slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a"}}},
			wantErr: true,
		},
		{
			name:    "duplicate id",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a"}, {ID: "a", Slug: "b"}}},
			wantErr: true,
		},
		{
			name:    "duplicate slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "s"}, {ID: "b", Slug: "s"}}},
			wantErr: true,
		},
		{
			name:    "dangling system inbox",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a"}}, SystemInboxProjectID: "missing"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.catalog.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestCatalogProjectOmitsAbsolutePath(t *testing.T) {
	// The catalog model has no Path field; a project's location is derived from
	// the data dir and slug, never persisted.
	m := &models.Project{ID: "p1", Slug: "my-proj", Name: "My Proj", Path: "/absolute/legacy/path"}
	cp := catalogProjectFromModel(m)

	dataDir := "/data/root"
	got := cp.CatalogPath(dataDir)
	want := filepath.Join(dataDir, "projects", "my-proj")
	if got != want {
		t.Errorf("CatalogPath = %q, want %q", got, want)
	}

	rehydrated := cp.ToModel(dataDir)
	if rehydrated.Path != want {
		t.Errorf("ToModel Path = %q, want derived %q (must not carry the legacy absolute path)", rehydrated.Path, want)
	}
	if rehydrated.Path == m.Path {
		t.Error("rehydrated project must not reuse the persisted legacy absolute path")
	}
}

func TestWorkspaceCatalogRemoveProjectReferences(t *testing.T) {
	c := WorkspaceCatalog{
		SchemaVersion:        3,
		Projects:             []CatalogProject{{ID: "keep", Slug: "keep"}, {ID: "drop", Slug: "drop-slug"}},
		SystemInboxProjectID: "drop",
	}
	if !c.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if len(c.Projects) != 1 || c.Projects[0].ID != "keep" {
		t.Errorf("expected only 'keep' to remain, got %+v", c.Projects)
	}
	if c.SystemInboxProjectID != "" {
		t.Errorf("system inbox reference should be cleared, got %q", c.SystemInboxProjectID)
	}
	if c.RemoveProjectReferences("nonexistent", "") {
		t.Error("removing a missing project should report no change")
	}
}

func TestNewPlanningStateDefaults(t *testing.T) {
	p := NewPlanningState()
	if err := p.Validate(); err != nil {
		t.Errorf("default planning should validate: %v", err)
	}
}

func TestPlanningStateValidate(t *testing.T) {
	valid := &PlanningState{
		FocusQueue:   []FocusQueueItem{{TaskID: "t", ProjectID: "p"}},
		DailyPlan:    []DailyPlanItem{{TaskID: "t", ProjectID: "p"}},
		SavedViews:   []savedView{{Name: "v"}},
		SavedFilters: []SavedFilter{{Name: "f"}},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid planning should pass: %v", err)
	}

	cases := []*PlanningState{
		{FocusQueue: []FocusQueueItem{{ProjectID: "p"}}},
		{DailyPlan: []DailyPlanItem{{ProjectID: "p"}}},
		{SavedViews: []savedView{{}}},
		{SavedFilters: []SavedFilter{{}}},
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestPlanningStateRemoveProjectReferences(t *testing.T) {
	p := &PlanningState{
		FocusQueue: []FocusQueueItem{{TaskID: "t1", ProjectID: "keep"}, {TaskID: "t2", ProjectID: "drop"}},
		DailyPlan:  []DailyPlanItem{{TaskID: "t3", ProjectID: "drop-slug"}},
	}
	if !p.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if len(p.FocusQueue) != 1 || p.FocusQueue[0].ProjectID != "keep" {
		t.Errorf("focus queue not cleaned: %+v", p.FocusQueue)
	}
	if len(p.DailyPlan) != 0 {
		t.Errorf("daily plan not cleaned: %+v", p.DailyPlan)
	}
}

func TestNewSettingsDefaults(t *testing.T) {
	s := NewSettings()
	if !s.FzfEnabled || !s.GlowEnabled {
		t.Error("fzf/glow should default enabled")
	}
	if s.DefaultContext != "personal" {
		t.Errorf("DefaultContext = %q, want personal", s.DefaultContext)
	}
	if s.UndoHistoryDepth == nil || *s.UndoHistoryDepth != DefaultUndoHistoryDepth {
		t.Errorf("UndoHistoryDepth should default to %d", DefaultUndoHistoryDepth)
	}
	if s.KanbanSettings.DefaultMode != defaultKanbanMode {
		t.Errorf("KanbanSettings.DefaultMode = %q, want %q", s.KanbanSettings.DefaultMode, defaultKanbanMode)
	}
	if err := s.Validate(); err != nil {
		t.Errorf("default settings should validate: %v", err)
	}
}

func TestSettingsValidate(t *testing.T) {
	neg := -1
	if err := (&Settings{FzfTimeout: &neg}).Validate(); err == nil {
		t.Error("negative fzf timeout should fail")
	}
	if err := (&Settings{UndoHistoryDepth: &neg}).Validate(); err == nil {
		t.Error("negative undo depth should fail")
	}
	zero := 0
	if err := (&Settings{FzfTimeout: &zero, UndoHistoryDepth: &zero}).Validate(); err != nil {
		t.Errorf("zero values should validate: %v", err)
	}
}

func TestSettingsAIConsentDefaultsOff(t *testing.T) {
	s := NewSettings()
	if s.AIConsent.Granted {
		t.Error("AI consent must default to not granted (explicit opt-in)")
	}
}

func TestNewAutomationConfigDefaults(t *testing.T) {
	a := NewAutomationConfig()
	if err := a.Validate(); err != nil {
		t.Errorf("default automation should validate: %v", err)
	}
}

func TestAutomationConfigValidate(t *testing.T) {
	if err := (&AutomationConfig{ScoringProfiles: nil}).Validate(); err != nil {
		t.Errorf("empty automation should validate: %v", err)
	}
}

func TestAutomationConfigRemoveProjectReferences(t *testing.T) {
	a := &AutomationConfig{
		ProjectScoringProfile: map[string]string{"keep": "prof", "drop": "prof2"},
		WorkflowEvalStates:    map[string]WorkflowEvalState{"drop-slug": {}},
	}
	if !a.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if _, ok := a.ProjectScoringProfile["drop"]; ok {
		t.Error("scoring profile for dropped project should be removed")
	}
	if _, ok := a.ProjectScoringProfile["keep"]; !ok {
		t.Error("scoring profile for kept project should remain")
	}
	if _, ok := a.WorkflowEvalStates["drop-slug"]; ok {
		t.Error("workflow eval state for dropped slug should be removed")
	}
	if a.RemoveProjectReferences("missing", "") {
		t.Error("removing a missing project should report no change")
	}
}

func TestNewMachineStateDefaults(t *testing.T) {
	m := NewMachineState()
	if m.Editor != "code" {
		t.Errorf("Editor = %q, want code", m.Editor)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("default machine should validate: %v", err)
	}
}

func TestMachineStateValidate(t *testing.T) {
	bad := 5
	if err := (&MachineState{DashboardBottomLayout: &bad}).Validate(); err == nil {
		t.Error("out-of-range dashboard layout should fail")
	}
	ok := 1
	if err := (&MachineState{DashboardBottomLayout: &ok}).Validate(); err != nil {
		t.Errorf("in-range dashboard layout should pass: %v", err)
	}
}

func TestMachineStateRemoveProjectReferences(t *testing.T) {
	m := &MachineState{
		CurrentProject:       "drop",
		ProjectViewStates:    map[string]ProjectViewState{"drop-slug": {}, "keep": {}},
		RecentTaskVisits:     []RecentTaskVisit{{TaskID: "t", ProjectID: "drop"}, {TaskID: "t2", ProjectID: "keep"}},
		CycleProjects:        []string{"drop", "keep"},
		ContextCycleProjects: map[string][]string{"work": {"drop-slug", "keep"}},
		CustomContexts:       map[string][]string{"work": {"drop"}},
	}
	if !m.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if m.CurrentProject != "" {
		t.Errorf("current project should be cleared, got %q", m.CurrentProject)
	}
	if _, ok := m.ProjectViewStates["drop-slug"]; ok {
		t.Error("view state for dropped slug should be removed")
	}
	if len(m.RecentTaskVisits) != 1 || m.RecentTaskVisits[0].ProjectID != "keep" {
		t.Errorf("recent visits not cleaned: %+v", m.RecentTaskVisits)
	}
	if len(m.CycleProjects) != 1 || m.CycleProjects[0] != "keep" {
		t.Errorf("cycle projects not cleaned: %+v", m.CycleProjects)
	}
	if got := m.ContextCycleProjects["work"]; len(got) != 1 || got[0] != "keep" {
		t.Errorf("context cycle not cleaned: %+v", got)
	}
	if got := m.CustomContexts["work"]; len(got) != 0 {
		t.Errorf("custom context not cleaned: %+v", got)
	}
}

func TestNewDomainsYieldsDefaults(t *testing.T) {
	d := NewDomains()
	if d.Catalog == nil || d.Planning == nil || d.Settings == nil || d.Automation == nil || d.Machine == nil {
		t.Fatal("all domains should be populated with defaults")
	}
	if d.Settings.DefaultContext != "personal" {
		t.Errorf("nil-config settings should carry defaults, got %q", d.Settings.DefaultContext)
	}
}

func TestDomainsRemoveProjectReferencesFansOut(t *testing.T) {
	d := &Domains{
		Catalog:    &WorkspaceCatalog{SchemaVersion: 3, Projects: []CatalogProject{{ID: "drop", Slug: "drop"}}, SystemInboxProjectID: "drop"},
		Planning:   &PlanningState{FocusQueue: []FocusQueueItem{{TaskID: "t", ProjectID: "drop"}}},
		Automation: &AutomationConfig{ProjectScoringProfile: map[string]string{"drop": "p"}},
		Machine:    &MachineState{CurrentProject: "drop"},
	}
	if !d.RemoveProjectReferences("drop", "drop") {
		t.Fatal("expected change across domains")
	}
	if len(d.Catalog.Projects) != 0 {
		t.Error("catalog project not removed")
	}
	if len(d.Planning.FocusQueue) != 0 {
		t.Error("planning focus queue not cleaned")
	}
	if _, ok := d.Automation.ProjectScoringProfile["drop"]; ok {
		t.Error("automation scoring profile not cleaned")
	}
	if d.Machine.CurrentProject != "" {
		t.Error("machine current project not cleared")
	}
}
