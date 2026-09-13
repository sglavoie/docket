package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// This file exercises the one thing the v17 boundary owes a saved lens: schema
// 15 removes the review tag from every task as it sets the stored flag, so a
// lens that narrowed by that tag stops selecting the rows it was saved for
// unless the same transaction rewrites it. The fixtures below are therefore
// actual schema-16 bytes rather than in-memory structs: a schema-16 build wrote
// "tasks" and "projects" unconditionally, so every record here carries both
// keys even when they are empty.

// v17LensCarrierFixture is one built-in template as schema 16 persisted it: the
// two retired carriers verbatim, beside the rows the lens must still select
// after migration.
type v17LensCarrierFixture struct {
	query    string
	tasks    string
	projects string
	wantRows []string
}

// v17BuiltinLensCarriers is the schema-16 carrier form of every built-in
// template, keyed by name. The test refuses a built-in with no entry here, so a
// template added later cannot pass migration coverage by simply not being
// listed.
func v17BuiltinLensCarriers() map[string]v17LensCarrierFixture {
	return map[string]v17LensCarrierFixture{
		lensWaitingStarterName: {
			tasks:    `{"states":["waiting"],"states_set":true}`,
			wantRows: []string{"waiting"},
		},
		lensHighPriorityStarter: {
			tasks:    `{"priority":1}`,
			wantRows: []string{"high"},
		},
		lensStaleStarterName: {
			tasks:    `{"stale":true}`,
			wantRows: []string{"stale"},
		},
		lensOverdueTemplateName: {
			tasks:    `{"due_date":"overdue"}`,
			wantRows: []string{"overdue"},
		},
		lensActiveWorkTmplName: {
			tasks:    `{"states":["doing"],"states_set":true}`,
			wantRows: []string{"doing"},
		},
		lensReviewQueueTmplName: {
			tasks:    `{"review":true}`,
			wantRows: []string{"review", "api-review"},
		},
		lensNeedsReviewTmplName: {
			// Schema 16 accepted an arbitrary saved query without parsing it. A
			// pre-feature ?review spelling therefore survives the migration as text
			// and becomes the canonical derived clause in the current build.
			query:    "?review",
			wantRows: []string{"review", "api-review"},
		},
		lensDeferredTmplName: {
			tasks:    `{"deferred":true}`,
			wantRows: []string{"deferred"},
		},
		lensCleanup90dTmplName: {
			// Schema 16 stored the state list in the order the template declared it;
			// canonical order sorts one field's values, so this pair must come back
			// as canceled before done.
			tasks: `{"states":["done","canceled"],"states_set":true,` +
				`"completion_age":"90d","completion_age_set":true}`,
			wantRows: []string{"done-old", "canceled-old"},
		},
		lensParkedWorkTmplName: {
			projects: `{"parked":"only"}`,
			wantRows: []string{"parked"},
		},
	}
}

// v17PersistedLensCase is one migrated lens under assertion.
type v17PersistedLensCase struct {
	id       string
	name     string
	record   string
	wantQuer string
	wantRows []string
	// wantRemainder is the half of the migrated query the canonical vocabulary
	// cannot spell, which the textgrammar surfaces own and no predicate here can
	// answer.
	wantRemainder string
}

// v16LensRecord renders one lens exactly as schema 16 wrote it.
func v16LensRecord(id, name, mode, query, sortField string, ascending bool, tasks, projects string) string {
	if tasks == "" {
		tasks = "{}"
	}
	if projects == "" {
		projects = "{}"
	}
	return fmt.Sprintf(
		`{"id":%q,"name":%q,"mode":%q,"query":%q,"sort":{"field":%q,"ascending":%t},"tasks":%s,"projects":%s}`,
		id, name, mode, query, sortField, ascending, tasks, projects,
	)
}

// v17LensRow is one (task, project) pair the migrated predicates are evaluated
// over. The population is already in its post-migration shape: review is the
// stored flag and no task carries the retired tag, which is exactly the state
// that makes an unmigrated lens select nothing.
type v17LensRow struct {
	name    string
	task    *models.Task
	project *models.Project
}

func v17LensPopulation(now time.Time) []v17LensRow {
	ordinary := &models.Project{ID: "project-ordinary", Slug: "ordinary", Name: "Ordinary"}
	parked := &models.Project{
		ID: "project-parked", Slug: "parked", Name: "Parked", Tags: []string{models.ParkedProjectTag},
	}
	overdue := now.AddDate(0, 0, -2)
	deferUntil := now.Add(48 * time.Hour)
	doneOld := now.AddDate(0, 0, -120)
	canceledOld := now.AddDate(0, 0, -100)
	doneFresh := now.AddDate(0, 0, -2)
	rows := []v17LensRow{
		{name: "waiting", task: &models.Task{Title: "Waiting on review board", WorkflowState: models.StateWaiting}},
		{name: "high", task: &models.Task{Title: "Ship the release", Priority: models.PriorityHigh}},
		{name: "stale", task: &models.Task{Title: "Untouched draft", UpdatedAt: now.AddDate(0, 0, -200)}},
		{name: "overdue", task: &models.Task{Title: "Late invoice", DueDate: &overdue}},
		{name: "doing", task: &models.Task{Title: "In flight", WorkflowState: models.StateDoing}},
		{name: "review", task: &models.Task{Title: "Needle draft", Review: true}},
		{name: "api-review", task: &models.Task{Title: "Endpoint audit", Tags: []string{"api"}, Review: true}},
		{name: "deferred", task: &models.Task{Title: "Later", DeferredUntil: &deferUntil}},
		{
			name: "done-old",
			task: &models.Task{Title: "Closed long ago", WorkflowState: models.StateDone, CompletedAt: &doneOld},
		},
		{
			name: "canceled-old",
			task: &models.Task{Title: "Dropped long ago", WorkflowState: models.StateCanceled, CompletedAt: &canceledOld},
		},
		{
			name: "done-fresh",
			task: &models.Task{Title: "Closed this week", WorkflowState: models.StateDone, CompletedAt: &doneFresh},
		},
		{name: "parked", task: &models.Task{Title: "Shelved work"}, project: parked},
	}
	for index := range rows {
		if rows[index].project == nil {
			rows[index].project = ordinary
		}
		rows[index].task.ID = rows[index].name
		// Staleness reads the touch clock, so every row but the stale one is fresh.
		if rows[index].task.UpdatedAt.IsZero() {
			rows[index].task.UpdatedAt = now
		}
	}
	return rows
}

// v17SelectedLensRows names the rows one canonical predicate admits, in
// population order.
func v17SelectedLensRows(query models.Query, rows []v17LensRow, now time.Time) []string {
	selected := make([]string, 0, len(rows))
	for _, row := range rows {
		if query.Admits(row.task, row.project, now) {
			selected = append(selected, row.name)
		}
	}
	return selected
}

// writeV16PortableLensDefinitions publishes the schema-16 collection into the
// portable home the guarded transaction owns.
func writeV16PortableLensDefinitions(t *testing.T, root, workspaceID string, records []string) {
	t.Helper()
	path := LensDefinitionsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatal(err)
	}
	envelope := fmt.Sprintf(
		`{"schema_version":%d,"workspace_id":%q,"kind":%q,"data":{"lenses":[%s]}}`,
		CurrentDomainSchemaVersion, workspaceID, domainLenses.name(), strings.Join(records, ","),
	)
	if err := os.WriteFile(path, []byte(envelope), FilePermissions); err != nil {
		t.Fatal(err)
	}
}

// readPortableLensDefinitions decodes the published collection as strictly as the
// runtime store does, after proving the retired carriers are gone from the bytes
// rather than merely ignored on load: a republished record that still carried
// them would be a second answer to what the lens selects.
func readPortableLensDefinitions(t *testing.T, root, workspaceID string) *WorkspaceLensDefinitions {
	t.Helper()
	path := LensDefinitionsPath(root)
	published, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{`"tasks"`, `"projects"`, `"states_set"`, `"completion_age"`} {
		if bytes.Contains(published, []byte(retired)) {
			t.Fatalf("migrated lens definitions still carry %s:\n%s", retired, published)
		}
	}
	definitions, err := readV17SourceLensDefinitions(path, published, workspaceID)
	if err != nil {
		t.Fatalf("decode migrated lens definitions: %v", err)
	}
	return definitions
}

// v17MigratedLensCases builds the whole assertion table from the live built-in
// catalog, so every template is covered by construction and a migrated built-in
// is held to the query its canonical definition now declares.
func v17MigratedLensCases(t *testing.T) []v17PersistedLensCase {
	t.Helper()
	carriers := v17BuiltinLensCarriers()
	templates := BuiltinNamedLensTemplates()
	cases := make([]v17PersistedLensCase, 0, len(templates)+2)
	for _, template := range templates {
		carrier, ok := carriers[template.Name]
		if !ok {
			t.Fatalf("built-in template %q has no schema-16 carrier fixture", template.Name)
		}
		cases = append(cases, v17PersistedLensCase{
			id:   template.ID,
			name: template.Name,
			record: v16LensRecord(
				template.ID, template.Name, template.Mode, carrier.query,
				template.Sort.Field, template.Sort.Ascending, carrier.tasks, carrier.projects,
			),
			wantQuer: template.Query,
			wantRows: carrier.wantRows,
		})
	}
	// A user lens that narrowed by the review tag is the finding's own case, in
	// both places schema 16 let it be spelled: the task-tag carrier, and the free
	// text the saved query carried.
	cases = append(cases,
		v17PersistedLensCase{
			id:   "9c2f4b6a-1d3e-4f80-9a51-7b6c8d0e2f11",
			name: "Tagged Review",
			record: v16LensRecord(
				"9c2f4b6a-1d3e-4f80-9a51-7b6c8d0e2f11", "Tagged Review", LensModeAll, "",
				LensSortUpdated, true, `{"tags":["api","review"]}`, "",
			),
			wantQuer: "#api =review",
			wantRows: []string{"api-review"},
		},
		v17PersistedLensCase{
			id:   "9c2f4b6a-1d3e-4f80-9a51-7b6c8d0e2f12",
			name: "Typed Review",
			record: v16LensRecord(
				"9c2f4b6a-1d3e-4f80-9a51-7b6c8d0e2f12", "Typed Review", LensModeAll, "#review -#api",
				LensSortUpdated, true, "", "",
			),
			wantQuer: "-#api =review",
			wantRows: []string{"review"},
		},
	)
	return cases
}

// TestManagerV17MigrationRewritesPortableLensQueries is the finding's own
// rehearsal: real schema-16 lens bytes go into the portable home, the guarded
// transaction migrates them with the catalog and every task document, and each
// definition comes back as one canonical query selecting exactly the rows it
