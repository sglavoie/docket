package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
)

// Regular contexts are labels on cataloged projects, so every assertion here
// reads the catalog rather than the hub's own list: the hub is only correct if
// what it renders and what the catalog stores agree after a save.

func catalogContextOf(t *testing.T, app *AppModel, id string) string {
	t.Helper()
	for _, entry := range app.runtime.Catalog().Projects {
		if entry.ID == id {
			return entry.Context
		}
	}
	t.Fatalf("catalog has no project %q", id)
	return ""
}

// addContextHubProject extends the shared fixture with one more cataloged
// project, archived or not, so tests can cover members the editor never lists.
func addContextHubProject(app *AppModel, id, name, label string, archived bool) {
	project := scopeTestProject(id, name, "1")
	project.Slug = id
	project.Context = label
	project.Archived = archived
	addTestAppProject(app, project)
}

// openRegularEditorOn focuses a regular label and opens its membership editor.
func openRegularEditorOn(t *testing.T, app *AppModel, label string) *ContextHubModel {
	t.Helper()
	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, RegularContextScope(label))
	hub.Update(syntheticKey('e'))
	if hub.editor == nil {
		t.Fatalf("e on %q opened no editor", label)
	}
	return hub
}

// saveRegularEditor commits the editor and returns the message the hub emitted,
// failing when the save was rejected instead.
func saveRegularEditor(t *testing.T, hub *ContextHubModel) RegularContextSavedMsg {
	t.Helper()
	_, cmd := hub.Update(contextHubSaveKey)
	if cmd == nil {
		errText := ""
		if hub.editor != nil {
			errText = hub.editor.err
		}
		t.Fatalf("save emitted no command (editor error: %q)", errText)
	}
	saved, ok := cmd().(RegularContextSavedMsg)
	if !ok {
		t.Fatalf("save emitted %#v, want RegularContextSavedMsg", cmd())
	}
	return saved
}

// TestContextHubAssignsProjectToRegularContext is the session's "done when": a
// project takes a label from the hub instead of one trip through Project Editor.
func TestContextHubAssignsProjectToRegularContext(t *testing.T) {
	app := contextHubApp(t)
	hub := openRegularEditorOn(t, app, "home")

	checkEditorProject(t, hub, "alpha")
	saved := saveRegularEditor(t, hub)

	if saved.Assigned != 1 || saved.Removed != 0 || saved.Renamed {
		t.Fatalf("save reported %#v, want one assignment", saved)
	}
	if got := catalogContextOf(t, app, "alpha"); got != "home" {
		t.Fatalf("alpha context = %q, want home", got)
	}
	if got := catalogContextOf(t, app, "beta"); got != "home" {
		t.Fatalf("beta lost its label: %q", got)
	}
	if hub.editor != nil {
		t.Fatal("a committed save must close the editor")
	}
	// "work" has no members left, so the hub must stop offering it.
	for _, scope := range contextHubScopes(hub) {
		if scope == RegularContextScope("work") {
			t.Fatal("the emptied label survived the refresh")
		}
	}
}

// TestContextHubRemovesProjectToDefaultContext covers the other direction:
// unchecking a member moves it to the default context rather than clearing it.
func TestContextHubRemovesProjectToDefaultContext(t *testing.T) {
	app := contextHubApp(t)
	hub := openRegularEditorOn(t, app, "work")

	checkEditorProject(t, hub, "alpha")
	saved := saveRegularEditor(t, hub)

	if saved.Removed != 1 || saved.Assigned != 0 {
		t.Fatalf("save reported %#v, want one removal", saved)
	}
	want := app.runtime.DefaultContext()
	if got := catalogContextOf(t, app, "alpha"); got != want {
		t.Fatalf("alpha context = %q, want the default %q", got, want)
	}
	if saved.Destination != want {
		t.Fatalf("save reported destination %q, want %q", saved.Destination, want)
	}
}

// TestContextHubKeepsHiddenRegularMembers guards the membership request itself:
// SaveRegularContext treats the request as the full membership, so an archived
// member the editor cannot show would be relabeled by an edit the user never
// saw.
func TestContextHubKeepsHiddenRegularMembers(t *testing.T) {
	app := contextHubApp(t)
	addContextHubProject(app, "gamma", "Gamma", "work", true)
	hub := openRegularEditorOn(t, app, "work")

	for _, project := range hub.editor.projects {
		if project.ID == "gamma" {
			t.Fatal("the editor listed an archived project")
		}
	}
	checkEditorProject(t, hub, "beta")
	saveRegularEditor(t, hub)

	if got := catalogContextOf(t, app, "gamma"); got != "work" {
		t.Fatalf("archived member context = %q, want work", got)
	}
	if got := catalogContextOf(t, app, "beta"); got != "work" {
		t.Fatalf("beta context = %q, want work", got)
	}
}

// TestContextHubPromptsForDestinationLeavingDefault covers the one label with no
// fallback of its own: removing from the default context asks where to go.

// TestContextHubBlocksDefaultRemovalWithoutAlternative separates "under-specified"
// from "impossible": with no other label to receive it, the removal is refused
// rather than prompted.

// TestContextHubRenamesRegularContextAcrossCatalog locks the catalog-wide rename
// including the archived members no surface lists, and the active scope following
// the new name.
func TestContextHubRenamesRegularContextAcrossCatalog(t *testing.T) {
	app := contextHubApp(t)
	addContextHubProject(app, "gamma", "Gamma", "work", true)
	app.crossOutliner.applyContextScope(RegularContextScope("work"))
	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, RegularContextScope("work"))

	hub.Update(syntheticKey('r'))
	if hub.editor == nil || hub.editor.listFocus {
		t.Fatal("r must open the editor on the name field")
	}
	hub.editor.nameInput.SetValue("Deep Work")
	saved := saveRegularEditor(t, hub)

	if !saved.Renamed || saved.RelabeledCount != 2 || saved.PreviousName != "work" {
		t.Fatalf("rename reported %#v, want 2 relabeled from work", saved)
	}
	for _, id := range []string{"alpha", "gamma"} {
		if got := catalogContextOf(t, app, id); got != "Deep Work" {
			t.Fatalf("%s context = %q, want Deep Work", id, got)
		}
	}
	if hub.active != RegularContextScope("Deep Work") {
		t.Fatalf("hub active scope = %#v, want the renamed label", hub.active)
	}

	handled, _ := app.handleContextMessages(saved)
	if !handled {
		t.Fatal("the shell ignored the regular-context save")
	}
	if got := app.crossOutliner.activeContext; got != RegularContextScope("Deep Work") {
		t.Fatalf("Outliner scope = %#v, want the renamed label", got)
	}
	if got := app.projectByID("alpha"); got == nil || got.Context != "Deep Work" {
		t.Fatalf("the shell's cached project kept a stale label: %#v", got)
	}
}

// TestContextHubPersonalNameCollisionLeavesMembershipUntouched locks the
// protected personal identity at the editor boundary: another label cannot be
// renamed onto it, and a rejected combined rename/membership save changes
// nothing.
func TestContextHubPersonalNameCollisionLeavesMembershipUntouched(t *testing.T) {
	app := contextHubApp(t)
	addContextHubProject(app, "gamma", "Gamma", "work", false)
	hub := openRegularEditorOn(t, app, "work")

	hub.editor.nameInput.SetValue(app.runtime.DefaultContext())
	checkEditorProject(t, hub, "gamma")

	if _, cmd := hub.Update(contextHubSaveKey); cmd != nil {
		t.Fatalf("the protected rename committed: %#v", cmd())
	}
	if hub.editor.err == "" {
		t.Fatal("the protected rename reported no editor error")
	}
	for _, id := range []string{"alpha", "gamma"} {
		if got := catalogContextOf(t, app, id); got != "work" {
			t.Fatalf("%s context = %q after a rejected protected rename", id, got)
		}
	}
	if hub.editor.originalName != "work" {
		t.Fatalf("editor identity = %q, want work", hub.editor.originalName)
	}
	if got := app.projectByID("alpha"); got == nil || got.Context != "work" {
		t.Fatalf("the shell's cached project drifted from the catalog: %#v", got)
	}
}

// TestContextHubRegularSaveFailureLeavesNothingBehind drives the same combined
// edit into a failing catalog commit: nothing durable, no runtime change, no
// shell cache refresh, and an editor still identified by the name it opened on.
func TestContextHubRegularSaveFailureLeavesNothingBehind(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	catalog := config.NewWorkspaceCatalog()
	catalog.Projects = []config.CatalogProject{
		{ID: "alpha", Slug: "alpha", Name: "Alpha", Context: "work", CreatedAt: now, UpdatedAt: now},
		{ID: "gamma", Slug: "gamma", Name: "Gamma", Context: "work", CreatedAt: now, UpdatedAt: now},
		{ID: "beta", Slug: "beta", Name: "Beta", Context: "home", CreatedAt: now, UpdatedAt: now},
	}
	if err := config.NewDomainFileStore(dataDir).SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	app, err := NewAppModel(runtime, config.NewManager(config.ConfigPathForDataDir(dataDir)), WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	hub := openRegularEditorOn(t, app, "work")

	// A catalog the store can read no longer is a commit that cannot be staged.
	// The backup sidecar goes too, because the store otherwise self-heals from it
	// and the commit would succeed.
	catalogPath := config.WorkspaceCatalogPath(dataDir)
	for _, path := range []string{catalogPath, catalogPath + ".bak"} {
		if writeErr := os.WriteFile(path, []byte("{broken"), config.FilePermissions); writeErr != nil {
			t.Fatalf("corrupt %s: %v", filepath.Base(path), writeErr)
		}
	}

	hub.editor.nameInput.SetValue("Deep Work")
	checkEditorProject(t, hub, "gamma")
	if _, cmd := hub.Update(contextHubSaveKey); cmd != nil {
		t.Fatalf("a failed commit still reported success: %#v", cmd())
	}
	if hub.editor == nil {
		t.Fatal("a failed commit closed the editor, stranding the edit")
	}
	if hub.editor.err == "" {
		t.Fatal("a failed commit left the editor with no error to show")
	}
	if hub.editor.originalName != "work" {
		t.Fatalf("editor identity = %q, want work after a failed commit", hub.editor.originalName)
	}

	for _, id := range []string{"alpha", "gamma"} {
		if got := runtime.GetProject(id); got == nil || got.Context != "work" {
			t.Fatalf("runtime snapshot for %s = %#v, want the pre-save label", id, got)
		}
		if got := app.projectByID(id); got == nil || got.Context != "work" {
			t.Fatalf("shell project cache for %s = %#v, want the pre-save label", id, got)
		}
	}
	// Any half that committed would have rewritten the file over the corruption.
	if durable, readErr := os.ReadFile(catalogPath); readErr != nil || string(durable) != "{broken" {
		t.Fatalf("durable catalog = %q (err %v), want the untouched sentinel", durable, readErr)
	}
}

// TestContextHubRejectsRegularNameCollision checks names case-insensitively so a
// workspace never carries two labels that only differ in casing.
func TestContextHubRejectsRegularNameCollision(t *testing.T) {
	app := contextHubApp(t)
	hub := openRegularEditorOn(t, app, "work")

	hub.editor.nameInput.SetValue("HOME")
	if _, cmd := hub.Update(contextHubSaveKey); cmd != nil {
		t.Fatalf("the colliding rename committed: %#v", cmd())
	}
	if !strings.Contains(hub.editor.err, "already exists") {
		t.Fatalf("editor error = %q, want a duplicate-name rejection", hub.editor.err)
	}
	if got := catalogContextOf(t, app, "alpha"); got != "work" {
		t.Fatalf("alpha context = %q, want work", got)
	}
}

// TestContextHubAllowsCaseOnlyRegularRename is the flip side: the collision rule
// must not block repainting a label's own display casing.
func TestContextHubAllowsCaseOnlyRegularRename(t *testing.T) {
	app := contextHubApp(t)
	hub := openRegularEditorOn(t, app, "work")

	hub.editor.nameInput.SetValue("Work")
	saved := saveRegularEditor(t, hub)
	if !saved.Renamed || saved.RelabeledCount != 1 {
		t.Fatalf("case-only rename reported %#v", saved)
	}
	if got := catalogContextOf(t, app, "alpha"); got != "Work" {
		t.Fatalf("alpha context = %q, want Work", got)
	}
}

// defaultContextWorkspace seeds a durable workspace whose default context names a
// label two projects carry. Renaming it has to move the catalog labels and
// Settings.DefaultContext together, so both halves must be readable from disk.

// durableContextOf reads a project's label straight off disk, which is the only
// way to tell a committed pair from a runtime snapshot that merely agrees.

// TestContextHubRenamesDefaultContextWithItsSetting covers the default label's
// rename: the catalog labels and Settings.DefaultContext name the same thing
// before and after, so they have to travel in one commit.

// TestContextHubDefaultContextRenameFailureMovesNeitherDomain breaks the settings
// half specifically: the catalog half is perfectly writable, so a catalog that
// moved anyway would leave the default naming a label no project carries.

// TestContextHubDeletesRegularContextIntoDefault covers `d` on a label: the
// projects survive under the default context and an active scope falls back.
func TestContextHubDeletesRegularContextIntoDefault(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(RegularContextScope("work"))
	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, RegularContextScope("work"))

	hub.Update(syntheticKey('d'))
	if !hub.confirm.IsActive() {
		t.Fatal("d did not arm the delete confirmation")
	}
	// The prompt wraps inside the overlay box, so assert on the destination name
	// rather than the whole sentence.
	overlay := ansi.Strip(app.renderConfirmDialogOverlay())
	if !strings.Contains(overlay, app.runtime.DefaultContext()) {
		t.Fatalf("the confirmation hid where the projects go: %q", overlay)
	}

	_, cmd := hub.Update(syntheticKey('y'))
	if cmd == nil {
		t.Fatal("confirming the delete emitted no command")
	}
	deleted, ok := cmd().(RegularContextDeletedMsg)
	if !ok || deleted.Name != "work" || deleted.ProjectCount != 1 || !deleted.WasActive {
		t.Fatalf("delete emitted %#v, want the active work label", cmd())
	}
	if got := catalogContextOf(t, app, "alpha"); got != app.runtime.DefaultContext() {
		t.Fatalf("alpha context = %q, want the default", got)
	}
	if hub.active != AllContextScope() {
		t.Fatalf("hub active scope = %#v, want All", hub.active)
	}

	app.handleContextMessages(deleted)
	if got := app.crossOutliner.activeContext; got != AllContextScope() {
		t.Fatalf("Outliner scope = %#v, want All", got)
	}
}

// TestContextHubRefusesDeletingDefaultContext protects the label every other
// removal falls back to.

// TestContextHubEmptyingActiveRegularContextFallsBackToAll locks the chosen
// resolution for a label that loses its last active member: regular scopes are
// derived from the labels active projects carry, so the emptied label leaves the
// hub list and cannot be reselected or marked active. Keeping it as the scope
// would leave the Outliner empty under a name nothing in the workspace carries.
func TestContextHubEmptyingActiveRegularContextFallsBackToAll(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(RegularContextScope("work"))
	if got := taskRowProjectIDs(app.crossOutliner); containsProject(got, "beta") {
		t.Fatalf("the work context did not narrow the Outliner first: %v", got)
	}
	hub := openRegularEditorOn(t, app, "work")

	// Alpha is the label's only active member, so unchecking it empties "work".
	checkEditorProject(t, hub, "alpha")
	saved := saveRegularEditor(t, hub)
	if !saved.WasActive || saved.Removed != 1 {
		t.Fatalf("save reported %#v, want one removal from the active label", saved)
	}
	if slices.Contains(contextHubScopes(hub), RegularContextScope("work")) {
		t.Fatalf("the emptied label survived the refresh: %#v", contextHubScopes(hub))
	}
	if hub.active != AllContextScope() {
		t.Fatalf("hub active scope = %#v, want All once the label stopped existing", hub.active)
	}

	handled, _ := app.handleContextMessages(saved)
	if !handled {
		t.Fatal("the shell ignored the regular-context save")
	}
	if got := app.crossOutliner.activeContext; got != AllContextScope() {
		t.Fatalf("Outliner scope = %#v, want All", got)
	}
	if got := app.crossOutliner.contextScopeProjectIDs; got != nil {
		t.Fatalf("All must admit every project, got %#v", got)
	}
	// The orphaned scope was visible as an empty Outliner, so the fallback is only
	// real if the rows came back.
	if got := taskRowProjectIDs(app.crossOutliner); !containsProject(got, "alpha") || !containsProject(got, "beta") {
		t.Fatalf("Outliner rows = %v, want every project under All", got)
	}
}

// TestContextHubEmptyingUnrelatedRegularContextFallsBackToAll covers the other
// way a label empties: the active scope is not the one being edited, but the
// edit takes in its last member. The scope has no members left either way.
func TestContextHubEmptyingUnrelatedRegularContextFallsBackToAll(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(RegularContextScope("home"))
	hub := openRegularEditorOn(t, app, "work")

	// Beta is "home"'s only member; assigning it to "work" vacates "home".
	checkEditorProject(t, hub, "beta")
	saved := saveRegularEditor(t, hub)
	if saved.WasActive || saved.Assigned != 1 {
		t.Fatalf("save reported %#v, want one assignment into an inactive label", saved)
	}
	if hub.active != AllContextScope() {
		t.Fatalf("hub active scope = %#v, want All once home stopped existing", hub.active)
	}

	app.handleContextMessages(saved)
	if got := app.crossOutliner.activeContext; got != AllContextScope() {
		t.Fatalf("Outliner scope = %#v, want All", got)
	}
	if got := taskRowProjectIDs(app.crossOutliner); !containsProject(got, "alpha") || !containsProject(got, "beta") {
		t.Fatalf("Outliner rows = %v, want every project under All", got)
	}
}

// TestContextHubReconcilesUnrelatedRegularScope covers the reconciliation the
// A regular-context save can move projects between
// labels, so a scope the user was not editing can still gain or lose members.
func TestContextHubReconcilesUnrelatedRegularScope(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(RegularContextScope("home"))
	hub := openRegularEditorOn(t, app, "home")

	// Edit "work" indirectly: alpha joins "home" while "home" stays the active
	// scope, so the Outliner must re-resolve even though nothing was renamed.
	checkEditorProject(t, hub, "alpha")
	saved := saveRegularEditor(t, hub)
	if !saved.WasActive {
		t.Fatalf("save reported %#v, want the active scope", saved)
	}

	app.handleContextMessages(saved)
	ids := app.crossOutliner.contextScopeProjectIDs
	if len(ids) != 2 || !ids["alpha"] || !ids["beta"] {
		t.Fatalf("Outliner scope resolved to %#v, want alpha and beta", ids)
	}
}
