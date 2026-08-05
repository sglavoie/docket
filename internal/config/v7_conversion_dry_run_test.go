package config

import (
	"slices"
	"strings"
	"testing"
)

const v7SourceWorkspaceID = "11111111-1111-4111-8111-111111111117"

func v7SourceFixture() v7SourceCatalog {
	return v7SourceCatalog{
		SchemaVersion:          V6ConfigSchemaVersion,
		WorkspaceID:            v7SourceWorkspaceID,
		SystemUnfiledProjectID: "project-unfiled",
		Boundaries: []v7SourceScope{
			{ID: "active", Name: "Active"},
			{ID: "scope-shelf", Name: "On The Shelf"},
			{ID: "scope-parked", Name: "Parked"},
			{ID: "scope-punct", Name: "#"},
			{ID: "scope-messy", Name: "Deep, Work;Now"},
		},
	}
}

func v7SourceProjectFixture(id, slug, context, scope string) v7SourceProject {
	return v7SourceProject{ID: id, Slug: slug, Name: strings.ToTitle(slug), Context: context, BoundaryID: scope}
}

func convertV7Fixture(t *testing.T, projects ...v7SourceProject) (WorkspaceCatalog, []V7CatalogMigrationAuditEntry) {
	t.Helper()
	source := v7SourceFixture()
	source.Projects = projects
	catalog, audit, err := convertV7CatalogCandidate(source)
	if err != nil {
		t.Fatalf("convert v7 candidate: %v", err)
	}
	return catalog, audit
}

func v7ProjectTags(t *testing.T, catalog WorkspaceCatalog, id string) []string {
	t.Helper()
	for _, project := range catalog.Projects {
		if project.ID == id {
			return project.Tags
		}
	}
	t.Fatalf("converted catalog has no project %q", id)
	return nil
}

func v7AuditActions(audit []V7CatalogMigrationAuditEntry, projectID, field string) []string {
	var actions []string
	for _, entry := range audit {
		if entry.ProjectID == projectID && entry.Field == field {
			actions = append(actions, entry.Action)
		}
	}
	return actions
}

func TestConvertV7CatalogMapsRetiredScopePairOntoProjectTags(t *testing.T) {
	catalog, _ := convertV7Fixture(t,
		v7SourceProjectFixture("project-unfiled", "unfiled", "", ""),
		v7SourceProjectFixture("project-alpha", "alpha", "Work", "active"),
		v7SourceProjectFixture("project-beta", "beta", "Personal", "scope-shelf"),
	)
	if catalog.SchemaVersion != V7ConfigSchemaVersion {
		t.Fatalf("converted schema = %d, want %d", catalog.SchemaVersion, V7ConfigSchemaVersion)
	}
	if got := v7ProjectTags(t, catalog, "project-unfiled"); len(got) != 0 {
		t.Errorf("Unfiled tags = %v, want none", got)
	}
	if got := v7ProjectTags(t, catalog, "project-alpha"); !slices.Equal(got, []string{"work"}) {
		t.Errorf("unparked project tags = %v, want [work]", got)
	}
	if got := v7ProjectTags(t, catalog, "project-beta"); !slices.Equal(got, []string{"on-the-shelf", "parked", "personal"}) {
		t.Errorf("parked project tags = %v, want the scope name, parked, and the context", got)
	}
}

func TestConvertV7CatalogRefusesToParkTheProtectedUnfiledProject(t *testing.T) {
	catalog, audit := convertV7Fixture(t,
		v7SourceProjectFixture("project-unfiled", "unfiled", "Work", "scope-shelf"),
	)
	tags := v7ProjectTags(t, catalog, "project-unfiled")
	if slices.Contains(tags, "parked") {
		t.Fatalf("protected Unfiled tags = %v, want no parked tag", tags)
	}
	if !slices.Equal(tags, []string{"work"}) {
		t.Errorf("protected Unfiled tags = %v, want its context retained", tags)
	}
	if got := v7AuditActions(audit, "project-unfiled", "scope_reference"); !slices.Equal(got, []string{"dropped"}) {
		t.Errorf("protected Unfiled scope audit = %v, want one dropped entry", got)
	}
}

func TestConvertV7CatalogSeparatesAnUnparkedNameReadingParked(t *testing.T) {
	catalog, audit := convertV7Fixture(t,
		v7SourceProjectFixture("project-unfiled", "unfiled", "", ""),
		v7SourceProjectFixture("project-alpha", "alpha", "parked", "active"),
		v7SourceProjectFixture("project-beta", "beta", "", "scope-parked"),
	)
	if got := v7ProjectTags(t, catalog, "project-alpha"); !slices.Equal(got, []string{"parked-2"}) {
		t.Errorf("unparked project reading parked = %v, want the renamed alias", got)
	}
	if got := v7ProjectTags(t, catalog, "project-beta"); !slices.Equal(got, []string{"parked"}) {
		t.Errorf("parked project named Parked = %v, want one merged parked tag", got)
	}
	if got := v7AuditActions(audit, "project-alpha", "context"); !slices.Equal(got, []string{"renamed"}) {
		t.Errorf("alias audit = %v, want one renamed entry", got)
	}
	if got := v7AuditActions(audit, "project-beta", "scope_name"); !slices.Equal(got, []string{"merged"}) {
		t.Errorf("merge audit = %v, want one merged entry", got)
	}
}

func TestConvertV7CatalogSanitizesAndDropsUnusableRetiredNames(t *testing.T) {
	catalog, audit := convertV7Fixture(t,
		v7SourceProjectFixture("project-unfiled", "unfiled", "", ""),
		v7SourceProjectFixture("project-alpha", "alpha", "", "scope-messy"),
		v7SourceProjectFixture("project-beta", "beta", "  ", "scope-punct"),
	)
	if got := v7ProjectTags(t, catalog, "project-alpha"); !slices.Equal(got, []string{"deep-work-now", "parked"}) {
		t.Errorf("sanitized scope tags = %v, want the rewritten name plus parked", got)
	}
	if got := v7ProjectTags(t, catalog, "project-beta"); !slices.Equal(got, []string{"parked"}) {
		t.Errorf("unusable scope tags = %v, want parked alone", got)
	}
	if got := v7AuditActions(audit, "project-alpha", "scope_name"); !slices.Equal(got, []string{"sanitized"}) {
		t.Errorf("sanitization audit = %v, want one sanitized entry", got)
	}
	if got := v7AuditActions(audit, "project-beta", "scope_name"); !slices.Equal(got, []string{"dropped"}) {
		t.Errorf("unusable-name audit = %v, want one dropped entry", got)
	}
}

func TestConvertV7CatalogFoldsAMatchingContextAndScopeNameOntoOneTag(t *testing.T) {
	catalog, audit := convertV7Fixture(t,
		v7SourceProjectFixture("project-unfiled", "unfiled", "", ""),
		v7SourceProjectFixture("project-alpha", "alpha", "on the shelf", "scope-shelf"),
	)
	if got := v7ProjectTags(t, catalog, "project-alpha"); !slices.Equal(got, []string{"on-the-shelf", "parked"}) {
		t.Errorf("folded tags = %v, want one shared tag plus parked", got)
	}
	if got := v7AuditActions(audit, "project-alpha", "tags"); !slices.Equal(got, []string{"deduped"}) {
		t.Errorf("fold audit = %v, want one deduped entry", got)
	}
}

func TestConvertV7CatalogKeepsAnArchivedParkedProjectParked(t *testing.T) {
	project := v7SourceProjectFixture("project-alpha", "alpha", "", "scope-shelf")
	project.Archived = true
	catalog, audit := convertV7Fixture(t,
		v7SourceProjectFixture("project-unfiled", "unfiled", "", ""),
		project,
	)
	if got := v7ProjectTags(t, catalog, "project-alpha"); !slices.Contains(got, "parked") {
		t.Errorf("archived parked tags = %v, want the parked tag retained", got)
	}
	if got := v7AuditActions(audit, "project-alpha", "tags"); !slices.Equal(got, []string{"retained"}) {
		t.Errorf("archived parked audit = %v, want one retained entry", got)
	}
}

func TestConvertV7CatalogRejectsAnUndefinedScopeReference(t *testing.T) {
	source := v7SourceFixture()
	source.Projects = []v7SourceProject{
		v7SourceProjectFixture("project-unfiled", "unfiled", "", ""),
		v7SourceProjectFixture("project-alpha", "alpha", "", "scope-missing"),
	}
	_, _, err := convertV7CatalogCandidate(source)
	if err == nil || !strings.Contains(err.Error(), "undefined retired scope") {
		t.Fatalf("undefined scope reference error = %v", err)
	}
}

func TestConvertV7CatalogDiscardsEveryRetiredScopeDefinition(t *testing.T) {
	_, audit := convertV7Fixture(t, v7SourceProjectFixture("project-unfiled", "unfiled", "", ""))
	dropped := 0
	for _, entry := range audit {
		if entry.Field == "scope_definition" && entry.Action == "dropped" {
			dropped++
		}
	}
	if want := len(v7SourceFixture().Boundaries); dropped != want {
		t.Fatalf("dropped scope definitions = %d, want %d", dropped, want)
	}
}
