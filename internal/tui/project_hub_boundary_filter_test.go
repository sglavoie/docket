package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// The Project Hub is the one surface that must show a parked project, so these
// tests pin the three properties that make one findable: it is grouped under
// its boundary, it is reachable by boundary name as well as project name, and
// Enter still focuses it. The boundary filter is a listing narrowing only —
// every test that applies one also asserts stored membership is untouched.

// boundaryHubApp extends the two-project fixture with a parked pair and an
// archived project that retains a boundary of its own. The lowercase
// "reference" against "Someday" is deliberate: byte ordering would reverse them.
func boundaryHubApp(t *testing.T) *AppModel {
	t.Helper()
	app := contextHubApp(t)

	gamma := scopeTestProject("gamma", "Gamma")
	gamma.Slug = "gamma"
	gamma.Context = "research"
	gamma.Pinned = true
	gamma.Path = t.TempDir()
	addTestAppProject(app, gamma)

	delta := scopeTestProject("delta", "Delta")
	delta.Slug = "delta"
	delta.Context = "work"
	delta.Archived = true
	delta.Path = t.TempDir()
	addTestAppProject(app, delta)

	parkProject(t, app.runtime, "reference", "beta", "gamma")
	parkProject(t, app.runtime, "Someday", "delta")
	return app
}

func boundaryIDNamed(t *testing.T, app *AppModel, name string) string {
	t.Helper()
	for _, boundary := range app.runtime.Catalog().Boundaries {
		if boundary.Name == name {
			return boundary.ID
		}
	}
	t.Fatalf("boundary %q is not in the catalog", name)
	return ""
}

func projectHubItemByID(t *testing.T, hub *ProjectHubModel, id ListItemID) ListItem {
	t.Helper()
	for _, item := range hub.surface.Items() {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Project Hub omitted %q", id)
	return ListItem{}
}

func projectHubCategoryLabels(hub *ProjectHubModel) []string {
	labels := make([]string, 0, len(hub.surface.Categories()))
	for _, category := range hub.surface.Categories() {
		labels = append(labels, category.Label)
	}
	return labels
}

func projectHubMatchIDs(hub *ProjectHubModel) []ListItemID {
	ids := make([]ListItemID, 0, len(hub.surface.Matches()))
	for _, match := range hub.surface.Matches() {
		ids = append(ids, match.Item.ID)
	}
	return ids
}

func previewFieldValue(t *testing.T, preview ListPreview, label string) string {
	t.Helper()
	for _, field := range preview.Fields {
		if field.Label == label {
			return field.Value
		}
	}
	t.Fatalf("preview %q has no %q field", preview.Title, label)
	return ""
}

// boundaryMembership snapshots stored boundary assignments so a listing change
// can be told apart from a mutation.
func boundaryMembership(app *AppModel) map[string]string {
	catalog := app.runtime.Catalog()
	membership := make(map[string]string, len(catalog.Projects))
	for _, project := range catalog.Projects {
		membership[project.ID] = catalog.ProjectBoundaryID(project.ID)
	}
	return membership
}

func TestProjectHubGroupsProjectsByBoundary(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)

	wantCategories := []string{"Active Projects", "reference", "Someday", "Archived Projects"}
	if got := projectHubCategoryLabels(hub); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("category labels = %#v, want %#v", got, wantCategories)
	}

	// Alpha alone remains in the base workspace; inside reference the pinned
	// Gamma precedes Beta; Delta stays in the archived tail.
	wantRows := []ListItemID{
		"project-hub:project:alpha",
		"project-hub:project:gamma",
		"project-hub:project:beta",
		"project-hub:archived:delta",
	}
	if got := projectHubMatchIDs(hub); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("rows = %#v, want %#v", got, wantRows)
	}

	reference := boundaryIDNamed(t, app, "reference")
	if got := projectHubItemByID(t, hub, "project-hub:project:gamma").CategoryID; got != projectHubBoundaryCategory(reference) {
		t.Fatalf("Gamma category = %q, want the reference boundary group", got)
	}
}

func TestProjectHubArchivedRowRetainsItsBoundary(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)
	item := projectHubItemByID(t, hub, "project-hub:archived:delta")

	if item.CategoryID != projectHubCategoryArchived {
		t.Fatalf("archived Delta category = %q, want the archived group", item.CategoryID)
	}
	if !strings.Contains(item.SecondaryText, "Someday") {
		t.Fatalf("archived row detail = %q, want the retained boundary", item.SecondaryText)
	}
	if !slices.Contains(item.Aliases, "Someday") {
		t.Fatalf("archived aliases = %#v, want the retained boundary", item.Aliases)
	}
	if got := previewFieldValue(t, item.Preview, "Boundary"); got != "Someday" {
		t.Fatalf("archived preview boundary = %q, want Someday", got)
	}
}

func TestProjectHubReportsUnloadedTaskCountsAsUnavailable(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)

	loaded := projectHubItemByID(t, hub, "project-hub:project:alpha")
	if got := previewFieldValue(t, loaded.Preview, "Tasks"); got != "2" {
		t.Fatalf("loaded Alpha task count = %q, want 2", got)
	}
	if !strings.Contains(loaded.SecondaryText, "2 tasks") {
		t.Fatalf("loaded Alpha row detail = %q, want a task count", loaded.SecondaryText)
	}

	for _, id := range []ListItemID{"project-hub:project:gamma", "project-hub:archived:delta"} {
		item := projectHubItemByID(t, hub, id)
		if got := previewFieldValue(t, item.Preview, "Tasks"); got != projectTaskCountUnknown {
			t.Errorf("%s task count = %q, want %q", id, got, projectTaskCountUnknown)
		}
	}
	if detail := projectHubItemByID(t, hub, "project-hub:project:gamma").SecondaryText; !strings.Contains(detail, "not loaded") {
		t.Fatalf("unloaded Gamma row detail = %q, want an unavailable count", detail)
	}
}

// openProjectHubFilter leaves the initial search and opens the boundary picker.
func openProjectHubFilter(t *testing.T, hub *ProjectHubModel) {
	t.Helper()
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	hub.Update(syntheticKey('f'))
	if hub.filterPicker == nil {
		t.Fatal("browse f did not open the boundary filter picker")
	}
}

// chooseProjectHubReferenceFilter searches the open picker for the "reference"
// boundary and applies the match.
func chooseProjectHubReferenceFilter(t *testing.T, hub *ProjectHubModel) {
	t.Helper()
	for _, r := range "reference" {
		hub.Update(syntheticKey(r))
	}
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if hub.filterPicker != nil {
		t.Fatal(`choosing "reference" left the picker open`)
	}
}

func TestProjectHubBoundaryFilterListsEveryGroupWithCounts(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)
	openProjectHubFilter(t, hub)

	type row struct {
		primary   string
		secondary string
	}
	var got []row
	for _, item := range hub.filterPicker.surface.Items() {
		got = append(got, row{item.PrimaryText, item.SecondaryText})
	}
	want := []row{
		{"All (current)", "4 projects"},
		{config.ActiveBoundaryName, "1 project"},
		{"reference", "2 projects"},
		{"Someday", "0 projects"},
		{"Archived", "1 project"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter options = %#v, want %#v", got, want)
	}
}

func TestProjectHubBoundaryFilterNarrowsWithoutMutatingMembership(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)
	before := boundaryMembership(app)

	openProjectHubFilter(t, hub)
	chooseProjectHubReferenceFilter(t, hub)

	if hub.boundaryFilter != boundaryIDNamed(t, app, "reference") {
		t.Fatalf("filter = %q, want the reference boundary ID", hub.boundaryFilter)
	}
	wantRows := []ListItemID{"project-hub:project:gamma", "project-hub:project:beta"}
	if got := projectHubMatchIDs(hub); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("filtered rows = %#v, want %#v", got, wantRows)
	}
	if subtitle := hub.subtitle(); !strings.Contains(subtitle, "reference") {
		t.Fatalf("subtitle = %q, want the active filter", subtitle)
	}
	if !reflect.DeepEqual(boundaryMembership(app), before) {
		t.Fatalf("boundary filtering changed stored membership: %#v", boundaryMembership(app))
	}
}

func TestProjectHubBoundaryFilterComposesWithSearchAndSurvivesIt(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)
	openProjectHubFilter(t, hub)
	chooseProjectHubReferenceFilter(t, hub)
	reference := hub.boundaryFilter

	hub.Update(syntheticKey('/'))
	for _, r := range "alpha" {
		hub.Update(syntheticKey(r))
	}
	if got := projectHubMatchIDs(hub); len(got) != 0 {
		t.Fatalf("search reached outside the filter: %#v", got)
	}

	for range "alpha" {
		hub.Update(syntheticSpecialKey(tea.KeyBackspace))
	}
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	wantRows := []ListItemID{"project-hub:project:beta"}
	if got := projectHubMatchIDs(hub); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("filtered search rows = %#v, want %#v", got, wantRows)
	}

	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() || hub.boundaryFilter != reference {
		t.Fatalf("leaving search dropped the filter: search=%v filter=%q", hub.HasActiveSearch(), hub.boundaryFilter)
	}
	if got := projectHubMatchIDs(hub); len(got) != 2 {
		t.Fatalf("rows after leaving search = %#v, want the two reference projects", got)
	}
}

func TestProjectHubBoundaryFilterResetsWhenReopened(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)
	openProjectHubFilter(t, hub)
	chooseProjectHubReferenceFilter(t, hub)
	if hub.boundaryFilter == projectHubFilterAll {
		t.Fatal("fixture did not apply a boundary filter")
	}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled {
		t.Fatal("browse Esc did not close the Project Hub")
	}
	reopened := openProjectHub(t, app)
	if reopened.boundaryFilter != projectHubFilterAll {
		t.Fatalf("reopened filter = %q, want All", reopened.boundaryFilter)
	}
	if len(reopened.surface.Matches()) != 4 {
		t.Fatalf("reopened rows = %d, want every project", len(reopened.surface.Matches()))
	}
}

func TestProjectHubBoundaryFilterFollowsRenameAndClearsOnDelete(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)
	openProjectHubFilter(t, hub)
	chooseProjectHubReferenceFilter(t, hub)
	reference := hub.boundaryFilter

	if err := app.runtime.Catalog().RenameBoundary(reference, "Parked"); err != nil {
		t.Fatalf("RenameBoundary: %v", err)
	}
	hub.RefreshProjectHub()
	if hub.boundaryFilter != reference {
		t.Fatalf("rename moved the filter to %q, want the same boundary ID", hub.boundaryFilter)
	}
	if label := hub.boundaryFilterLabel(); label != "Parked" {
		t.Fatalf("filter label = %q, want the new name", label)
	}
	if !slices.Contains(projectHubCategoryLabels(hub), "Parked") {
		t.Fatalf("category labels = %#v, want the renamed boundary", projectHubCategoryLabels(hub))
	}

	if _, err := app.runtime.Catalog().DeleteBoundary(reference, models.ActiveBoundaryID); err != nil {
		t.Fatalf("DeleteBoundary: %v", err)
	}
	hub.RefreshProjectHub()
	if hub.boundaryFilter != projectHubFilterAll {
		t.Fatalf("filter after delete = %q, want All", hub.boundaryFilter)
	}
	if !strings.Contains(hub.notice, "no longer exists") {
		t.Fatalf("delete notice = %q, want an explanation", hub.notice)
	}
	if len(hub.surface.Matches()) != 4 {
		t.Fatalf("rows after delete = %d, want every project", len(hub.surface.Matches()))
	}
}

func TestProjectHubEnterFocusesParkedProject(t *testing.T) {
	app := boundaryHubApp(t)
	hub := openProjectHub(t, app)

	// The parked project is reachable by its boundary name, not only its own.
	for _, r := range "reference" {
		hub.Update(syntheticKey(r))
	}
	if got := projectHubMatchIDs(hub); len(got) != 2 {
		t.Fatalf("boundary-name search matched %#v, want both reference projects", got)
	}

	focusProjectHubItem(t, hub, "project-hub:project:gamma")
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a parked project emitted no selection")
	}
	selected, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || selected.ProjectID != "gamma" {
		t.Fatalf("Enter on a parked project emitted %#v, want gamma", cmd())
	}
}
