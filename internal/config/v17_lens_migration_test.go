package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
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
				template.ID, template.Name, template.Mode, "",
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
// selected before.
func TestManagerV17MigrationRewritesPortableLensQueries(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.WorkspaceID == "" {
		t.Fatal("schema-16 catalog names no workspace")
	}

	cases := v17MigratedLensCases(t)
	records := make([]string, 0, len(cases))
	for _, testCase := range cases {
		records = append(records, testCase.record)
	}
	writeV16PortableLensDefinitions(t, root, catalog.WorkspaceID, records)

	if migrateErr := manager.MigrateWorkspaceToV17(); migrateErr != nil {
		t.Fatalf("migrate schema 17: %v", migrateErr)
	}

	definitions := readPortableLensDefinitions(t, root, catalog.WorkspaceID)
	if got := len(definitions.Lenses); got != len(cases) {
		t.Fatalf("migrated lens count = %d, want %d", got, len(cases))
	}

	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	rows := v17LensPopulation(now)
	for index, testCase := range cases {
		migrated := definitions.Lenses[index]
		t.Run(testCase.name, func(t *testing.T) {
			if migrated.ID != testCase.id {
				t.Fatalf("migrated lens ID = %q, want the frozen %q", migrated.ID, testCase.id)
			}
			if migrated.Name != testCase.name {
				t.Fatalf("migrated lens name = %q, want %q", migrated.Name, testCase.name)
			}
			if migrated.Query != testCase.wantQuer {
				t.Fatalf("migrated query = %q, want %q", migrated.Query, testCase.wantQuer)
			}
			if err := migrated.Validate(); err != nil {
				t.Fatalf("migrated lens does not validate: %v", err)
			}
			facets, remainder := models.PartitionQueryString(migrated.Query)
			if remainder != testCase.wantRemainder {
				t.Fatalf("migrated query remainder = %q, want %q", remainder, testCase.wantRemainder)
			}
			selected := v17SelectedLensRows(facets, rows, now)
			if strings.Join(selected, ",") != strings.Join(testCase.wantRows, ",") {
				t.Fatalf("migrated lens selects %v, want %v", selected, testCase.wantRows)
			}
		})
	}

	// The control the finding rests on: the retired spelling selects nothing over
	// a migrated population, so leaving a saved lens untranslated would have
	// emptied it silently.
	retired, _ := models.PartitionQueryString("#" + v16LensReviewTag)
	if selected := v17SelectedLensRows(retired, rows, now); len(selected) != 0 {
		t.Fatalf("retired review-tag narrowing still selects %v after migration", selected)
	}
}

// v17ReviewTagLensRecords is the smallest schema-16 collection that carries the
// finding: one lens narrowing by the review tag, which schema 15 removes from
// every task.
func v17ReviewTagLensRecords() []string {
	return []string{
		v16LensRecord(
			lensReviewQueueTmplID, lensReviewQueueTmplName, LensModeAll, "",
			LensSortUpdated, true, `{"tags":["review"]}`, "",
		),
	}
}

// v17StoredReviewSnapshotArchive is the published rollback archive's path.
func v17StoredReviewSnapshotArchive(t *testing.T, root string) string {
	t.Helper()
	for _, name := range preStoredReviewV17SnapshotNames(t, root) {
		if strings.HasSuffix(name, ".tar.gz") {
			return filepath.Join(PreScopeV7BackupsDir(root), name)
		}
	}
	t.Fatal("rollback snapshot archive missing")
	return ""
}

// waitForNextV17SnapshotStem sleeps into the next UTC second, which is the
// resolution a rollback snapshot stem is named at.
func waitForNextV17SnapshotStem() {
	now := time.Now().UTC()
	time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now) + 10*time.Millisecond)
}

// TestV17MigrationSnapshotArchivesTheSourceLensDefinitions proves the rewrite is
// recoverable: the archive published before activation holds the schema-16 lens
// bytes verbatim, so the one file the transaction rewrites in place can be
// restored beside the catalog and the task documents it rewrote with it.
func TestV17MigrationSnapshotArchivesTheSourceLensDefinitions(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	writeV16PortableLensDefinitions(t, root, catalog.WorkspaceID, v17ReviewTagLensRecords())
	source, err := os.ReadFile(LensDefinitionsPath(root))
	if err != nil {
		t.Fatal(err)
	}

	if migrateErr := manager.MigrateWorkspaceToV17(); migrateErr != nil {
		t.Fatalf("migrate schema 17: %v", migrateErr)
	}

	entries := readPreSimplificationV5Archive(t, v17StoredReviewSnapshotArchive(t, root))
	archived, ok := entries["workspace/.tt/lenses.json"]
	if !ok {
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		t.Fatalf("rollback archive holds no lens definitions: %v", names)
	}
	if !bytes.Equal(archived, source) {
		t.Fatalf("archived lens definitions are not the source bytes:\n%s", archived)
	}
	definitions := readPortableLensDefinitions(t, root, catalog.WorkspaceID)
	if len(definitions.Lenses) != 1 || definitions.Lenses[0].Query != "=review" {
		t.Fatalf("published lens definitions = %+v, want the one rewritten record", definitions.Lenses)
	}
}

// TestV17MigrationRefusesALensWriteAfterTheSnapshot holds the lens file to the
// same inventory guard every other workspace byte answers to, and then proves the
// refusal is retryable rather than terminal.
func TestV17MigrationRefusesALensWriteAfterTheSnapshot(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	writeV16PortableLensDefinitions(t, root, catalog.WorkspaceID, v17ReviewTagLensRecords())

	// The interposed write is itself a valid schema-16 collection, so the only
	// thing that can refuse activation is the inventory comparison.
	late := append(v17ReviewTagLensRecords(), v16LensRecord(
		"9c2f4b6a-1d3e-4f80-9a51-7b6c8d0e2f13", "Late Waiting", LensModeAll, "",
		LensSortDefault, true, `{"states":["waiting"],"states_set":true}`, "",
	))
	migrateErr := manager.migrateWorkspaceToV17(&v17MigrationActivationTestHooks{
		afterSnapshotPublished: func(string, string) {
			writeV16PortableLensDefinitions(t, root, catalog.WorkspaceID, late)
		},
	})
	if migrateErr == nil || !strings.Contains(migrateErr.Error(), "changed after the rollback snapshot was published") {
		t.Fatalf("interposed lens write error = %v, want the inventory refusal", migrateErr)
	}
	if !strings.Contains(migrateErr.Error(), filepath.Base(LensDefinitionsPath(root))) {
		t.Fatalf("inventory refusal does not name the lens file: %v", migrateErr)
	}
	schema, exists, err := workspaceCatalogPayloadSchema(root)
	if err != nil || !exists || schema != V16ConfigSchemaVersion {
		t.Fatalf("refused migration left schema %d exists=%v err=%v, want %d", schema, exists, err, V16ConfigSchemaVersion)
	}
	current, err := os.ReadFile(LensDefinitionsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(current, []byte(`"tasks"`)) {
		t.Fatalf("refused migration rewrote the lens definitions:\n%s", current)
	}

	// A snapshot stem is stamped to the second and never overwritten, so a retry
	// inside the same second refuses on the archive rather than on the workspace.
	// Waiting out that second is what a real retry does.
	waitForNextV17SnapshotStem()
	if retryErr := manager.MigrateWorkspaceToV17(); retryErr != nil {
		t.Fatalf("retry after the inventory refusal: %v", retryErr)
	}
	definitions := readPortableLensDefinitions(t, root, catalog.WorkspaceID)
	if len(definitions.Lenses) != len(late) {
		t.Fatalf("retry published %d lenses, want %d", len(definitions.Lenses), len(late))
	}
	for _, want := range []string{"=review", "$" + string(models.StateWaiting)} {
		found := false
		for _, lens := range definitions.Lenses {
			if lens.Query == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("retry published %+v, want a lens querying %q", definitions.Lenses, want)
		}
	}
}

// TestMachineLocalV16LensesLiftOnLoadAndHealOnTheNextWrite covers the other lens
// home. The guarded transaction owns the portable file alone, so a machine-local
// list an older build wrote is lifted by the shared decoder on every load and
// rewritten the next time that machine commits its own state — the only upgrade
// path a file the transaction does not own can have.
func TestMachineLocalV16LensesLiftOnLoadAndHealOnTheNextWrite(t *testing.T) {
	useMachine(t, t.TempDir())
	_, manager := stageWorkspaceBeforeAdoption(t)
	store := manager.domainStore()
	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	machinePath, err := MachineStatePath(catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	// The marker is absent, which reads as unadopted, so this write stays
	// machine-local instead of following the definitions to the workspace.
	records := strings.Join([]string{
		v16LensRecord(
			lensReviewQueueTmplID, lensReviewQueueTmplName, LensModeAll, "",
			LensSortUpdated, true, `{"tags":["review"]}`, "",
		),
		v16LensRecord(
			"9c2f4b6a-1d3e-4f80-9a51-7b6c8d0e2f14", "Typed Stale Review", LensModeAll, "#review",
			LensSortUpdated, true, `{"stale":true}`, "",
		),
	}, ",")
	envelope := fmt.Sprintf(
		`{"schema_version":%d,"workspace_id":%q,"kind":"machine","data":{"theme":"light","named_lenses":[%s]}}`,
		CurrentDomainSchemaVersion, catalog.WorkspaceID, records,
	)
	if err = os.WriteFile(machinePath, []byte(envelope), FilePermissions); err != nil {
		t.Fatal(err)
	}

	machine, revision, err := store.LoadMachineChecked()
	if err != nil {
		t.Fatalf("a machine-local schema-16 lens list must still load: %v", err)
	}
	wantQueries := []string{"=review", "=review ?stale"}
	if len(machine.NamedLenses) != len(wantQueries) {
		t.Fatalf("loaded %d machine-local lenses, want %d", len(machine.NamedLenses), len(wantQueries))
	}
	for index, want := range wantQueries {
		if got := machine.NamedLenses[index].Query; got != want {
			t.Fatalf("lifted machine-local query = %q, want %q", got, want)
		}
	}

	if err = store.SaveMachineChecked(machine, revision); err != nil {
		t.Fatalf("commit the machine's own state: %v", err)
	}
	healed, err := os.ReadFile(machinePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(healed, []byte(`"tasks"`)) {
		t.Fatalf("the next machine write did not heal the retired carriers:\n%s", healed)
	}
	for _, want := range wantQueries {
		if !bytes.Contains(healed, []byte(`"query": "`+want+`"`)) {
			t.Fatalf("healed machine payload does not carry %q:\n%s", want, healed)
		}
	}
}

// TestV17MigrationRecoversTheInterruptedLensRewrite proves the lens artifact
// rides the same prepared transaction as the catalog: an activation interrupted
// after prepare is completed by ordinary startup recovery, lens rewrite included.
func TestV17MigrationRecoversTheInterruptedLensRewrite(t *testing.T) {
	root := copyPreScopeV7Fixture(t).DataDir
	manager := NewManager(ConfigPathForDataDir(root))
	if err := manager.MigrateWorkspaceToV16(); err != nil {
		t.Fatalf("publish schema 16: %v", err)
	}
	catalog, err := readV8PublishedCatalog(WorkspaceCatalogPath(root))
	if err != nil {
		t.Fatal(err)
	}
	writeV16PortableLensDefinitions(t, root, catalog.WorkspaceID, v17ReviewTagLensRecords())

	plan, err := buildV17MigrationPlan(root)
	if err != nil {
		t.Fatalf("build activation plan: %v", err)
	}
	staged := false
	ops := make([]transaction.Op, 0, len(plan.artifacts)+len(plan.cleanup))
	for _, artifact := range plan.artifacts {
		if artifact.Path == LensDefinitionsPath(root) {
			staged = true
		}
		ops = append(ops, transaction.Op{
			Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode,
		})
	}
	if !staged {
		t.Fatal("activation plan stages no lens definitions")
	}
	ops = append(ops, plan.cleanup...)
	if _, err := createPreStoredReviewV17Snapshot(root); err != nil {
		t.Fatalf("create rollback snapshot: %v", err)
	}
	journal := transaction.NewJournal(TransactionsDir(root), []string{root, LocalDataRoot()}, "")
	if _, err := journal.Prepare(ops); err != nil {
		t.Fatalf("prepare interrupted activation: %v", err)
	}

	if err := manager.RecoverWorkspace(); err != nil {
		t.Fatalf("recover prepared schema-17 activation on startup: %v", err)
	}
	definitions := readPortableLensDefinitions(t, root, catalog.WorkspaceID)
	if len(definitions.Lenses) != 1 || definitions.Lenses[0].Query != "=review" {
		t.Fatalf("recovered lens definitions = %+v, want the one rewritten record", definitions.Lenses)
	}
}
