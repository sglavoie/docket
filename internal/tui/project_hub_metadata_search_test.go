package tui

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/route"
)

// Metadata-search fixture identities. The Hub is searched by these exact words,
// so no project word may appear in a task and no task word in a project: the
// whole point of the negative half is that the two vocabularies never overlap.
const (
	hubMetaLedgerDescription  = "Reconcile the quarterly settlement ledger."
	hubMetaHarborDescription  = "Migrate the telemetry pipeline onto the new broker."
	hubMetaRetiredDescription = "Shelved the kiosk pilot after the field trial."
)

// projectHubMetadataApp builds three projects whose descriptions and project
// tags are the only place their distinguishing words appear, plus tasks whose
// titles, descriptions, tags, contexts, and comments carry words nothing else
// does. One project is archived and one is pinned, so the same query can be put
// to the active, archived, and pinned populations without rebuilding the shell.
func projectHubMetadataApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)

	ledger := scopeTestProject("ledger", "Sierra One")
	ledger.Path = t.TempDir()
	ledger.Description = hubMetaLedgerDescription
	ledger.Tags = []string{"finance", "reporting"}
	ledger.Tasks = []*models.Task{metadataFixtureTask()}
	ledger.BuildTaskTree()

	harbor := scopeTestProject("harbor", "Sierra Two")
	harbor.Path = t.TempDir()
	harbor.Description = hubMetaHarborDescription
	harbor.Tags = []string{"infrastructure"}
	harbor.Pinned = true

	retired := scopeTestProject("retired", "Sierra Three")
	retired.Path = t.TempDir()
	retired.Description = hubMetaRetiredDescription
	retired.Tags = []string{"hardware"}
	retired.Archived = true

	setTestAppProjects(app, ledger, harbor, retired)
	app.currentProject = nil
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.loadedProjects = []*models.Project{ledger, harbor}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

// metadataFixtureTask wears every task-owned text channel at once, so a single
// project is enough to prove that none of them reaches the Hub's index.
func metadataFixtureTask() *models.Task {
	return &models.Task{
		ID:            "ledger-1",
		ShortID:       1,
		Title:         "Countersign the escrow packet",
		Description:   "Notarize the addendum before the courier collects it.",
		WorkflowState: models.StateBacklog,
		Tags:          []string{"courier"},
		Context:       "notary",
		Comments:      []*models.Comment{{Text: "Blocked on the registrar", CreatedAt: time.Now()}},
	}
}

// projectHubMatchIDs runs one query through the live surface and reports the
// rows it kept, in match order.
func projectHubMatchIDs(hub *ProjectHubModel, query string) []listsurface.ListItemID {
	hub.SetSearchQuery(query)
	hub.applyProjectFilter()
	matches := hub.surface.Matches()
	ids := make([]listsurface.ListItemID, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.Item.ID)
	}
	return ids
}

func assertProjectHubMatches(t *testing.T, hub *ProjectHubModel, query string, want ...listsurface.ListItemID) {
	t.Helper()
	got := projectHubMatchIDs(hub, query)
	sorted := append([]listsurface.ListItemID(nil), got...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	expected := append([]listsurface.ListItemID(nil), want...)
	sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })
	if len(sorted) != len(expected) {
		t.Fatalf("query %q matched %v, want %v", query, got, want)
	}
	for i := range sorted {
		if sorted[i] != expected[i] {
			t.Fatalf("query %q matched %v, want %v", query, got, want)
		}
	}
}

// TestProjectHubSearchMatchesDescriptionsAndProjectTags puts the same three
// query shapes — a description word, a bare tag, and the `#`-prefixed tag the
// row's own chips are written in — to the active and archived populations.
func TestProjectHubSearchMatchesDescriptionsAndProjectTags(t *testing.T) {
	hub := openProjectHub(t, projectHubMetadataApp(t))

	assertProjectHubMatches(t, hub, "settlement ledger", "project-hub:project:ledger")
	assertProjectHubMatches(t, hub, "telemetry broker", "project-hub:project:harbor")
	assertProjectHubMatches(t, hub, "kiosk pilot", "project-hub:archived:retired")

	assertProjectHubMatches(t, hub, "reporting", "project-hub:project:ledger")
	assertProjectHubMatches(t, hub, "#reporting", "project-hub:project:ledger")
	assertProjectHubMatches(t, hub, "infrastructure", "project-hub:project:harbor")
	assertProjectHubMatches(t, hub, "#infrastructure", "project-hub:project:harbor")
	assertProjectHubMatches(t, hub, "hardware", "project-hub:archived:retired")
	assertProjectHubMatches(t, hub, "#hardware", "project-hub:archived:retired")
}

// TestPinnedProjectHubSearchMatchesProjectMetadata proves the pinned mode reads
// the same alias vocabulary rather than a narrowed one: the pinned project
// answers its description and both tag spellings, and the unpinned projects stay
// outside the population no matter which of their words is typed.
func TestPinnedProjectHubSearchMatchesProjectMetadata(t *testing.T) {
	hub := openProjectHub(t, projectHubMetadataApp(t), route.ProjectHubModePinned)

	for _, query := range []string{"telemetry broker", "infrastructure", "#infrastructure"} {
		assertProjectHubMatches(t, hub, query, "project-hub:project:harbor")
	}
	for _, query := range []string{"settlement ledger", "reporting", "#reporting", "kiosk pilot", "#hardware"} {
		assertProjectHubMatches(t, hub, query)
	}
}

// TestProjectHubSearchNeverMatchesTaskContent is the boundary the metadata
// aliases must not cross. Every word here belongs to a task the Hub's own
// project holds, and the Hub still reports no match: Select Project chooses a
// project, and task text is the Outliner's `/` to search.
func TestProjectHubSearchNeverMatchesTaskContent(t *testing.T) {
	app := projectHubMetadataApp(t)
	for _, mode := range []route.ProjectHubMode{route.ProjectHubModeAll, route.ProjectHubModePinned} {
		hub := openProjectHub(t, app, mode)
		for _, query := range []string{
			"escrow packet",    // task title
			"courier collects", // task description
			"courier",          // task tag
			"notary",           // task context
			"registrar",        // task comment
		} {
			assertProjectHubMatches(t, hub, query)
		}
	}
}

// TestProjectHubAliasesCarryProjectMetadataOnly reads the constructed row rather
// than the search result, so the alias vocabulary itself is pinned: the name,
// the slug, the lifecycle phrase, the description, and each tag twice.
func TestProjectHubAliasesCarryProjectMetadataOnly(t *testing.T) {
	hub := openProjectHub(t, projectHubMetadataApp(t))

	active := projectHubItem(t, hub, "project-hub:project:ledger").Aliases
	want := []string{
		"Sierra One",
		"ledger",
		"active project",
		hubMetaLedgerDescription,
		"finance",
		"#finance",
		"reporting",
		"#reporting",
	}
	if strings.Join(active, "|") != strings.Join(want, "|") {
		t.Fatalf("active aliases = %v, want %v", active, want)
	}

	archived := projectHubItem(t, hub, "project-hub:archived:retired").Aliases
	if len(archived) < 3 || archived[2] != "archived project" {
		t.Fatalf("archived aliases = %v, want the archived lifecycle phrase", archived)
	}
	for _, alias := range archived {
		if strings.Contains(alias, "escrow") || strings.Contains(alias, "courier") {
			t.Fatalf("archived aliases leaked task content: %v", archived)
		}
	}
}

// TestProjectHubAliasesSkipAnEmptyDescription keeps a blank description out of
// the index entirely rather than joining an empty string into it, which would
// leave a stray separator in the haystack every query is scored against.
func TestProjectHubAliasesSkipAnEmptyDescription(t *testing.T) {
	project := scopeTestProject("plain", "Plain")
	project.Description = "   \n\t "
	got := projectHubAliases(project, "active project")
	want := []string{"Plain", "plain", "active project"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("aliases = %v, want %v", got, want)
	}
}
