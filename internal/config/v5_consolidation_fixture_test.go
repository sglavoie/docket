package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

const (
	v5ConsolidationConfigFixtureRoot = "../storage/testdata/v5_consolidation"
	v5ConsolidationWorkspaceID       = "11111111-1111-4111-8111-111111111115"
)

func TestV5ConsolidationDomainFixturesCoverProtectedIdentityAndContexts(t *testing.T) {
	var catalog V5WorkspaceCatalogCandidate
	readV5ConsolidationDomain(t, "workspace/.tt/workspace.json", "workspace", &catalog)
	if err := catalog.Validate(); err != nil {
		t.Fatalf("validate workspace fixture: %v", err)
	}
	if catalog.SchemaVersion != V5ConfigSchemaVersion || catalog.WorkspaceID != v5ConsolidationWorkspaceID {
		t.Fatalf("workspace fixture identity = schema %d workspace %q", catalog.SchemaVersion, catalog.WorkspaceID)
	}
	if catalog.SystemInboxProjectID != "project-inbox" {
		t.Fatalf("protected Inbox ID = %q, want project-inbox", catalog.SystemInboxProjectID)
	}

	contexts := make(map[string]bool)
	var protected *V5ProjectCandidate
	for i := range catalog.Projects {
		project := &catalog.Projects[i]
		contexts[project.Context] = true
		if project.ID == catalog.SystemInboxProjectID {
			protected = project
		}
	}
	if protected == nil || protected.Name != "Inbox" || protected.Slug != "inbox" {
		t.Fatalf("protected Inbox project = %+v", protected)
	}
	for _, context := range []string{"personal", "work", "archive"} {
		if !contexts[context] {
			t.Errorf("workspace fixture is missing regular project context %q", context)
		}
	}

	var machine V5MachineCandidate
	readV5ConsolidationDomain(
		t,
		"machine/runtime/"+v5ConsolidationWorkspaceID+"/machine.json",
		"machine",
		&machine,
	)
	if err := machine.Validate(catalog); err != nil {
		t.Fatalf("validate machine fixture: %v", err)
	}
	wantContexts := map[string][]string{
		"Deep Work": {"project-alpha", "project-beta"},
		"Personal":  {"project-inbox"},
	}
	if machine.Theme != "light" || !reflect.DeepEqual(machine.CustomContexts, wantContexts) {
		t.Fatalf("machine fixture = %+v, want theme light and contexts %+v", machine, wantContexts)
	}
}

func TestV5ConsolidationPlanningFixtureCharacterizesContradictions(t *testing.T) {
	var catalog V5WorkspaceCatalogCandidate
	readV5ConsolidationDomain(t, "workspace/.tt/workspace.json", "workspace", &catalog)
	var planning V5PlanningCandidate
	readV5ConsolidationDomain(t, "workspace/.tt/planning.json", "planning", &planning)
	if err := planning.Validate(catalog); err != nil {
		t.Fatalf("schema-5 planning fixture must remain structurally valid: %v", err)
	}

	wantToday := []V5PlanningTaskRef{
		{TaskID: "task-rich", ProjectID: "project-alpha"},
		{TaskID: "task-next", ProjectID: "project-alpha"},
		{TaskID: "task-done", ProjectID: "project-alpha"},
		{TaskID: "task-missing", ProjectID: "project-alpha"},
	}
	wantNextOrder := []V5PlanningTaskRef{
		{TaskID: "task-rich", ProjectID: "project-alpha"},
		{TaskID: "task-doing", ProjectID: "project-alpha"},
		{TaskID: "task-canceled", ProjectID: "project-alpha"},
		{TaskID: "task-next", ProjectID: "project-alpha"},
	}
	if !slices.Equal(planning.Today, wantToday) || !slices.Equal(planning.NextOrder, wantNextOrder) {
		t.Fatalf("planning fixture changed:\nToday: %+v\nNextOrder: %+v", planning.Today, planning.NextOrder)
	}

	states := readV5ConsolidationTaskStates(t)
	if states["project-alpha/task-next"] != "next" || states["project-alpha/task-done"] != "done" {
		t.Fatalf("planning fixture lost Next/terminal source tasks: %+v", states)
	}
	if _, exists := states["project-alpha/task-missing"]; exists {
		t.Fatal("dangling Today fixture unexpectedly resolves to a task")
	}
	if states["project-beta/task-next-unordered"] != "next" {
		t.Fatal("fixture is missing a StateBacklog task that must be appended after the saved order")
	}
	if slices.ContainsFunc(planning.NextOrder, func(ref V5PlanningTaskRef) bool {
		return ref.ProjectID == "project-beta" && ref.TaskID == "task-next-unordered"
	}) {
		t.Fatal("unordered StateBacklog task unexpectedly appears in legacy NextOrder")
	}
	for _, ref := range []V5PlanningTaskRef{wantToday[0], wantToday[1]} {
		if !slices.Contains(planning.NextOrder, ref) {
			t.Errorf("fixture lost Today/NextOrder overlap for %+v", ref)
		}
	}
}

func readV5ConsolidationDomain(t *testing.T, rel, kind string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(v5ConsolidationConfigFixtureRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope domainEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("decode fixture envelope %s: %v", rel, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		t.Fatalf("decode fixture envelope %s: %v", rel, err)
	}
	if envelope.SchemaVersion != CurrentDomainSchemaVersion || envelope.WorkspaceID != v5ConsolidationWorkspaceID || envelope.Kind != kind {
		t.Fatalf("fixture envelope %s identity = schema %d workspace %q kind %q", rel, envelope.SchemaVersion, envelope.WorkspaceID, envelope.Kind)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		t.Fatalf("decode fixture payload %s: %v", rel, err)
	}
}

func readV5ConsolidationTaskStates(t *testing.T) map[string]string {
	t.Helper()
	states := make(map[string]string)
	for project, rel := range map[string]string{
		"project-alpha": "workspace/projects/alpha/tasks.md",
		"project-beta":  "workspace/projects/beta/tasks.md",
		"project-inbox": "workspace/projects/inbox/tasks.md",
	} {
		data, err := os.ReadFile(filepath.Join(v5ConsolidationConfigFixtureRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read task fixture %s: %v", rel, err)
		}
		document, err := storage.ParseV5ProjectDocument(string(data))
		if err != nil {
			t.Fatalf("parse task fixture %s: %v", rel, err)
		}
		for _, task := range document.Tasks {
			states[project+"/"+task.ID] = task.State
		}
	}
	return states
}
