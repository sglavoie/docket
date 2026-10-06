package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// headlessClock is the injected application clock. It advances only when a test
// says so, so every stored instant is one the test named.
type headlessClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *headlessClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *headlessClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// headlessIDs mints deterministic UUID-shaped identities.
type headlessIDs struct {
	mu   sync.Mutex
	next int
}

func (g *headlessIDs) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", g.next)
}

// headlessEnvironment is one machine's whole configuration: a workspace, a
// machine home, and a process $HOME the runtime must never touch.
type headlessEnvironment struct {
	dataDir     string
	machineHome string
	processHome string
	clock       *headlessClock
	ids         *headlessIDs
}

func newHeadlessEnvironment(t *testing.T) headlessEnvironment {
	t.Helper()
	root := t.TempDir()
	env := headlessEnvironment{
		dataDir:     filepath.Join(root, "workspace"),
		machineHome: filepath.Join(root, "device"),
		processHome: filepath.Join(root, "process-home"),
		clock:       &headlessClock{now: time.Date(2026, time.October, 6, 10, 0, 0, 0, time.Local)},
		ids:         &headlessIDs{},
	}
	// Point the process environment somewhere the runtime has no business
	// writing. The explicit configuration is the only machine it may use, so this
	// directory must still not exist when the test ends.
	t.Setenv("HOME", env.processHome)
	t.Cleanup(func() {
		if _, err := os.Stat(env.processHome); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("runtime wrote beneath the process $HOME %s (stat err %v)", env.processHome, err)
		}
	})
	return env
}

func (env headlessEnvironment) config(create bool) Config {
	return Config{
		DataDir:         env.dataDir,
		Machine:         ExplicitMachine(env.machineHome, "headless-device-1"),
		Clock:           env.clock,
		IDs:             env.ids,
		CreateIfMissing: create,
	}
}

func openHeadless(t *testing.T, cfg Config) *Runtime {
	t.Helper()
	runtime, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open(%+v): %v", cfg, err)
	}
	return runtime
}

func queryOf(t *testing.T, mode models.QueryMode, clauses ...models.QueryClause) models.Query {
	t.Helper()
	query, err := models.NewQuery(mode, "", models.QuerySort{}, clauses...)
	if err != nil {
		t.Fatalf("NewQuery: %v", err)
	}
	return query
}

func clauseOf(t *testing.T, field models.QueryField, polarity models.QueryPolarity, operator models.QueryOperator, value string) models.QueryClause {
	t.Helper()
	clause, err := models.NewQueryClause(field, polarity, operator, value)
	if err != nil {
		t.Fatalf("NewQueryClause(%s): %v", field, err)
	}
	return clause
}

func runQuery(t *testing.T, runtime *Runtime, query models.Query) []QueryItem {
	t.Helper()
	result, err := runtime.Query(query)
	if err != nil {
		t.Fatalf("Query(%s): %v", query.String(), err)
	}
	if len(result.Diagnostics) > 0 {
		t.Fatalf("Query(%s) diagnostics: %+v", query.String(), result.Diagnostics)
	}
	return result.Items
}

func titlesOf(items []QueryItem) []string {
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Task.Title
	}
	return titles
}

func requireTitles(t *testing.T, label string, items []QueryItem, want ...string) {
	t.Helper()
	got := titlesOf(items)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("%s: got %q, want %q", label, got, want)
	}
}

// TestRuntimeHeadlessCapturePlanEditCompleteReopen drives the whole task life
// cycle through the shared runtime with no frontend: capture, planning, editing,
// completion, and reopening, then a second open of the same workspace that must
// read back exactly what the first committed, all under an explicit machine.
func TestRuntimeHeadlessCapturePlanEditCompleteReopen(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	service := runtime.Service()

	unfiled := runtime.State().SystemUnfiledProject()
	if unfiled == nil {
		t.Fatal("a fresh workspace has no protected Unfiled project")
	}

	captured, err := service.Capture(app.CaptureRequest{
		ProjectID: unfiled.ID,
		Title:     "Draft the release brief",
		Priority:  2,
		Tags:      []string{"work"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if captured.TaskID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("capture did not mint its identity through the injected generator: %q", captured.TaskID)
	}
	locator := app.TaskLocator{ProjectID: captured.ProjectID, TaskID: captured.TaskID}
	requireTitles(t, "inbox after capture", runQuery(t, runtime, queryOf(t, models.QueryModeInbox)), "Draft the release brief")

	env.clock.advance(time.Minute)
	if _, planErr := service.ApplyPlanning(app.PlanningRequest{Locator: locator, Action: app.PlanningAddToday}); planErr != nil {
		t.Fatalf("ApplyPlanning(add_today): %v", planErr)
	}
	today := runQuery(t, runtime, queryOf(t, models.QueryModeToday))
	requireTitles(t, "today after planning", today, "Draft the release brief")
	if !today[0].Today || today[0].Locator() != locator {
		t.Fatalf("today item does not carry its membership and identity: %+v", today[0])
	}

	env.clock.advance(time.Minute)
	title := "Draft and send the release brief"
	if _, editErr := service.Edit(app.EditRequest{Locator: locator, Title: &title}); editErr != nil {
		t.Fatalf("Edit: %v", editErr)
	}
	text := clauseOf(t, models.QueryFieldText, models.QueryPolarityPositive, models.QueryOperatorContains, "send")
	requireTitles(t, "text query after edit", runQuery(t, runtime, queryOf(t, models.QueryModeAll, text)), title)

	env.clock.advance(time.Minute)
	if _, doneErr := service.Transition(app.StateRequest{Locator: locator, Target: models.StateDone}); doneErr != nil {
		t.Fatalf("Transition(done): %v", doneErr)
	}
	requireTitles(t, "all after completion", runQuery(t, runtime, queryOf(t, models.QueryModeAll)))
	requireTitles(t, "today after completion", runQuery(t, runtime, queryOf(t, models.QueryModeToday)))
	done := clauseOf(t, models.QueryFieldState, models.QueryPolarityPositive, models.QueryOperatorIs, string(models.StateDone))
	requireTitles(t, "done query after completion", runQuery(t, runtime, queryOf(t, models.QueryModeAll, done)), title)
	if len(runtime.State().Planning().GetToday()) != 0 {
		t.Fatalf("completion left Today membership behind: %+v", runtime.State().Planning().GetToday())
	}

	env.clock.advance(time.Minute)
	if _, reopenErr := service.Transition(app.StateRequest{Locator: locator, Target: models.StateInbox}); reopenErr != nil {
		t.Fatalf("Transition(reopen): %v", reopenErr)
	}
	requireTitles(t, "inbox after reopen", runQuery(t, runtime, queryOf(t, models.QueryModeInbox)), title)

	undo, err := runtime.Ledger().UndoCount(storage.LedgerScope{})
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}
	if undo < 5 {
		t.Fatalf("expected one undo entry per committed step, got %d", undo)
	}

	// A second open is a fresh process reading what the first committed.
	reopened := openHeadless(t, env.config(false))
	items := runQuery(t, reopened, queryOf(t, models.QueryModeInbox))
	requireTitles(t, "inbox after reopening the workspace", items, title)
	task := items[0].Task
	if task.Priority != 2 || !task.HasTag("work") || task.CompletedAt != nil {
		t.Fatalf("reopened task lost a field: priority %d tags %v completed %v", task.Priority, task.Tags, task.CompletedAt)
	}
	if want := env.clock.Now(); !task.UpdatedAt.Equal(want) || !task.StateChangedAt.Equal(want) {
		t.Fatalf("stored clocks are not the injected clock: updated %v state changed %v, want %v", task.UpdatedAt, task.StateChangedAt, want)
	}
	markdown, err := os.ReadFile(storage.TasksFilePath(items[0].Project.Path))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	if !strings.Contains(string(markdown), title) {
		t.Fatalf("tasks.md does not carry the edited title:\n%s", markdown)
	}

	// Machine-local state lives beneath the configured machine home.
	machineFiles := 0
	_ = filepath.WalkDir(env.machineHome, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == "machine.json" {
			machineFiles++
		}
		return nil
	})
	if machineFiles != 1 {
		t.Fatalf("expected one machine.json beneath the machine home, found %d", machineFiles)
	}
}

// TestRuntimeOpenRequiresExplicitConfiguration refuses every configuration that
// would leave the runtime to guess a machine or a workspace.
func TestRuntimeOpenRequiresExplicitConfiguration(t *testing.T) {
	env := newHeadlessEnvironment(t)
	cases := map[string]Config{
		"missing data dir":      {Machine: ExplicitMachine(env.machineHome, "device")},
		"zero machine":          {DataDir: env.dataDir},
		"home without device":   {DataDir: env.dataDir, Machine: Machine{Home: env.machineHome}},
		"device without home":   {DataDir: env.dataDir, Machine: Machine{DeviceID: "device"}},
		"relative home":         {DataDir: env.dataDir, Machine: ExplicitMachine("device-home", "device")},
		"malformed device id":   {DataDir: env.dataDir, Machine: ExplicitMachine(env.machineHome, "my device")},
		"process plus a home":   {DataDir: env.dataDir, Machine: Machine{Home: env.machineHome, followProcess: true}},
		"overlong device id":    {DataDir: env.dataDir, Machine: ExplicitMachine(env.machineHome, strings.Repeat("d", 65))},
		"leading punctuation":   {DataDir: env.dataDir, Machine: ExplicitMachine(env.machineHome, "-device")},
		"process plus a device": {DataDir: env.dataDir, Machine: Machine{DeviceID: "device", followProcess: true}},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Open(cfg); err == nil {
				t.Fatalf("Open accepted %+v", cfg)
			}
		})
	}

	if _, err := Open(env.config(false)); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("Open without CreateIfMissing on an empty directory: %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := os.Stat(env.dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused open created the data directory (stat err %v)", err)
	}
}

// TestRuntimeBindsTheConfiguredDevice proves the manager every commit and
// recovery opens through owns transactions as the configured device and keeps
// machine-local state beneath the configured home, never the hostname or $HOME.
func TestRuntimeBindsTheConfiguredDevice(t *testing.T) {
	env := newHeadlessEnvironment(t)
	openHeadless(t, env.config(true))
	manager := newManager(env.dataDir, env.config(false).Machine)
	if owner := manager.TransactionOwner(); owner != "headless-device-1" {
		t.Fatalf("manager owner = %q, want the configured device", owner)
	}
	if root := manager.LocalDataRoot(); root != filepath.Join(env.machineHome, ".tt") {
		t.Fatalf("manager local root = %q, want beneath the machine home", root)
	}
	other := config.NewManagerForDevice(config.ConfigPathForDataDir(env.dataDir), env.machineHome, "headless-device-2")
	if other.TransactionOwner() == manager.TransactionOwner() {
		t.Fatal("two configured devices share one transaction owner")
	}
}

// TestRuntimeOpenRefusesMigrationWhenAsked holds RefuseMigration to its
// promise: a workspace older or newer than the engine is refused from the
// catalog alone, before recovery or any write, and a current one opens.
func TestRuntimeOpenRefusesMigrationWhenAsked(t *testing.T) {
	env := newHeadlessEnvironment(t)
	openHeadless(t, env.config(true))
	catalogPath := config.WorkspaceCatalogPath(env.dataDir)
	original, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	refusing := env.config(false)
	refusing.RefuseMigration = true
	openHeadless(t, refusing)

	for _, tc := range []struct {
		schema int
		check  func(error) bool
	}{
		{config.CurrentConfigVersion - 1, func(err error) bool { return errors.Is(err, ErrMigrationRequired) }},
		{config.SupportedConfigVersion + 1, func(err error) bool {
			var tooNew *config.SchemaTooNewError
			return errors.As(err, &tooNew) && tooNew.Found == config.SupportedConfigVersion+1
		}},
	} {
		rewritten := strings.Replace(string(original),
			fmt.Sprintf(`"schema_version": %d`, config.CurrentConfigVersion),
			fmt.Sprintf(`"schema_version": %d`, tc.schema), 1)
		if rewritten == string(original) {
			t.Fatal("the catalog payload schema was not found to rewrite")
		}
		if err = os.WriteFile(catalogPath, []byte(rewritten), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, openErr := Open(refusing); !tc.check(openErr) {
			t.Fatalf("schema %d: Open returned %v", tc.schema, openErr)
		}
		after, readErr := os.ReadFile(catalogPath)
		if readErr != nil || string(after) != rewritten {
			t.Fatalf("schema %d: a refused open rewrote the catalog (err %v)", tc.schema, readErr)
		}
	}
}
