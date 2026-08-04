package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// The Boundary Manager is the only surface that can administer boundaries, so
// these tests pin the two properties that make it trustworthy: Active is
// visible but never mutable, and every destructive path names its whole effect
// against identities frozen before the question was asked.

// openBoundaryManager leaves the Hub's opening search and presses B.
func openBoundaryManager(t *testing.T, app *AppModel) (*ProjectHubModel, *projectBoundaryManager) {
	t.Helper()
	hub := openProjectHub(t, app)
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	hub.Update(syntheticKey('B'))
	if hub.boundaries == nil {
		t.Fatal("browse B did not open the Boundary Manager")
	}
	return hub, hub.boundaries
}

func boundaryManagerRows(manager *projectBoundaryManager) []string {
	rows := make([]string, 0, len(manager.surface.Matches()))
	for _, match := range manager.surface.Matches() {
		rows = append(rows, match.Item.PrimaryText)
	}
	return rows
}

func boundaryManagerRow(t *testing.T, manager *projectBoundaryManager, name string) ListItem {
	t.Helper()
	for _, item := range manager.surface.Items() {
		if strings.HasPrefix(item.PrimaryText, name) {
			return item
		}
	}
	t.Fatalf("Boundary Manager omitted %q from %#v", name, boundaryManagerRows(manager))
	return ListItem{}
}

func focusBoundaryRow(t *testing.T, hub *ProjectHubModel, name string) projectBoundaryActionTarget {
	t.Helper()
	item := boundaryManagerRow(t, hub.boundaries, name)
	if !hub.boundaries.surface.SetCursorToID(item.ID) {
		t.Fatalf("Boundary Manager could not focus %q", name)
	}
	target, ok := item.Data.(projectBoundaryActionTarget)
	if !ok {
		t.Fatalf("boundary row %q carries %T, want a frozen target", name, item.Data)
	}
	return target
}

func typeIntoBoundaryManager(hub *ProjectHubModel, text string) {
	for _, r := range text {
		hub.Update(syntheticKey(r))
	}
}

// singleBoundaryMsg drains one command and returns the only message it produced.
func singleBoundaryMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msgs := drainTUITestCmd(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("command produced %d messages, want exactly 1: %#v", len(msgs), msgs)
	}
	return msgs[0]
}

func TestBoundaryManagerOpensFromProjectHubAndEscReturnsToProjects(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)

	if !hub.HasTextInput() || !hub.ShouldCaptureEsc() {
		t.Fatal("the open manager must own the Hub key stream so no shell interceptor fires")
	}
	if got := hub.footerLabel(); got != boundaryManagerFooterLabel {
		t.Fatalf("footer label = %q, want %q", got, boundaryManagerFooterLabel)
	}
	if title := ansi.Strip(hub.renderProjectHubTitle(120)); !strings.Contains(title, "Boundaries") {
		t.Fatalf("title = %q, want the manager heading", title)
	}
	if manager.surface == nil {
		t.Fatal("the manager opened without a listing")
	}

	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.boundaries != nil {
		t.Fatal("browse Esc did not return to the project list")
	}
	if got := hub.footerLabel(); got != projectHubFooterLabel {
		t.Fatalf("footer label after Esc = %q, want %q", got, projectHubFooterLabel)
	}
	if len(hub.surface.Matches()) != 4 {
		t.Fatalf("project rows after Esc = %d, want every project", len(hub.surface.Matches()))
	}
}

func TestBoundaryManagerListsActiveFirstWithCatalogMemberCounts(t *testing.T) {
	app := boundaryHubApp(t)
	_, manager := openBoundaryManager(t, app)

	wantRows := []string{config.ActiveBoundaryName + " (protected)", "reference", "Someday"}
	if got := boundaryManagerRows(manager); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("rows = %#v, want %#v", got, wantRows)
	}

	// Counts come from catalog metadata, so the archived member of Someday is
	// counted even though it is never loaded.
	wantCounts := map[string]string{
		config.ActiveBoundaryName: "1 project · 1 unarchived · 0 archived",
		"reference":               "2 projects · 2 unarchived · 0 archived",
		"Someday":                 "1 project · 0 unarchived · 1 archived",
	}
	for name, want := range wantCounts {
		if got := boundaryManagerRow(t, manager, name).SecondaryText; got != want {
			t.Errorf("%s counts = %q, want %q", name, got, want)
		}
	}
}

func TestBoundaryManagerKeepsActiveVisibleButRefusesToRenameOrDeleteIt(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)
	focusBoundaryRow(t, hub, config.ActiveBoundaryName)

	for _, key := range []rune{'r', 'd'} {
		_, cmd := hub.Update(syntheticKey(key))
		if cmd != nil {
			t.Fatalf("%q on Active emitted a command", string(key))
		}
		if manager.editor != nil || manager.destination != nil || manager.confirm.IsActive() {
			t.Fatalf("%q on Active opened a mutation surface", string(key))
		}
		if !strings.Contains(manager.notice, "protected task workspace") {
			t.Fatalf("%q on Active notice = %q, want a concrete reason", string(key), manager.notice)
		}
	}

	// The sheet still lists both actions so the reason is discoverable there too.
	hub.Update(syntheticKey(' '))
	if manager.actions == nil {
		t.Fatal("Space did not open the boundary action sheet")
	}
	disabled := map[OperationID]string{}
	for _, item := range manager.actions.surface.Items() {
		descriptor, ok := item.Data.(OperationDescriptor)
		if !ok {
			continue
		}
		disabled[descriptor.ID] = item.DisabledReason
	}
	for _, id := range []OperationID{operationBoundaryRename, operationBoundaryDelete} {
		if !strings.Contains(disabled[id], "cannot be") {
			t.Errorf("%s on Active is offered without a reason: %q", id, disabled[id])
		}
	}
	if disabled[operationBoundaryCreate] != "" {
		t.Errorf("create is disabled on Active: %q", disabled[operationBoundaryCreate])
	}
}

func TestBoundaryManagerCreateCommitsAndRebuildsBothSurfaces(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)

	_, cmd := hub.Update(syntheticKey('n'))
	drainTUITestCmd(t, cmd)
	if manager.editor == nil || manager.editor.boundaryID != "" {
		t.Fatalf("n opened editor %#v, want an empty-name creation prompt", manager.editor)
	}
	typeIntoBoundaryManager(hub, "Archive Ideas")
	_, cmd = hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if manager.editor != nil {
		t.Fatal("saving left the editor open")
	}
	created, ok := singleBoundaryMsg(t, cmd).(CreateBoundaryMsg)
	if !ok || created.Name != "Archive Ideas" {
		t.Fatalf("save emitted %#v, want CreateBoundaryMsg for the typed name", created)
	}

	app.Update(created)
	if !catalogHasBoundary(app, "Archive Ideas") {
		t.Fatal("the create was not published to the catalog")
	}
	if got := boundaryManagerRow(t, manager, "Archive Ideas").SecondaryText; got != "0 projects · 0 unarchived · 0 archived" {
		t.Fatalf("new boundary counts = %q, want an empty boundary", got)
	}
	if !sliceContainsString(projectHubCategoryLabels(hub), "Archive Ideas") {
		t.Fatalf("Project Hub groups = %#v, want the new boundary", projectHubCategoryLabels(hub))
	}
}

func TestBoundaryManagerEditorRefusesInvalidNamesInPlace(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)

	hub.Update(syntheticKey('n'))
	typeIntoBoundaryManager(hub, "REFERENCE")
	_, cmd := hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("a duplicate name was submitted for commit")
	}
	if manager.editor == nil {
		t.Fatal("a rejected name closed the editor")
	}
	if !strings.Contains(manager.editor.err, "already exists") {
		t.Fatalf("editor error = %q, want a name conflict", manager.editor.err)
	}

	for range "REFERENCE" {
		hub.Update(syntheticSpecialKey(tea.KeyBackspace))
	}
	if _, cmd = hub.Update(syntheticSpecialKey(tea.KeyEnter)); cmd != nil {
		t.Fatal("an empty name was submitted for commit")
	}
	if manager.editor == nil || !strings.Contains(manager.editor.err, "empty") {
		t.Fatalf("empty-name error = %#v, want an inline rejection", manager.editor)
	}
}

func TestBoundaryManagerRenameCommitsAndKeepsTheSelectedBoundary(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)
	target := focusBoundaryRow(t, hub, "reference")

	hub.Update(syntheticKey('r'))
	if manager.editor == nil || manager.editor.boundaryID != target.BoundaryID {
		t.Fatalf("r opened editor %#v, want the frozen reference boundary", manager.editor)
	}
	for range "reference" {
		hub.Update(syntheticSpecialKey(tea.KeyBackspace))
	}
	typeIntoBoundaryManager(hub, "Parked")
	_, cmd := hub.Update(syntheticSpecialKey(tea.KeyEnter))
	renamed, ok := singleBoundaryMsg(t, cmd).(RenameBoundaryMsg)
	if !ok || renamed.BoundaryID != target.BoundaryID || renamed.Name != "Parked" {
		t.Fatalf("save emitted %#v, want a rename of the frozen boundary", renamed)
	}

	app.Update(renamed)
	if got := boundaryManagerRow(t, manager, "Parked").ID; got != boundaryManagerItemID(target.BoundaryID) {
		t.Fatalf("renamed row ID = %q, want the durable boundary ID", got)
	}
	if got := manager.currentItemID(); got != boundaryManagerItemID(target.BoundaryID) {
		t.Fatalf("cursor after rename = %q, want the renamed boundary", got)
	}
}

func TestBoundaryManagerDiscardsAnUnsavedNameOnlyAfterConfirmation(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)

	hub.Update(syntheticKey('n'))
	typeIntoBoundaryManager(hub, "Draft")
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if manager.editor == nil {
		t.Fatal("Esc discarded a typed name without asking")
	}
	dialog := hub.GetConfirmDialog()
	if dialog == nil || !dialog.IsActive() || !strings.Contains(dialog.Prompt(), "Discard") {
		t.Fatalf("discard prompt = %#v, want an active discard confirmation", dialog)
	}

	hub.Update(syntheticKey('n'))
	if manager.editor == nil || manager.confirmDiscard {
		t.Fatal("declining the discard did not return to the editor")
	}
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	hub.Update(syntheticKey('y'))
	if manager.editor != nil {
		t.Fatal("accepting the discard kept the editor open")
	}
	if hub.boundaries == nil {
		t.Fatal("discarding a name also closed the manager")
	}
}

func TestBoundaryManagerDeleteAsksForADestinationExcludingTheSource(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)
	source := focusBoundaryRow(t, hub, "reference")

	hub.Update(syntheticKey('d'))
	if manager.destination == nil {
		t.Fatal("deleting a non-empty boundary skipped the destination step")
	}
	var offered []string
	for _, item := range manager.destination.surface.Items() {
		offered = append(offered, item.PrimaryText)
	}
	want := []string{config.ActiveBoundaryName, "Someday"}
	if !reflect.DeepEqual(offered, want) {
		t.Fatalf("destinations = %#v, want %#v", offered, want)
	}

	typeIntoBoundaryManager(hub, "Someday")
	hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if manager.destination != nil {
		t.Fatal("choosing a destination left the picker open")
	}
	prompt := manager.confirm.Prompt()
	for _, fragment := range []string{"reference", "Someday", "2 projects"} {
		if !strings.Contains(prompt, fragment) {
			t.Errorf("confirm prompt = %q, want it to state %q", prompt, fragment)
		}
	}

	_, cmd := hub.Update(syntheticKey('y'))
	deleted, ok := singleBoundaryMsg(t, cmd).(DeleteBoundaryMsg)
	someday := boundaryIDNamed(t, app, "Someday")
	if !ok || deleted.BoundaryID != source.BoundaryID || deleted.DestinationID != someday {
		t.Fatalf("confirm emitted %#v, want the frozen source and destination", deleted)
	}

	app.Update(deleted)
	if catalogHasBoundary(app, "reference") {
		t.Fatal("the confirmed deletion was not committed")
	}
	if got := app.runtime.Catalog().ProjectBoundaryID("gamma"); got != someday {
		t.Fatalf("Gamma boundary after delete = %q, want the chosen destination", got)
	}
	if got := boundaryManagerRows(manager); !reflect.DeepEqual(got, []string{
		config.ActiveBoundaryName + " (protected)", "Someday",
	}) {
		t.Fatalf("rows after delete = %#v, want the source gone", got)
	}
}

func TestBoundaryManagerDeleteOfAnEmptyBoundarySkipsTheDestinationStep(t *testing.T) {
	app := boundaryHubApp(t)
	if _, err := app.runtime.Catalog().CreateBoundary("Spare"); err != nil {
		t.Fatalf("CreateBoundary: %v", err)
	}
	hub, manager := openBoundaryManager(t, app)
	source := focusBoundaryRow(t, hub, "Spare")

	hub.Update(syntheticKey('d'))
	if manager.destination != nil {
		t.Fatal("an empty boundary asked for a destination")
	}
	if !manager.confirm.IsActive() {
		t.Fatal("an empty boundary was deleted without confirmation")
	}
	if !strings.Contains(manager.confirm.Prompt(), "no projects") {
		t.Fatalf("confirm prompt = %q, want it to say the boundary is empty", manager.confirm.Prompt())
	}

	_, cmd := hub.Update(syntheticKey('y'))
	deleted, ok := singleBoundaryMsg(t, cmd).(DeleteBoundaryMsg)
	if !ok || deleted.BoundaryID != source.BoundaryID || deleted.DestinationID != models.ActiveBoundaryID {
		t.Fatalf("confirm emitted %#v, want an empty deletion into the task workspace", deleted)
	}
}

func TestBoundaryManagerDeclinedDeletionMutatesNothing(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)
	focusBoundaryRow(t, hub, "reference")
	before := boundaryMembership(app)

	hub.Update(syntheticKey('d'))
	typeIntoBoundaryManager(hub, config.ActiveBoundaryName)
	hub.Update(syntheticSpecialKey(tea.KeyEnter))
	_, cmd := hub.Update(syntheticKey('n'))
	if cmd != nil {
		t.Fatal("declining the confirmation still emitted a mutation")
	}
	if manager.pending != (pendingBoundaryDelete{}) {
		t.Fatalf("declined deletion left %#v armed", manager.pending)
	}
	if !reflect.DeepEqual(boundaryMembership(app), before) {
		t.Fatal("a declined deletion changed stored membership")
	}
}

// The confirmation answers a question about one boundary, so the rows the
// cursor happens to be on when the answer arrives must not matter.
func TestBoundaryManagerFreezesTheDeletionAgainstCursorMovement(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)
	source := focusBoundaryRow(t, hub, "reference")

	hub.Update(syntheticKey('d'))
	typeIntoBoundaryManager(hub, config.ActiveBoundaryName)
	hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if !manager.confirm.IsActive() {
		t.Fatal("the deletion was not armed")
	}

	// A confirmation blocks navigation, so retargeting is forced directly.
	manager.surface.MoveCursorToEnd()
	if got, _ := manager.selectedTarget(); got.BoundaryID == source.BoundaryID {
		t.Fatal("the fixture did not move the cursor off the armed boundary")
	}

	_, cmd := hub.Update(syntheticKey('y'))
	deleted, ok := singleBoundaryMsg(t, cmd).(DeleteBoundaryMsg)
	if !ok || deleted.BoundaryID != source.BoundaryID {
		t.Fatalf("confirm deleted %#v, want the boundary the prompt named", deleted)
	}
	if deleted.DestinationID != models.ActiveBoundaryID {
		t.Fatalf("destination = %q, want the frozen answer", deleted.DestinationID)
	}
}

func TestBoundaryManagerSearchNarrowsRowsAndEscClearsBeforeClosing(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)

	hub.Update(syntheticKey('/'))
	typeIntoBoundaryManager(hub, "some")
	if got := boundaryManagerRows(manager); !reflect.DeepEqual(got, []string{"Someday"}) {
		t.Fatalf("search rows = %#v, want only Someday", got)
	}
	if bar := ansi.Strip(hub.renderSearchBar()); !strings.Contains(bar, "some") {
		t.Fatalf("search bar = %q, want the typed query", bar)
	}

	hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if manager.searching || manager.query != "some" {
		t.Fatalf("Enter dropped the committed query: searching=%v query=%q", manager.searching, manager.query)
	}

	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if manager.query != "" {
		t.Fatalf("Esc left the query %q", manager.query)
	}
	if hub.boundaries == nil {
		t.Fatal("the first Esc closed the manager instead of clearing the search")
	}
	if len(manager.surface.Matches()) != 3 {
		t.Fatalf("rows after clearing = %d, want every boundary", len(manager.surface.Matches()))
	}
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.boundaries != nil {
		t.Fatal("the second Esc did not return to the project list")
	}
}

func TestBoundaryManagerSpaceActionsRunRegisteredDescriptors(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)
	target := focusBoundaryRow(t, hub, "reference")

	for _, id := range []OperationID{operationBoundaryCreate, operationBoundaryRename, operationBoundaryDelete} {
		descriptor, ok := app.GetOperationRegistry().Descriptor(id)
		if !ok {
			t.Fatalf("operation %s is not registered", id)
		}
		if descriptor.Category != OperationCategoryProject {
			t.Errorf("%s category = %q, want project", id, descriptor.Category)
		}
	}

	hub.Update(syntheticKey(' '))
	if manager.actions == nil {
		t.Fatal("Space did not open the boundary action sheet")
	}
	if !manager.actions.surface.SetCursorToID("boundary-manager-action:" + ListItemID(operationBoundaryRename)) {
		t.Fatalf("the sheet omitted %s", operationBoundaryRename)
	}
	hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if manager.actions != nil {
		t.Fatal("running a sheet action left the sheet open")
	}
	if manager.editor == nil || manager.editor.boundaryID != target.BoundaryID {
		t.Fatalf("the sheet rename opened %#v, want the frozen row", manager.editor)
	}
}

func TestBoundaryManagerFooterHelpStaysOnOneLine(t *testing.T) {
	app := boundaryHubApp(t)
	hub, manager := openBoundaryManager(t, app)

	modes := []struct {
		name  string
		setup func()
	}{
		{name: "browse", setup: func() {}},
		{name: "search", setup: func() {
			hub.Update(syntheticKey('/'))
			typeIntoBoundaryManager(hub, "so")
		}},
		{name: "search committed", setup: func() {
			hub.Update(syntheticSpecialKey(tea.KeyEnter))
		}},
		{name: "editor", setup: func() {
			hub.Update(syntheticSpecialKey(tea.KeyEscape))
			hub.Update(syntheticKey('n'))
		}},
		{name: "destination picker", setup: func() {
			hub.Update(syntheticSpecialKey(tea.KeyEscape))
			focusBoundaryRow(t, hub, "reference")
			hub.Update(syntheticKey('d'))
			if manager.destination == nil {
				t.Fatal("the destination picker did not open")
			}
		}},
	}
	for _, mode := range modes {
		mode.setup()
		for _, width := range []int{60, 70, 80, 100, 120} {
			hub.SetSize(width, 20)
			help := hub.footerHelp(width)
			footer := renderStatusBarFooter(
				HelpStyle.Render(help),
				MutedStyle.Render(hub.footerLabel()),
				width,
				hub.AccentColor(),
			)
			if lines := strings.Count(footer, "\n"); lines != 1 {
				t.Errorf("%s at width %d: footer occupies %d lines\n%s",
					mode.name, width, lines+1, ansi.Strip(footer))
			}
			if got := lipgloss.Width(help) + lipgloss.Width(hub.footerLabel()) + 1; got > width {
				t.Errorf("%s at width %d: help+label needs %d columns", mode.name, width, got)
			}
			if strings.TrimSpace(ansi.Strip(help)) == "" {
				t.Errorf("%s at width %d: help collapsed to nothing", mode.name, width)
			}
		}
	}
}

func catalogHasBoundary(app *AppModel, name string) bool {
	for _, boundary := range app.runtime.Catalog().Boundaries {
		if boundary.Name == name {
			return true
		}
	}
	return false
}

func sliceContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
