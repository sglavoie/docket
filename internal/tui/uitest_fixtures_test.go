package tui

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

func TestUITestFixtureNames(t *testing.T) {
	got := BuiltinFixtureNames()
	want := []string{"busy", "check-ins", "checklist", "empty", "everyday", "fresh", "inbox-triage", "multi-project", "narrow", "parked", "project-mix", "release", "rich", "signals", "stale", "ux-acceptance", "workflow"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuiltinFixtureNames() = %v, want %v", got, want)
	}
}

func TestUITestFixtureUnknown(t *testing.T) {
	driver, ws, err := NewFixtureDriver("does-not-exist", 100, 30)
	if err == nil {
		if ws != nil {
			_ = ws.Cleanup()
		}
		t.Fatal("NewFixtureDriver(unknown) expected error, got nil")
	}
	if driver != nil || ws != nil {
		t.Fatalf("NewFixtureDriver(unknown) returned non-nil driver/ws on error: %v %v", driver, ws)
	}
}

func TestUITestFixtureBuildsAndRenders(t *testing.T) {
	for _, name := range BuiltinFixtureNames() {
		t.Run(name, func(t *testing.T) {
			width, height := 100, 30
			if name == FixtureNarrow {
				width, height = 62, 24
			}
			driver, ws, err := NewFixtureDriver(name, width, height)
			if err != nil {
				t.Fatalf("NewFixtureDriver(%q) error: %v", name, err)
			}
			t.Cleanup(func() { _ = ws.Cleanup() })

			// The workspace must live in a temp dir, not real user data.
			if !strings.HasPrefix(ws.Root, os.TempDir()) {
				t.Fatalf("workspace root %q is not under temp dir %q", ws.Root, os.TempDir())
			}
			if ws.Runtime.DataDir() != ws.DataDir || !strings.HasPrefix(ws.DataDir, ws.Root) {
				t.Fatalf("runtime DataDir %q not rooted in workspace %q", ws.Runtime.DataDir(), ws.Root)
			}

			obs := driver.Observe(ObserveOptions{})
			if strings.TrimSpace(obs.PlainScreen) == "" {
				t.Fatalf("fixture %q rendered an empty screen", name)
			}
			if strings.Contains(obs.PlainScreen, "\x1b") {
				t.Fatalf("fixture %q screen still contains ANSI escapes", name)
			}

			// Cleanup must remove the temp tree.
			root := ws.Root
			if err := ws.Cleanup(); err != nil {
				t.Fatalf("Cleanup() error: %v", err)
			}
			if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
				t.Fatalf("Cleanup() left temp dir behind: stat err = %v", statErr)
			}
		})
	}
}

func TestUITestFixtureEmptyHasNoUserTasks(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureEmpty, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver(empty) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	for _, p := range driver.App().taskWorkspaceProjects() {
		if len(p.Tasks) != 0 {
			t.Fatalf("empty fixture project %q unexpectedly has %d tasks", p.Name, len(p.Tasks))
		}
	}
}

func TestUITestFixtureParkedSplitsTaskWorkspaceFromCatalog(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureParked, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(parked) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	runtime := driver.App().domain.runtime
	parked := projectByFixtureName(runtime.AvailableProjects(), parkedFixtureParkedName)
	if parked == nil {
		t.Fatalf("%q is not an available project", parkedFixtureParkedName)
	}
	if !models.ProjectTagsParked(parked.Tags) {
		t.Fatalf("parked project tags = %v, want the reserved parked tag", parked.Tags)
	}
	if runtime.TaskWorkspaceProject(parked.ID) != nil {
		t.Fatalf("%q is in the base task workspace despite being parked", parked.Name)
	}

	active := projectByFixtureName(runtime.TaskWorkspaceProjects(), parkedFixtureWorkspaceName)
	if active == nil {
		t.Fatalf("%q is missing from the base task workspace", parkedFixtureWorkspaceName)
	}
	if models.ProjectTagsParked(active.Tags) {
		t.Fatalf("base workspace project tags = %v, want no parked tag", active.Tags)
	}

	// The fixture's work is Backlog, which the launch folds.
	driver.App().crossOutliner.ClearAllFolds()
	screen := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(screen, "Ship the delivery checklist") {
		t.Fatalf("Active Projects is missing the unparked project's work:\n%s", screen)
	}
	if strings.Contains(screen, "Revisit the paused migration") {
		t.Fatalf("Active Projects leaked a parked project's work:\n%s", screen)
	}
}

func projectByFixtureName(projects []*models.Project, name string) *models.Project {
	for _, project := range projects {
		if project != nil && project.Name == name {
			return project
		}
	}
	return nil
}

func TestFreshFixtureBootstrapsStableVisibleSystemInbox(t *testing.T) {
	driver, ws, fixtureErr := NewFixtureDriver(FixtureFresh, 100, 35)
	if fixtureErr != nil {
		t.Fatalf("NewFixtureDriver(fresh) error: %v", fixtureErr)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	inbox := driver.App().domain.runtime.SystemUnfiledProject()
	if inbox == nil {
		t.Fatal("fresh install has no system Inbox")
	}
	if inbox.Name != config.SystemUnfiledName || inbox.Archived {
		t.Fatalf("system Inbox metadata = %+v", inbox)
	}
	active := driver.App().domain.runtime.TaskWorkspaceProjects()
	if len(active) != 1 || active[0].ID != inbox.ID {
		t.Fatalf("fresh active projects = %+v, want visible system Inbox", active)
	}
	if _, err := os.Stat(storage.TasksFilePath(inbox.Path)); err != nil {
		t.Fatalf("system Inbox tasks file: %v", err)
	}

	reloaded, reloadErr := config.LoadRuntimeState(ws.DataDir)
	if reloadErr != nil {
		t.Fatalf("reload runtime: %v", reloadErr)
	}
	relaunched, err := newAppModelFromSnapshot(reloaded, config.NewManager(ws.ConfigPath))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	if got := relaunched.domain.runtime.SystemUnfiledProject(); got == nil || got.ID != inbox.ID {
		t.Fatalf("system Inbox identity changed across launch: before=%q after=%v", inbox.ID, got)
	}
}

func TestContextualQuickCapturePersistsInOwningProject(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	target := driver.App().currentProject
	inbox := driver.App().domain.runtime.SystemUnfiledProject()
	if target == nil || inbox == nil || target.ID == inbox.ID {
		t.Fatalf("fixture projects: target=%v inbox=%v", target, inbox)
	}
	targetCount := len(target.Tasks)
	inboxCount := len(inbox.Tasks)

	driver.dispatch(driver.App().captureComposerMessage(target, driver.App().taskWorkspaceProjects()))
	if driver.App().currentView() != presentation.ViewTaskComposer {
		t.Fatalf("current view = %s, want QuickCapture", driver.App().currentView())
	}
	const title = "Contextual capture stays with Rich Project"
	for _, action := range []UITestAction{
		{Type: UITestActionText, Text: title},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	} {
		if _, actionErr := driver.ApplyAction(action, ObserveOptions{}); actionErr != nil {
			t.Fatal(actionErr)
		}
	}

	if err = storage.LoadProject(target); err != nil {
		t.Fatalf("reload target project: %v", err)
	}
	if err = storage.LoadProject(inbox); err != nil {
		t.Fatalf("reload system Inbox: %v", err)
	}
	if len(target.Tasks) != targetCount+1 {
		t.Fatalf("target task count = %d, want %d", len(target.Tasks), targetCount+1)
	}
	if len(inbox.Tasks) != inboxCount {
		t.Fatalf("system Inbox task count = %d, want %d", len(inbox.Tasks), inboxCount)
	}

	var captured *models.Task
	for _, task := range target.Tasks {
		if task.Title == title {
			captured = task
			break
		}
	}
	if captured == nil {
		t.Fatalf("captured task %q not found in target project", title)
	}
	if captured.State() != models.StateInbox {
		t.Fatalf("captured task section = %q, want Inbox", captured.State())
	}
	for _, task := range inbox.Tasks {
		if task.Title == title {
			t.Fatalf("captured task %q was persisted in the system Inbox", title)
		}
	}
}
