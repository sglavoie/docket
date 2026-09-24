package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// A vocabulary rename is the one mutation whose write set is wider than the
// population the reader is looking at: the census reaches every unarchived
// project, so a parked one is written and then handed to the three refreshes
// the shell runs afterwards — reloadProjectByID, reloadCrossProjectOutlinerProject
// and refreshLoadedTaskWorkspace. Those refreshes are the seam where a project
// the base task workspace deliberately excludes could be revived into the
// Outliner, and the reload that follows the picker is where a project-qualified
// selection could quietly lose the qualification that makes it addressable.
// Everything below drives that one path through real keys against a real
// workspace on disk.

// unavailableProjectRenameEnv seeds one shared tag across the parked fixture's
// two projects and hands back the driver on the settled workspace. The tag has
// to live on both sides for the assertions to mean anything: on the active task
// so the `f#` facet can list and rename it, and on the parked task so the
// rename's census actually writes the project that must not come back.
func unavailableProjectRenameEnv(t *testing.T, tag string) (*UITestDriver, *models.Project, *models.Project) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureParked, 100, 35)
	if err != nil {
		t.Fatalf("fixture %s: %v", FixtureParked, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	runtime := driver.App().domain.runtime
	active := projectByFixtureName(runtime.TaskWorkspaceProjects(), parkedFixtureWorkspaceName)
	parked := projectByFixtureName(runtime.AvailableProjects(), parkedFixtureParkedName)
	if active == nil || parked == nil {
		t.Fatal("the parked fixture no longer holds one active and one parked project")
	}
	tagFixtureTaskOnDisk(t, active, "Ship the delivery checklist", tag)
	tagFixtureTaskOnDisk(t, parked, "Revisit the paused migration", tag)

	// The seed was written under the running app, so read it back the way any
	// external write is picked up rather than by reaching into the caches.
	driver.dispatch(status.ViewDidAppearMsg{})
	driver.drain()
	return driver, active, parked
}

// tagFixtureTaskOnDisk writes one tag onto one task's durable bytes. It goes
// through the same load/save pair the fixtures themselves use, so the running
// app sees an ordinary external edit and not a doctored in-memory pointer.
func tagFixtureTaskOnDisk(t *testing.T, project *models.Project, title, tag string) {
	t.Helper()
	loaded := diskCopyOfProject(t, project)
	for _, task := range loaded.Tasks {
		if task.Title != title {
			continue
		}
		task.Tags = append(task.Tags, tag)
		if err := storage.SaveProject(loaded); err != nil {
			t.Fatalf("SaveProject(%q): %v", project.Name, err)
		}
		return
	}
	t.Fatalf("project %q has no task %q to tag", project.Name, title)
}

func diskCopyOfProject(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	loaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(loaded); err != nil {
		t.Fatalf("LoadProject(%q): %v", project.Name, err)
	}
	return loaded
}

// diskTaskTags is the only tag reader here: an assertion about a project the
// Outliner is not allowed to hold can only be made against its bytes.
func diskTaskTags(t *testing.T, project *models.Project, title string) []string {
	t.Helper()
	for _, task := range diskCopyOfProject(t, project).Tasks {
		if task.Title == title {
			return task.Tags
		}
	}
	t.Fatalf("project %q has no task %q on disk", project.Name, title)
	return nil
}

// loadedProjectIDsForApp names the population the Outliner actually holds, which
// is the claim "not revived" is about: a project absent from the runtime's task
// workspace but present in this slice would be reachable by every mode, filter
// and projection downstream of it.
func loadedProjectIDsForApp(t *testing.T, driver *UITestDriver) []string {
	t.Helper()
	out := driver.App().crossOutliner
	if out == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	ids := make([]string, 0, len(out.Population.Projects))
	for _, project := range out.Population.Projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return ids
}

// assertParkedProjectStaysUnavailable is the invariant restated at every step of
// the workflow, because each step is a separate opportunity to break it: the
// runtime population, the loaded snapshot, and the drawn rows must all agree
// that the parked project is not here.
func assertParkedProjectStaysUnavailable(t *testing.T, driver *UITestDriver, parked *models.Project, when string) {
	t.Helper()
	app := driver.App()
	if app.domain.runtime.TaskWorkspaceProject(parked.ID) != nil {
		t.Fatalf("%s: the parked project rejoined the base task workspace", when)
	}
	if ids := loadedProjectIDsForApp(t, driver); slices.Contains(ids, parked.ID) {
		t.Fatalf("%s: loaded projects = %v, want the parked project absent", when, ids)
	}
	for _, row := range app.crossOutliner.Rows {
		if row.ProjectID == parked.ID {
			t.Fatalf("%s: a row was drawn for the parked project", when)
		}
	}
}

// moveCursorToTaskRow walks the cursor down to the first task row. The launch
// view opens on a section header, and a selection key is a no-op there, so the
// walk is what makes "v selects" a statement about selection rather than about
// where the cursor happened to land.
func moveCursorToTaskRow(t *testing.T, driver *UITestDriver) {
	t.Helper()
	out := driver.App().crossOutliner
	for step := 0; step < len(out.Rows); step++ {
		if out.Cursor >= 0 && out.Cursor < len(out.Rows) && out.Rows[out.Cursor].IsTaskRow() {
			return
		}
		// A launch folds the Backlog lane, so the walk opens it on the way.
		if row := out.Rows[out.Cursor]; row.IsCollapsibleHeader() && out.IsSectionRowCollapsed(row) {
			sendKeys(t, driver, "l")
		}
		sendKeys(t, driver, "j")
	}
	t.Fatal("the launch view drew no task row to select")
}

func sendKeys(t *testing.T, driver *UITestDriver, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
		driver.drain()
	}
}

func sendText(driver *UITestDriver, text string) {
	for _, r := range text {
		driver.dispatch(syntheticKey(r))
		driver.drain()
	}
}

// TestVocabularyRenameRefreshesNeverReviveAnUnavailableProject drives the whole
// picker workflow — open, rename, confirm, report, exit, undo — over a workspace
// whose parked project the rename genuinely writes. The parked bytes changing is
// what makes every "still absent" assertion non-vacuous: the refreshes were
// handed that project's ID and still left it out.
func TestVocabularyRenameRefreshesNeverReviveAnUnavailableProject(t *testing.T) {
	driver, active, parked := unavailableProjectRenameEnv(t, "logistics")
	assertParkedProjectStaysUnavailable(t, driver, parked, "at launch")

	sendKeys(t, driver, "f", "#", "e", "ctrl+u")
	sendText(driver, "delivery-ops")
	sendKeys(t, driver, "enter")
	screen := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(screen, "#logistics → #delivery-ops") || !strings.Contains(screen, "2 tasks in 2 projects") {
		t.Fatalf("rename confirmation did not name both projects:\n%s", screen)
	}
	sendKeys(t, driver, "y")
	if screen = driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "2 tasks rewritten in 2 projects") {
		t.Fatalf("rename report did not commit both projects:\n%s", screen)
	}

	if got := diskTaskTags(t, parked, "Revisit the paused migration"); !slices.Contains(got, "delivery-ops") {
		t.Fatalf("parked task tags on disk = %v, want the rename applied", got)
	}
	assertParkedProjectStaysUnavailable(t, driver, parked, "after the rename committed")

	sendKeys(t, driver, "esc", "esc")
	assertParkedProjectStaysUnavailable(t, driver, parked, "after leaving the picker")

	sendKeys(t, driver, "u")
	if got := diskTaskTags(t, parked, "Revisit the paused migration"); !slices.Contains(got, "logistics") {
		t.Fatalf("parked task tags after undo = %v, want the source name back", got)
	}
	if got := diskTaskTags(t, active, "Ship the delivery checklist"); !slices.Contains(got, "logistics") {
		t.Fatalf("active task tags after undo = %v, want the source name back", got)
	}
	assertParkedProjectStaysUnavailable(t, driver, parked, "after undo")
}

// TestSelectionKeepsItsProjectQualificationAcrossRenameAndUndo is the other half
// of the same workflow: a selection is a set of project-qualified identities, and
// the picker's completion rebuilds the rows those identities are matched against.
// A rebuild that dropped the qualification would leave a selection that still
// counts but no longer addresses anything.
func TestSelectionKeepsItsProjectQualificationAcrossRenameAndUndo(t *testing.T) {
	driver, active, parked := unavailableProjectRenameEnv(t, "logistics")
	out := driver.App().crossOutliner

	sendKeys(t, driver, "home")
	moveCursorToTaskRow(t, driver)
	sendKeys(t, driver, "v", "j", "v")
	before := out.SelectedTaskIdentities()
	if len(before) != 2 {
		t.Fatalf("selected identities = %v, want two rows selected", before)
	}
	for _, identity := range before {
		if identity.ProjectID != active.ID || identity.TaskID == "" {
			t.Fatalf("selected identity %+v is not qualified by the active project", identity)
		}
	}

	sendKeys(t, driver, "f", "#", "e", "ctrl+u")
	sendText(driver, "delivery-ops")
	sendKeys(t, driver, "enter", "y")
	if screen := driver.Observe(ObserveOptions{}).PlainScreen; !strings.Contains(screen, "2 tasks rewritten in 2 projects") {
		t.Fatalf("the rename this selection is measured across did not commit:\n%s", screen)
	}
	assertSameTaskIdentities(t, out.SelectedTaskIdentities(), before, "after the rename committed")

	// Leaving the picker reveals the Outliner, whose appearance reload replaces
	// the whole snapshot and prunes selections outside it. That prune is the one
	// place a surviving project's rows could be dropped along with a parked
	// project's, so it is asserted on the far side of a settled reload.
	sendKeys(t, driver, "esc", "esc")
	driver.dispatch(status.ViewDidAppearMsg{})
	driver.drain()
	assertSameTaskIdentities(t, out.SelectedTaskIdentities(), before, "after the reload that follows the picker")
	assertParkedProjectStaysUnavailable(t, driver, parked, "after the reload that follows the picker")

	sendKeys(t, driver, "u")
	assertSameTaskIdentities(t, out.SelectedTaskIdentities(), before, "after undo")
}

// assertSameTaskIdentities compares two identity sets as sets: order is the
// projection's business, and the claim here is only that the same qualified
// tasks are still named.
func assertSameTaskIdentities(t *testing.T, got, want []taskref.TaskIdentity, when string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: selected identities = %v, want %v", when, got, want)
	}
	for _, identity := range want {
		if !slices.Contains(got, identity) {
			t.Fatalf("%s: selected identities = %v, want %v", when, got, want)
		}
	}
}
