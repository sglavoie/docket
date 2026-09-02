package config

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewPlanningStateDefaults(t *testing.T) {
	p := NewPlanningState()
	if err := p.Validate(); err != nil {
		t.Errorf("default planning should validate: %v", err)
	}
}

func TestPlanningStateValidate(t *testing.T) {
	valid := &PlanningState{
		Next:  []ProjectTaskLocator{{TaskID: "t", ProjectID: "p"}},
		Today: []ProjectTaskLocator{{TaskID: "t", ProjectID: "p"}},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid planning should pass: %v", err)
	}

	cases := []*PlanningState{
		{Next: []ProjectTaskLocator{{ProjectID: "p"}}},
		{Today: []ProjectTaskLocator{{ProjectID: "p"}}},
		{Next: []ProjectTaskLocator{{TaskID: "t", ProjectID: "p"}, {TaskID: "t", ProjectID: "p"}}},
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}

func TestPlanningStateRemoveProjectReferences(t *testing.T) {
	p := &PlanningState{
		Next:  []ProjectTaskLocator{{TaskID: "t1", ProjectID: "keep"}, {TaskID: "t2", ProjectID: "drop"}},
		Today: []ProjectTaskLocator{{TaskID: "t3", ProjectID: "drop-slug"}},
	}
	if !p.RemoveProjectReferences("drop", "drop-slug") {
		t.Fatal("expected change")
	}
	if len(p.Next) != 1 || p.Next[0].ProjectID != "keep" {
		t.Errorf("focus queue not cleaned: %+v", p.Next)
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
	for _, field := range []string{"start_view", "start_route", "home_projects_height_pct", "sidebar_enabled", "custom_contexts"} {
		if strings.Contains(string(data), field) {
			t.Fatalf("machine state should omit retired field %q, got %s", field, data)
		}
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
		Catalog:  &WorkspaceCatalog{SchemaVersion: 3, Projects: []CatalogProject{{ID: "keep", Slug: "keep"}, {ID: "drop", Slug: "drop"}}, SystemUnfiledProjectID: "keep"},
		Planning: &PlanningState{Next: []ProjectTaskLocator{{TaskID: "t", ProjectID: "drop"}}},
		Machine:  NewMachineState(),
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
	if len(d.Planning.Next) != 0 {
		t.Error("planning focus queue not cleaned")
	}
}

func TestDomainsProtectedInboxRemovalIsAtomic(t *testing.T) {
	d := &Domains{
		Catalog: &WorkspaceCatalog{
			SchemaVersion:          CurrentConfigVersion,
			Projects:               []CatalogProject{{ID: "inbox", Slug: "inbox", Name: "Inbox"}},
			SystemUnfiledProjectID: "inbox",
		},
		Planning: &PlanningState{Next: []ProjectTaskLocator{{TaskID: "t", ProjectID: "inbox"}}},
		Machine:  NewMachineState(),
	}
	if changed, err := d.RemoveProject("inbox", "inbox"); changed || !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("RemoveProject(system Inbox) = (%v, %v), want false and ErrProtectedSystemUnfiled", changed, err)
	}
	if len(d.Catalog.Projects) != 1 || len(d.Planning.Next) != 1 {
		t.Fatalf("protected removal partially mutated domains: %+v", d)
	}
}
