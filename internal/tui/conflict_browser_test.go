package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/storage"
)

// conflictBrowserFixture is one recorded bundle with both a field edit carrying
// three diverging values and a hierarchy conflict carrying only a detail line,
// so the detail pane's two shapes are both exercised.
func conflictBrowserFixture(id string, retryRequired bool) storage.ConflictRecord {
	return storage.ConflictRecord{
		ID:                     id,
		ProjectName:            "Rich Project",
		ProjectSlug:            "rich-project",
		Action:                 "update task",
		Context:                "renaming a task while the file moved",
		BaseRevision:           "b0b0b0b0b0b0b0b0b0",
		LocalRevision:          "1oca11oca11oca11o",
		RemoteRevision:         "",
		RequiresOperationRetry: &retryRequired,
		CreatedAt:              time.Date(2026, 3, 4, 9, 30, 15, 0, time.UTC),
		Conflicts: []storage.MergeConflict{
			{
				Kind:      storage.MergeConflictFieldEdit,
				TaskTitle: "Ship the release notes",
				Field:     "title",
				Base:      "Draft the release notes",
				Local:     "Ship the release notes",
				Current:   "Publish the release notes",
			},
			{
				Kind:   storage.MergeConflictHierarchy,
				Detail: "both sides reparented this task",
			},
		},
	}
}

func newTestConflictBrowser(t *testing.T, records ...storage.ConflictRecord) *ConflictBrowserModel {
	t.Helper()
	browser, err := NewConflictBrowserModel(records, func(id string) (*storage.ConflictBundle, error) {
		for _, record := range records {
			if record.ID == id {
				return &storage.ConflictBundle{Record: record}, nil
			}
		}
		return nil, fmt.Errorf("conflict %s not found", id)
	})
	if err != nil {
		t.Fatalf("build conflict browser: %v", err)
	}
	return browser
}

func pressConflictBrowserKey(
	t *testing.T,
	browser *ConflictBrowserModel,
	name string,
) (bool, tea.Msg) {
	t.Helper()
	key, err := parseUITestKey(name)
	if err != nil {
		t.Fatalf("parse key %q: %v", name, err)
	}
	_, dismissed, msg := browser.handleKey(key)
	return dismissed, msg
}

// TestConflictDetailLinesRenderTheCLIRecord holds the detail pane to `tt conflict
// show`'s field set and, above all, to its closing command: a retry-only bundle
// may only be cleared by adopting remote, and saying otherwise would invite a
// merge decision the store refuses.
func TestConflictDetailLinesRenderTheCLIRecord(t *testing.T) {
	record := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	body := strings.Join(conflictDetailLines(record), "\n")

	for _, want := range []string{
		"Conflict 11111111-2222-3333-4444-555555555555",
		"Project:   Rich Project (rich-project)",
		"Action:    update task",
		"Context:   renaming a task while the file moved",
		"Recorded:  ",
		"Revisions: base b0b0b0b0b0b0 · local 1oca11oca11o · remote <none>",
		"Retry:     required",
		"2 conflict(s):",
		`[1] field_edit — "Ship the release notes"`,
		"field:   title",
		"base:    Draft the release notes",
		"local:   Ship the release notes",
		"remote:  Publish the release notes",
		"[2] hierarchy",
		"detail:  both sides reparented this task",
		"Clear with: tt conflict resolve 11111111 --use remote",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("detail pane missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "Resolve with:") {
		t.Errorf("retry-only bundle offered a side to adopt\n%s", body)
	}
}

// TestConflictDetailLinesOfferEverySideWhenApplying is the other half of that
// rule: a bundle the resolver may apply names all three sides.
func TestConflictDetailLinesOfferEverySideWhenApplying(t *testing.T) {
	record := conflictBrowserFixture("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", false)
	body := strings.Join(conflictDetailLines(record), "\n")

	if want := "Resolve with: tt conflict resolve aaaaaaaa --use local|remote|file"; !strings.Contains(body, want) {
		t.Errorf("detail pane missing %q\n%s", want, body)
	}
	if strings.Contains(body, "Retry:") {
		t.Errorf("applying bundle claimed a retry requirement\n%s", body)
	}
}

// TestConflictBrowserListsInspectsAndUnwinds walks the whole overlay: the list
// names every bundle and says it cannot resolve them, Enter reads one, and Esc
// unwinds one level at a time.
func TestConflictBrowserListsInspectsAndUnwinds(t *testing.T) {
	first := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	second := conflictBrowserFixture("99999999-8888-7777-6666-555555555555", false)
	second.ProjectName = "Second Project"
	browser := newTestConflictBrowser(t, first, second)

	list := browser.render(120)
	for _, want := range []string{
		conflictBrowserTitle,
		conflictBrowserReadOnlyLine,
		"11111111",
		"Rich Project",
		"99999999",
		"Second Project",
		"2 conflicts",
	} {
		if !strings.Contains(list, want) {
			t.Errorf("conflict list missing %q\n%s", want, list)
		}
	}

	if dismissed, _ := pressConflictBrowserKey(t, browser, "enter"); dismissed {
		t.Fatal("Enter dismissed the browser; want the detail pane")
	}
	if !browser.inspecting() {
		t.Fatal("Enter did not open the detail pane")
	}
	detail := browser.render(120)
	if !strings.Contains(detail, "Conflict 11111111-2222-3333-4444-555555555555") {
		t.Errorf("detail pane did not render the inspected record\n%s", detail)
	}
	if strings.Contains(detail, conflictBrowserReadOnlyLine) {
		t.Errorf("detail pane kept the list's header line\n%s", detail)
	}

	if dismissed, _ := pressConflictBrowserKey(t, browser, "esc"); dismissed {
		t.Fatal("Esc closed the browser from the detail pane; want a return to the list")
	}
	if browser.inspecting() {
		t.Fatal("Esc did not leave the detail pane")
	}

	dismissed, msg := pressConflictBrowserKey(t, browser, "esc")
	if !dismissed {
		t.Fatal("Esc did not close the browser from the list")
	}
	if _, ok := msg.(ConflictBrowserClosedMsg); !ok {
		t.Fatalf("close message = %T, want ConflictBrowserClosedMsg", msg)
	}
}

// TestConflictBrowserScrollsTheDetailPane covers the one piece of state the pane
// owns. The closing command sits below the first window, so reaching it is the
// honest assertion that the pane scrolls at all.
func TestConflictBrowserScrollsTheDetailPane(t *testing.T) {
	record := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	browser := newTestConflictBrowser(t, record)

	pressConflictBrowserKey(t, browser, "enter")
	// Render inside a short terminal so the pane is genuinely windowed.
	browser.renderWithin(120, 20)
	if browser.detailHeight <= 0 {
		t.Fatal("detail pane recorded no height")
	}
	if browser.detailLines <= browser.detailHeight {
		t.Fatalf("detail pane fits in one window (%d lines, %d rows); nothing to scroll",
			browser.detailLines, browser.detailHeight)
	}

	pressConflictBrowserKey(t, browser, "G")
	tail := browser.renderWithin(120, 20)
	if !strings.Contains(tail, "Clear with: tt conflict resolve") {
		t.Errorf("end of the detail pane did not reach the closing command\n%s", tail)
	}

	pressConflictBrowserKey(t, browser, "home")
	if browser.detailOffset != 0 {
		t.Errorf("Home left the pane at offset %d, want 0", browser.detailOffset)
	}
	pressConflictBrowserKey(t, browser, "up")
	if browser.detailOffset != 0 {
		t.Errorf("scrolling above the top left offset %d, want 0", browser.detailOffset)
	}
}

// TestConflictBrowserRefusesAResolvedBundle is why inspecting reloads rather
// than reusing the listing: the CLI may have resolved the conflict in another
// terminal since the list was taken, and the row must say so instead of painting
// a record that is gone.
func TestConflictBrowserRefusesAResolvedBundle(t *testing.T) {
	record := conflictBrowserFixture("11111111-2222-3333-4444-555555555555", true)
	browser, err := NewConflictBrowserModel(
		[]storage.ConflictRecord{record},
		func(string) (*storage.ConflictBundle, error) {
			return nil, fmt.Errorf("conflict %s not found", record.ID)
		},
	)
	if err != nil {
		t.Fatalf("build conflict browser: %v", err)
	}

	dismissed, msg := pressConflictBrowserKey(t, browser, "enter")
	if dismissed {
		t.Fatal("a refused inspect closed the browser")
	}
	if browser.inspecting() {
		t.Fatal("a refused inspect opened the detail pane")
	}
	status, ok := msg.(StatusMsg)
	if !ok || !strings.Contains(string(status), "not found") {
		t.Fatalf("refusal message = %#v, want a StatusMsg naming the missing bundle", msg)
	}
	if !strings.Contains(browser.render(120), "not found") {
		t.Error("the refusal was not retained on the overlay")
	}
}

// TestConflictBrowserRequiresALoader keeps the one construction rule: a browser
// that cannot reload a bundle would silently show stale records.
func TestConflictBrowserRequiresALoader(t *testing.T) {
	if _, err := NewConflictBrowserModel(nil, nil); err == nil {
		t.Fatal("a browser without a loader was accepted")
	}
}

// TestOpenConflictBrowserReadsTheWorkspaceStore covers the host port both ways:
// an empty store refuses with a status and mounts nothing, and a recorded bundle
// mounts the overlay and settles the strip's count in the same read.
func TestOpenConflictBrowserReadsTheWorkspaceStore(t *testing.T) {
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	driver, err := workspace.launch(120, 40)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	app := driver.App()

	app.openConflictBrowser()
	if app.conflictBrowser != nil {
		t.Fatal("an empty workspace opened the conflict browser")
	}
	if !strings.Contains(app.statusMessage, conflictBrowserEmptyStatus) {
		t.Errorf("empty-store status = %q, want %q", app.statusMessage, conflictBrowserEmptyStatus)
	}

	if err := app.harnessRecordConflict(); err != nil {
		t.Fatalf("record conflict: %v", err)
	}
	app.openConflictBrowser()
	if app.conflictBrowser == nil {
		t.Fatal("a recorded conflict did not open the browser")
	}
	if app.outstandingConflictCount() != 1 {
		t.Errorf("outstanding conflicts = %d, want 1", app.outstandingConflictCount())
	}
	record, ok := app.conflictBrowser.CurrentRecord()
	if !ok {
		t.Fatal("the browser opened on no record")
	}
	if record.ProjectName != harnessConflictProjectName {
		t.Errorf("cursor record project = %q, want %q", record.ProjectName, harnessConflictProjectName)
	}
}

// TestConflictBrowserOpensFromTheGoToChord keeps gc a terminal path under the
// existing go-to prefix, which is what leaves root `c` retired.
func TestConflictBrowserOpensFromTheGoToChord(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	binding, _, resolution := registry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"g", "c"},
		ctx,
	)
	if resolution != ChordTerminal || binding.OperationID != operationCoreConflicts {
		t.Fatalf("gc resolved to %s (resolution %v), want %s",
			binding.OperationID, resolution, operationCoreConflicts)
	}
	if _, _, ok := registry.InputBindingForKey(InputStrategySimplified, "c", ctx); ok {
		t.Error("root c is bound; it must stay retired")
	}
}
