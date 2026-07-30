package config

import (
	"encoding/json"
	"errors"
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
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{Slug: "a"}}}, wantErr: true,
		},
		{
			name:    "empty project slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a"}}}, wantErr: true,
		},
		{
			name:    "duplicate id",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a"}, {ID: "a", Slug: "b"}}}, wantErr: true,
		},
		{
			name:    "duplicate slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "s"}, {ID: "b", Slug: "s"}}}, wantErr: true,
		},
		{
			name:    "dangling system inbox",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a"}}, SystemInboxProjectID: "missing"},
			wantErr: true,
		},
		{
			name:    "archived system inbox",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "inbox", Slug: "inbox", Archived: true}}, SystemInboxProjectID: "inbox"},
			wantErr: true,
		},
		{
			name:    "retired sandbox slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "not-sandbox", Slug: "_sandbox"}}}, wantErr: true,
		},
		{
			name:    "traversal slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "escape", Slug: "../escape"}}}, wantErr: true,
		},
		{
			name:    "noncanonical slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "upper", Slug: "Upper"}}}, wantErr: true,
		},
		{
			name:    "overlong slug",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "long", Slug: strings.Repeat("a", models.MaxProjectSlugLength+1)}}}, wantErr: true,
		},
		{
			name:    "negative color slot",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a", ColorSlot: -1}}}, wantErr: true,
		},
		{
			name:    "color slot above palette",
			catalog: WorkspaceCatalog{SchemaVersion: 1, Projects: []CatalogProject{{ID: "a", Slug: "a", ColorSlot: ProjectColorSlotCount + 1}}}, wantErr: true,
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
			{ID: "b", Slug: "b", CreatedAt: created},
			{ID: "preserved", Slug: "preserved", CreatedAt: created.Add(-2 * time.Hour), ColorSlot: 5},
			{ID: "duplicate", Slug: "duplicate", CreatedAt: created.Add(-time.Hour), ColorSlot: 5},
			{ID: "a", Slug: "a", CreatedAt: created, ColorSlot: ProjectColorSlotCount + 1},
		},
	}
	if !catalog.EnsureProjectColorSlots() {
		t.Fatal("expected legacy catalog to change")
	}

	want := map[string]int{"a": 1, "b": 2, "c": 3, "preserved": 5, "duplicate": 5}
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
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
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
	if err := catalog.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	originalSlot := project.ColorSlot

	project.Name = "Renamed"
	project.Archived = true
	project.Pinned = true
	project.ColorSlot = ProjectColorSlotCount
	if err := catalog.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if project.ColorSlot != originalSlot || catalog.Projects[0].ColorSlot != originalSlot {
		t.Fatalf("existing slot was overwritten: model=%d catalog=%d want=%d", project.ColorSlot, catalog.Projects[0].ColorSlot, originalSlot)
	}

	second := &models.Project{ID: "second", Slug: "second", Name: "Second"}
	if err := catalog.UpsertProject(second); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if second.ColorSlot != 2 {
		t.Fatalf("archived projects consumed allocation capacity: second slot=%d, want 2", second.ColorSlot)
	}
	if !catalog.RemoveProjectReferences(project.ID, project.Slug) {
		t.Fatal("expected project deletion to change catalog")
	}
	replacement := &models.Project{ID: "replacement", Slug: "replacement", Name: "Replacement"}
	if err := catalog.UpsertProject(replacement); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
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
		if err := catalog.UpsertProject(&models.Project{ID: "active", Slug: "active", Name: "Active"}); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
		restored := &models.Project{ID: "restored", Slug: "restored", Name: "Restored", ColorSlot: 7}
		if err := catalog.UpsertProject(restored); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
		if restored.ColorSlot != 7 {
			t.Fatalf("restored ColorSlot = %d, want 7", restored.ColorSlot)
		}
	})

	t.Run("collision moves to unused slot below capacity", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		catalog.Projects = []CatalogProject{{ID: "active", Slug: "active", Name: "Active", ColorSlot: 7}}
		restored := &models.Project{ID: "restored", Slug: "restored", Name: "Restored", ColorSlot: 7}
		if err := catalog.UpsertProject(restored); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
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
			if err := catalog.UpsertProject(&models.Project{ID: id, Slug: id, Name: id}); err != nil {
				t.Fatalf("UpsertProject: %v", err)
			}
		}
		restored := &models.Project{ID: "restored", Slug: "restored", Name: "Restored", ColorSlot: 17}
		if err := catalog.UpsertProject(restored); err != nil {
			t.Fatalf("UpsertProject: %v", err)
		}
		if restored.ColorSlot != 17 {
			t.Fatalf("restored ColorSlot = %d, want tombstoned slot 17", restored.ColorSlot)
		}
	})
}

func TestWorkspaceCatalogRemoveProjectReferences(t *testing.T) {
	c := WorkspaceCatalog{
		SchemaVersion:        3,
		Projects:             []CatalogProject{{ID: "keep", Slug: "keep"}, {ID: "drop", Slug: "drop-slug"}},
		SystemInboxProjectID: "keep",
	}
	changed, err := c.RemoveProject("drop", "drop-slug")
	if err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	if len(c.Projects) != 1 || c.Projects[0].ID != "keep" {
		t.Errorf("expected only 'keep' to remain, got %+v", c.Projects)
	}
	if c.SystemInboxProjectID != "keep" {
		t.Errorf("system inbox reference changed, got %q", c.SystemInboxProjectID)
	}
	if _, err := c.RemoveProject("keep", "keep"); !errors.Is(err, ErrProtectedSystemInbox) {
		t.Fatalf("RemoveProject(system Inbox) error = %v, want ErrProtectedSystemInbox", err)
	}
	if len(c.Projects) != 1 || c.Projects[0].ID != "keep" {
		t.Fatalf("protected removal mutated catalog: %+v", c.Projects)
	}
	if c.RemoveProjectReferences("nonexistent", "") {
		t.Error("removing a missing project should report no change")
	}
}

func TestWorkspaceCatalogProtectedInboxUpdatesFailClosed(t *testing.T) {
	created := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	catalog := WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion,
		Projects: []CatalogProject{{
			ID: "inbox", Slug: "inbox", Name: "Inbox", Context: "personal",
			CreatedAt: created, UpdatedAt: created, ColorSlot: 9,
		}},
		SystemInboxProjectID: "inbox",
	}

	rename := catalog.Projects[0].ToModel("/workspace")
	rename.Name = "Renamed"
	rename.Slug = "renamed"
	if err := catalog.UpsertProject(rename); !errors.Is(err, ErrProtectedSystemInbox) {
		t.Fatalf("UpsertProject(rename system Inbox) error = %v, want ErrProtectedSystemInbox", err)
	}
	if got := catalog.Projects[0]; got.Name != "Inbox" || got.Slug != "inbox" || got.ColorSlot != 9 {
		t.Fatalf("rejected rename mutated catalog: %+v", got)
	}

	archive := catalog.Projects[0].ToModel("/workspace")
	archive.Archive()
	if err := catalog.UpsertProject(archive); !errors.Is(err, ErrProtectedSystemInbox) {
		t.Fatalf("UpsertProject(archive system Inbox) error = %v, want ErrProtectedSystemInbox", err)
	}
	if catalog.Projects[0].Archived || catalog.Projects[0].ArchivedAt != nil {
		t.Fatalf("rejected archive mutated catalog: %+v", catalog.Projects[0])
	}

	edit := catalog.Projects[0].ToModel("/workspace")
	edit.Description = "Visible capture project"
	edit.Context = "work"
	edit.Pin()
	edit.ColorSlot = 48
	if err := catalog.UpsertProject(edit); err != nil {
		t.Fatalf("UpsertProject(allowed system Inbox metadata): %v", err)
	}
	if got := catalog.Projects[0]; got.Description != edit.Description || got.Context != "work" || !got.Pinned || got.ColorSlot != 9 {
		t.Fatalf("allowed metadata update lost fields or durable color: %+v", got)
	}

	ordinary := &models.Project{
		ID: "ordinary", Slug: "ordinary", Name: "Inbox", Context: "personal",
		CreatedAt: created, UpdatedAt: created,
	}
	if err := catalog.UpsertProject(ordinary); err != nil {
		t.Fatalf("display-name match was treated as protected identity: %v", err)
	}
	ordinary.Name = "Renamed Into Inbox"
	ordinary.Slug = "inbox"
	if err := catalog.UpsertProject(ordinary); err == nil {
		t.Fatal("ordinary project renamed into the protected Inbox slug")
	}
	if got := catalog.Projects[1]; got.Slug != "ordinary" {
		t.Fatalf("rejected protected-slug collision mutated ordinary project: %+v", got)
	}
}

func TestWorkspaceCatalogSystemInboxRepairIsExplicit(t *testing.T) {
	catalog := WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion,
		Projects: []CatalogProject{{
			ID: "legacy", Slug: LegacyActionInboxSlug, Name: "Action Inbox", Context: "personal",
		}},
		SystemInboxProjectID: "legacy",
	}
	project := catalog.Projects[0].ToModel("/workspace")
	project.Name = SystemInboxName
	if err := catalog.UpsertProject(project); !errors.Is(err, ErrProtectedSystemInbox) {
		t.Fatalf("ordinary Inbox rename error = %v, want ErrProtectedSystemInbox", err)
	}
	if err := catalog.RepairSystemInboxProject(project); err != nil {
		t.Fatalf("RepairSystemInboxProject: %v", err)
	}
	if got := catalog.Projects[0]; got.Name != SystemInboxName || got.ID != catalog.SystemInboxProjectID {
		t.Fatalf("system Inbox repair = %+v, id = %q", got, catalog.SystemInboxProjectID)
	}
	if !validProjectColorSlot(catalog.Projects[0].ColorSlot) {
		t.Fatalf("system Inbox repair did not allocate a color: %+v", catalog.Projects[0])
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
		NextOrder: []ProjectTaskLocator{{TaskID: "t", ProjectID: "p"}},
		Today:     []ProjectTaskLocator{{TaskID: "t", ProjectID: "p"}},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid planning should pass: %v", err)
	}

	cases := []*PlanningState{
		{NextOrder: []ProjectTaskLocator{{ProjectID: "p"}}},
		{Today: []ProjectTaskLocator{{ProjectID: "p"}}},
		{NextOrder: []ProjectTaskLocator{{TaskID: "t", ProjectID: "p"}, {TaskID: "t", ProjectID: "p"}}},
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestPlanningStateRemoveProjectReferences(t *testing.T) {
	p := &PlanningState{
		NextOrder: []ProjectTaskLocator{{TaskID: "t1", ProjectID: "keep"}, {TaskID: "t2", ProjectID: "drop"}},
		Today:     []ProjectTaskLocator{{TaskID: "t3", ProjectID: "drop-slug"}},
	}
	if !p.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if len(p.NextOrder) != 1 || p.NextOrder[0].ProjectID != "keep" {
		t.Errorf("focus queue not cleaned: %+v", p.NextOrder)
	}
	if len(p.Today) != 0 {
		t.Errorf("daily plan not cleaned: %+v", p.Today)
	}
}

func TestNewMachineStateDefaults(t *testing.T) {
	m := NewMachineState()
	if m.Theme != "dark" {
		t.Errorf("Theme = %q, want deterministic dark default", m.Theme)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("default machine should validate: %v", err)
	}
}

func TestMachineStateValidate(t *testing.T) {
	if err := (&MachineState{Theme: "sepia"}).Validate(); err == nil {
		t.Error("unknown machine theme should fail")
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
		CustomContexts: map[string][]string{"work": {"drop"}, "home": {"keep"}},
	}
	if !m.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if got := m.CustomContexts["work"]; len(got) != 0 {
		t.Errorf("custom context not cleaned: %+v", got)
	}
	if got := m.CustomContexts["home"]; len(got) != 1 || got[0] != "keep" {
		t.Errorf("unrelated custom context changed: %+v", got)
	}
}

func TestNewDomainsYieldsDefaults(t *testing.T) {
	d := NewDomains()
	if d.Catalog == nil || d.Planning == nil || d.Machine == nil {
		t.Fatal("all domains should be populated with defaults")
	}
}

func TestDomainsRemoveProjectReferencesFansOut(t *testing.T) {
	d := &Domains{
		Catalog:  &WorkspaceCatalog{SchemaVersion: 3, Projects: []CatalogProject{{ID: "keep", Slug: "keep"}, {ID: "drop", Slug: "drop"}}, SystemInboxProjectID: "keep"},
		Planning: &PlanningState{NextOrder: []ProjectTaskLocator{{TaskID: "t", ProjectID: "drop"}}},
		Machine:  &MachineState{CustomContexts: map[string][]string{"work": {"drop"}}},
	}
	changed, err := d.RemoveProject("drop", "drop")
	if err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}
	if !changed {
		t.Fatal("expected change across domains")
	}
	if len(d.Catalog.Projects) != 1 || d.Catalog.Projects[0].ID != "keep" {
		t.Error("catalog project not removed")
	}
	if len(d.Planning.NextOrder) != 0 {
		t.Error("planning focus queue not cleaned")
	}
	if len(d.Machine.CustomContexts["work"]) != 0 {
		t.Error("machine custom context not cleaned")
	}
}

func TestDomainsProtectedInboxRemovalIsAtomic(t *testing.T) {
	d := &Domains{
		Catalog: &WorkspaceCatalog{
			SchemaVersion:        CurrentConfigVersion,
			Projects:             []CatalogProject{{ID: "inbox", Slug: "inbox", Name: "Inbox"}},
			SystemInboxProjectID: "inbox",
		},
		Planning: &PlanningState{NextOrder: []ProjectTaskLocator{{TaskID: "t", ProjectID: "inbox"}}},
		Machine:  &MachineState{CustomContexts: map[string][]string{"work": {"inbox"}}},
	}
	if changed, err := d.RemoveProject("inbox", "inbox"); changed || !errors.Is(err, ErrProtectedSystemInbox) {
		t.Fatalf("RemoveProject(system Inbox) = (%v, %v), want false and ErrProtectedSystemInbox", changed, err)
	}
	if len(d.Catalog.Projects) != 1 || len(d.Planning.NextOrder) != 1 ||
		len(d.Machine.CustomContexts["work"]) != 1 {
		t.Fatalf("protected removal partially mutated domains: %+v", d)
	}
}
