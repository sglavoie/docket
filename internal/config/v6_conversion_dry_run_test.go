package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/storage/format"
)

//nolint:gocyclo // One end-to-end fixture keeps the full candidate contract comparable.
func TestDryRunV6WorkspaceConversionBuildsCompleteDeterministicCandidate(t *testing.T) {
	options := v6ConsolidationFixtureOptions(t)
	before := inventoryV5DryRunSources(t, options.DataDir, options.MachineStatePath)

	first, err := DryRunV6WorkspaceConversion(options)
	if err != nil {
		t.Fatalf("DryRunV6WorkspaceConversion: %v", err)
	}
	second, err := DryRunV6WorkspaceConversion(options)
	if err != nil {
		t.Fatalf("second DryRunV6WorkspaceConversion: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("unchanged schema-5 workspace produced a different second schema-6 candidate")
	}
	after := inventoryV5DryRunSources(t, options.DataDir, options.MachineStatePath)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("schema-6 dry run changed source inventory\nbefore=%v\nafter=%v", before, after)
	}

	catalog := first.Domains.Domains.Catalog
	if catalog.SchemaVersion != V6ConfigSchemaVersion ||
		catalog.SystemUnfiledProjectID != "project-inbox" {
		t.Fatalf("v6 catalog identity = %+v", catalog)
	}
	protected := findV6CatalogProject(t, catalog, catalog.SystemUnfiledProjectID)
	if protected.Name != V6UnfiledProjectName || protected.Slug != V6UnfiledProjectSlug {
		t.Fatalf("protected v6 project = %+v", protected)
	}
	wantContexts := map[string]string{
		"project-inbox":   "personal",
		"project-alpha":   "work",
		"project-beta":    "personal",
		"project-archive": "archive",
	}
	for _, project := range catalog.Projects {
		if project.Context != wantContexts[project.ID] {
			t.Errorf("project %s context = %q, want %q", project.ID, project.Context, wantContexts[project.ID])
		}
	}
	if got := first.Domains.Domains.Machine.Theme; got != "light" {
		t.Fatalf("v6 machine theme = %q, want light", got)
	}

	wantToday := []V6PlanningTaskRef{{ProjectID: "project-alpha", TaskID: "task-rich"}}
	wantNext := []V6PlanningTaskRef{
		{ProjectID: "project-alpha", TaskID: "task-next"},
		{ProjectID: "project-beta", TaskID: "task-next-unordered"},
	}
	planning := first.Domains.Domains.Planning
	if !slices.Equal(planning.Today, wantToday) || !slices.Equal(planning.Next, wantNext) {
		t.Fatalf("v6 planning = Today:%+v Next:%+v", planning.Today, planning.Next)
	}
	for _, want := range []struct {
		action string
		field  string
		id     string
	}{
		{action: "removed", field: "today", id: "task-next"},
		{action: "removed", field: "today", id: "task-done"},
		{action: "removed", field: "today", id: "task-missing"},
		{action: "appended", field: "next", id: "task-next-unordered"},
	} {
		if !hasV6DomainAudit(first.Domains.Audit, "planning", want.action, want.field, want.id) {
			t.Errorf("missing planning audit %+v", want)
		}
	}
	for _, context := range []struct {
		name  string
		count string
	}{{name: "Deep Work", count: "2 members"}, {name: "Personal", count: "1 members"}} {
		if !slices.ContainsFunc(first.Domains.Audit, func(entry V6DomainMigrationAuditEntry) bool {
			return entry.Domain == "machine" && entry.Field == "custom_context" &&
				entry.EntityID == context.name && entry.From == context.count
		}) {
			t.Errorf("missing custom-context audit for %q", context.name)
		}
	}

	if len(first.Projects) != len(catalog.Projects) {
		t.Fatalf("v6 project candidates = %d, want %d", len(first.Projects), len(catalog.Projects))
	}
	archived := findV6ProjectDocumentCandidate(t, first.Projects, "project-archive")
	if archived.SourceExists || len(archived.Document.Tasks) != 0 {
		t.Fatalf("archived empty project candidate = %+v", archived)
	}
	unfiled := findV6ProjectDocumentCandidate(t, first.Projects, "project-inbox")
	if unfiled.Document.ProjectName != V6UnfiledProjectName ||
		filepath.Base(filepath.Dir(unfiled.SourcePath)) != "inbox" ||
		filepath.Base(filepath.Dir(unfiled.Path)) != V6UnfiledProjectSlug {
		t.Fatalf("Unfiled document paths/name = %+v", unfiled)
	}
	alpha := findV6ProjectDocumentCandidate(t, first.Projects, "project-alpha")
	if state := findV6TaskState(alpha, "task-next"); state != "backlog" {
		t.Fatalf("converted StateBacklog lifecycle = %q, want backlog", state)
	}

	noteCount := 0
	for _, artifact := range first.Artifacts {
		if strings.Contains(artifact.Kind, "note") {
			noteCount++
		}
	}
	if noteCount != 2 {
		t.Fatalf("retained note artifacts = %d, want 2", noteCount)
	}
	if len(first.Attachments) != 1 {
		t.Fatalf("retained attachments = %+v, want one local file", first.Attachments)
	}
	attachment := first.Attachments[0]
	if attachment.ProjectID != "project-alpha" || attachment.TaskID != "task-rich" ||
		attachment.Filename != "specification.txt" ||
		filepath.Base(filepath.Dir(attachment.SourcePath)) != "task-rich" ||
		filepath.Base(filepath.Dir(attachment.Path)) != v6AttachmentTaskDirName("task-rich") ||
		attachment.Size == 0 || len(attachment.SHA256) != 64 {
		t.Fatalf("retained attachment candidate = %+v", attachment)
	}
	if err := ValidateV6WorkspaceConversionPlan(first); err != nil {
		t.Fatalf("ValidateV6WorkspaceConversionPlan: %v", err)
	}
}

func TestDryRunV6WorkspaceConversionRejectsUnfiledCollisions(t *testing.T) {
	t.Run("filesystem", func(t *testing.T) {
		options := copyV6ConsolidationFixture(t)
		collision := filepath.Join(options.DataDir, "projects", V6UnfiledProjectSlug)
		if err := os.MkdirAll(collision, DirPermissions); err != nil {
			t.Fatal(err)
		}
		if _, err := DryRunV6WorkspaceConversion(options); err == nil || !strings.Contains(err.Error(), "filesystem target already exists") {
			t.Fatalf("filesystem collision error = %v", err)
		}
	})

	t.Run("catalog", func(t *testing.T) {
		options := copyV6ConsolidationFixture(t)
		catalog, workspaceID, err := readV5CatalogCandidate(WorkspaceCatalogPath(options.DataDir))
		if err != nil {
			t.Fatal(err)
		}
		project := catalog.Projects[1]
		project.ID = "project-unfiled-collision"
		project.Name = "Existing Unfiled"
		project.Slug = V6UnfiledProjectSlug
		catalog.Projects = append(catalog.Projects, project)
		data, err := encodeV5DomainEnvelope("workspace", workspaceID, catalog)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(WorkspaceCatalogPath(options.DataDir), data, FilePermissions); err != nil {
			t.Fatal(err)
		}
		if _, err := DryRunV6WorkspaceConversion(options); err == nil || !strings.Contains(err.Error(), "already owned") {
			t.Fatalf("catalog collision error = %v", err)
		}
	})
}

func TestDryRunV6WorkspaceConversionRejectsBrokenRetainedTopology(t *testing.T) {
	options := copyV6ConsolidationFixture(t)
	attachment := filepath.Join(
		options.DataDir, "projects", "alpha", ".attachments", "task-rich", "specification.txt",
	)
	if err := os.Remove(attachment); err != nil {
		t.Fatal(err)
	}
	if _, err := DryRunV6WorkspaceConversion(options); err == nil || !strings.Contains(err.Error(), "local attachment \"specification.txt\" is missing") {
		t.Fatalf("missing attachment error = %v", err)
	}
}

func TestValidateV6WorkspaceConversionPlanRejectsPlanningAndHierarchyCorruption(t *testing.T) {
	plan, err := DryRunV6WorkspaceConversion(v6ConsolidationFixtureOptions(t))
	if err != nil {
		t.Fatal(err)
	}

	overlap := plan
	overlap.Domains.Domains.Planning.Today = append(
		append([]V6PlanningTaskRef(nil), plan.Domains.Domains.Planning.Today...),
		plan.Domains.Domains.Planning.Next[0],
	)
	if err := ValidateV6WorkspaceConversionPlan(overlap); err == nil || !strings.Contains(err.Error(), "belongs to Today and Next") {
		t.Fatalf("planning overlap error = %v", err)
	}

	brokenHierarchy := plan
	brokenHierarchy.Projects = append([]V6ProjectDocumentCandidate(nil), plan.Projects...)
	alphaIndex := slices.IndexFunc(brokenHierarchy.Projects, func(project V6ProjectDocumentCandidate) bool {
		return project.ProjectID == "project-alpha"
	})
	brokenHierarchy.Projects[alphaIndex].Document.Tasks = append(
		[]format.V6TaskRecord(nil),
		brokenHierarchy.Projects[alphaIndex].Document.Tasks...,
	)
	brokenHierarchy.Projects[alphaIndex].Document.Tasks[0].ParentID = "missing-parent"
	if err := ValidateV6WorkspaceConversionPlan(brokenHierarchy); err == nil || !strings.Contains(err.Error(), "missing project-local parent") {
		t.Fatalf("hierarchy error = %v", err)
	}
}

func v6ConsolidationFixtureOptions(t *testing.T) V6WorkspaceDryRunOptions {
	t.Helper()
	root, err := filepath.Abs(filepath.Join(v5ConsolidationConfigFixtureRoot, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	machinePath, err := filepath.Abs(filepath.Join(
		v5ConsolidationConfigFixtureRoot,
		"machine", "runtime", v5ConsolidationWorkspaceID, "machine.json",
	))
	if err != nil {
		t.Fatal(err)
	}
	return V6WorkspaceDryRunOptions{DataDir: root, MachineStatePath: machinePath}
}

func copyV6ConsolidationFixture(t *testing.T) V6WorkspaceDryRunOptions {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.CopyFS(root, os.DirFS(filepath.Join(v5ConsolidationConfigFixtureRoot, "workspace"))); err != nil {
		t.Fatalf("copy v5 consolidation fixture: %v", err)
	}
	options := v6ConsolidationFixtureOptions(t)
	options.DataDir = root
	return options
}

func findV6CatalogProject(
	t *testing.T,
	catalog V6WorkspaceCatalogCandidate,
	projectID string,
) V6CatalogProjectCandidate {
	t.Helper()
	index := slices.IndexFunc(catalog.Projects, func(project V6CatalogProjectCandidate) bool {
		return project.ID == projectID
	})
	if index < 0 {
		t.Fatalf("v6 catalog project %q not found", projectID)
	}
	return catalog.Projects[index]
}

func findV6ProjectDocumentCandidate(
	t *testing.T,
	projects []V6ProjectDocumentCandidate,
	projectID string,
) V6ProjectDocumentCandidate {
	t.Helper()
	index := slices.IndexFunc(projects, func(project V6ProjectDocumentCandidate) bool {
		return project.ProjectID == projectID
	})
	if index < 0 {
		t.Fatalf("v6 document project %q not found", projectID)
	}
	return projects[index]
}

func findV6TaskState(project V6ProjectDocumentCandidate, taskID string) string {
	for _, task := range project.Document.Tasks {
		if task.ID == taskID {
			return task.State
		}
	}
	return ""
}

func hasV6DomainAudit(
	entries []V6DomainMigrationAuditEntry,
	domain string,
	action string,
	field string,
	entityID string,
) bool {
	return slices.ContainsFunc(entries, func(entry V6DomainMigrationAuditEntry) bool {
		return entry.Domain == domain && entry.Action == action &&
			entry.Field == field && entry.EntityID == entityID
	})
}
