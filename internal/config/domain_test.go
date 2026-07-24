package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
			catalog: WorkspaceCatalog{SchemaVersion: 3, Projects: []CatalogProject{{ID: "a", Slug: "a", ColorSlot: ProjectColorSlotCount}}, SystemInboxProjectID: "a"},
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
		{
			name:    "sandbox slug on sandbox project",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "sandbox", Slug: "_sandbox", Sandbox: true}}},
		},
		{
			name:    "sandbox slug on ordinary project",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "not-sandbox", Slug: "_sandbox"}}},
			wantErr: true,
		},
		{
			name:    "traversal slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "escape", Slug: "../escape"}}},
			wantErr: true,
		},
		{
			name:    "noncanonical slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "upper", Slug: "Upper"}}},
			wantErr: true,
		},
		{
			name:    "overlong slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "long", Slug: strings.Repeat("a", models.MaxProjectSlugLength+1)}}},
			wantErr: true,
		},
		{
			name:    "negative color slot",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a", ColorSlot: -1}}},
			wantErr: true,
		},
		{
			name:    "color slot above palette",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a", ColorSlot: ProjectColorSlotCount + 1}}},
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
	m := &models.Project{ID: "p1", Slug: "my-proj", Name: "My Proj", Path: "/absolute/legacy/path", ColorSlot: 17}
	cp := catalogProjectFromModel(m)
	if cp.ColorSlot != m.ColorSlot {
		t.Fatalf("catalog ColorSlot = %d, want %d", cp.ColorSlot, m.ColorSlot)
	}

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
	if rehydrated.ColorSlot != m.ColorSlot {
		t.Errorf("rehydrated ColorSlot = %d, want %d", rehydrated.ColorSlot, m.ColorSlot)
	}
}

func TestWorkspaceCatalogEnsureProjectColorSlotsDeterministicAndIdempotent(t *testing.T) {
	created := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	catalog := WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion,
		Projects: []CatalogProject{
			{ID: "c", Slug: "c", CreatedAt: created.Add(time.Hour), Archived: true},
			{ID: "hidden", Slug: "hidden", CreatedAt: created.Add(-time.Hour), Internal: true},
			{ID: "b", Slug: "b", CreatedAt: created},
			{ID: "preserved", Slug: "preserved", CreatedAt: created.Add(-2 * time.Hour), ColorSlot: 5},
			{ID: "duplicate", Slug: "duplicate", CreatedAt: created.Add(-time.Hour), ColorSlot: 5},
			{ID: "a", Slug: "a", CreatedAt: created, ColorSlot: ProjectColorSlotCount + 1},
			{ID: "sandbox", Slug: "_sandbox", CreatedAt: created.Add(-time.Hour), Sandbox: true, ColorSlot: -1},
		},
	}
	if !catalog.EnsureProjectColorSlots() {
		t.Fatal("expected legacy catalog to change")
	}

	want := map[string]int{"a": 1, "b": 2, "c": 3, "preserved": 5, "duplicate": 5, "hidden": 0, "sandbox": 0}
	for _, project := range catalog.Projects {
		if project.ColorSlot != want[project.ID] {
			t.Errorf("project %s ColorSlot = %d, want %d", project.ID, project.ColorSlot, want[project.ID])
		}
	}
	if catalog.EnsureProjectColorSlots() {
		t.Fatal("second backfill should be idempotent")
	}
}

func TestWorkspaceCatalogProjectColorSlotAllocationIsUniqueThenBalanced(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	seenFirst := make(map[int]bool, ProjectColorSlotCount)
	usage := make([]int, ProjectColorSlotCount+1)
	for i := 1; i <= ProjectColorSlotCount*2+1; i++ {
		id := fmt.Sprintf("project-%03d", i)
		project := &models.Project{ID: id, Slug: id, Name: id, CreatedAt: time.Unix(int64(i), 0)}
		catalog.UpsertProject(project)
		if !validProjectColorSlot(project.ColorSlot) {
			t.Fatalf("project %d received invalid slot %d", i, project.ColorSlot)
		}
		usage[project.ColorSlot]++
		if i <= ProjectColorSlotCount {
			if seenFirst[project.ColorSlot] {
				t.Fatalf("slot %d reused among first %d projects", project.ColorSlot, ProjectColorSlotCount)
			}
			seenFirst[project.ColorSlot] = true
		}
		if i == ProjectColorSlotCount+1 && project.ColorSlot != 1 {
			t.Fatalf("project 49 ColorSlot = %d, want 1", project.ColorSlot)
		}
	}

	minUsage, maxUsage := usage[1], usage[1]
	for slot := 2; slot <= ProjectColorSlotCount; slot++ {
		if usage[slot] < minUsage {
			minUsage = usage[slot]
		}
		if usage[slot] > maxUsage {
			maxUsage = usage[slot]
		}
	}
	if maxUsage-minUsage > 1 {
		t.Fatalf("allocation is unbalanced: min=%d max=%d", minUsage, maxUsage)
	}
}

func TestWorkspaceCatalogProjectColorSlotLifecyclePreservationAndRelease(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	project := &models.Project{ID: "project", Slug: "project", Name: "Project"}
	catalog.UpsertProject(project)
	originalSlot := project.ColorSlot

	project.Name = "Renamed"
	project.Archived = true
	project.Pinned = true
	project.ColorSlot = ProjectColorSlotCount
	catalog.UpsertProject(project)
	if project.ColorSlot != originalSlot || catalog.Projects[0].ColorSlot != originalSlot {
		t.Fatalf("existing slot was overwritten: model=%d catalog=%d want=%d", project.ColorSlot, catalog.Projects[0].ColorSlot, originalSlot)
	}

	sandbox := &models.Project{ID: "sandbox", Slug: "_sandbox", Name: "Sandbox", Sandbox: true}
	internal := &models.Project{ID: "internal", Slug: "internal", Name: "Internal", Internal: true}
	catalog.UpsertProject(sandbox)
	catalog.UpsertProject(internal)
	if sandbox.ColorSlot != 0 || internal.ColorSlot != 0 {
		t.Fatalf("hidden projects received slots: sandbox=%d internal=%d", sandbox.ColorSlot, internal.ColorSlot)
	}

	second := &models.Project{ID: "second", Slug: "second", Name: "Second"}
	catalog.UpsertProject(second)
	if second.ColorSlot != 2 {
		t.Fatalf("hidden projects consumed allocation capacity: second slot=%d, want 2", second.ColorSlot)
	}
	if !catalog.RemoveProjectReferences(project.ID, project.Slug) {
		t.Fatal("expected project deletion to change catalog")
	}
	replacement := &models.Project{ID: "replacement", Slug: "replacement", Name: "Replacement"}
	catalog.UpsertProject(replacement)
	if replacement.ColorSlot != originalSlot {
		t.Fatalf("deleted slot was not released: got %d want %d", replacement.ColorSlot, originalSlot)
	}
	if second.ColorSlot != 2 {
		t.Fatalf("existing project was recolored after deletion: got %d want 2", second.ColorSlot)
	}
}

func TestWorkspaceCatalogProjectColorSlotRestoreCollisions(t *testing.T) {
	t.Run("free tombstoned slot is retained", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		catalog.UpsertProject(&models.Project{ID: "active", Slug: "active", Name: "Active"})
		restored := &models.Project{ID: "restored", Slug: "restored", Name: "Restored", ColorSlot: 7}
		catalog.UpsertProject(restored)
		if restored.ColorSlot != 7 {
			t.Fatalf("restored ColorSlot = %d, want 7", restored.ColorSlot)
		}
	})

	t.Run("collision moves to unused slot below capacity", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		catalog.Projects = []CatalogProject{{ID: "active", Slug: "active", Name: "Active", ColorSlot: 7}}
		restored := &models.Project{ID: "restored", Slug: "restored", Name: "Restored", ColorSlot: 7}
		catalog.UpsertProject(restored)
		if restored.ColorSlot != 1 {
			t.Fatalf("restored ColorSlot = %d, want first unused slot 1", restored.ColorSlot)
		}
		if catalog.Projects[0].ColorSlot != 7 {
			t.Fatal("existing collision owner was recolored")
		}
	})

	t.Run("collision is retained after palette exhaustion", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		for i := 1; i <= ProjectColorSlotCount; i++ {
			id := fmt.Sprintf("active-%02d", i)
			catalog.UpsertProject(&models.Project{ID: id, Slug: id, Name: id})
		}
		restored := &models.Project{ID: "restored", Slug: "restored", Name: "Restored", ColorSlot: 17}
		catalog.UpsertProject(restored)
		if restored.ColorSlot != 17 {
			t.Fatalf("restored ColorSlot = %d, want tombstoned slot 17", restored.ColorSlot)
		}
	})
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
	if err := (&MachineState{AIHarness: "unknown-harness"}).Validate(); err == nil {
		t.Error("unknown AI harness should fail")
	}
}

func TestMachineStateOmitsRetiredStartupAndLayoutFields(t *testing.T) {
	// The Outliner is the sole root: startup destination, Home layout sizing, and
	// sidebar state are no longer persisted and must never appear on save.
	data, err := json.Marshal(NewMachineState())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, field := range []string{"start_view", "start_route", "home_projects_height_pct", "sidebar_enabled"} {
		if strings.Contains(string(data), field) {
			t.Fatalf("machine state should omit retired field %q, got %s", field, data)
		}
	}
}

func TestMachineStateRemoveProjectReferences(t *testing.T) {
	m := &MachineState{
		CurrentProject:   "drop",
		RecentTaskVisits: []RecentTaskVisit{{TaskID: "t", ProjectID: "drop"}, {TaskID: "t2", ProjectID: "keep"}},
		CustomContexts:   map[string][]string{"work": {"drop"}},
	}
	if !m.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if m.CurrentProject != "" {
		t.Errorf("current project should be cleared, got %q", m.CurrentProject)
	}
	if len(m.RecentTaskVisits) != 1 || m.RecentTaskVisits[0].ProjectID != "keep" {
		t.Errorf("recent visits not cleaned: %+v", m.RecentTaskVisits)
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
