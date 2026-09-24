package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// This file is the integration half of project comments: not whether one
// mutation is correct, but whether the panel and the stored collection survive
// everything else the product does around them.
//
// Two questions are asked here and nowhere else. A draft is unsaved text that
// lives only in the panel, so every refresh the shell can run under it — an
// external write, a rename, an archival, a deletion — is a chance to lose it,
// and losing it is silent. And a project comment shares one tasks.md with the
// whole task list, so every use case that rewrites that document is a chance to
// drop the block: an inline task edit, an outline plan, a cross-project move,
// and a project rename each rewrite it through a different call.

// openProjectCommentsDraft mounts the panel on the fixture project's header and
// leaves an unsaved add-mode draft in it, which is the state every refresh case
// below perturbs.
func openProjectCommentsDraft(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: draft},
	})
	assertDraftIntact(t, driver, draft)
}

// openProjectCommentEditDraft commits one comment and then leaves an unsaved edit
// of it open, which is the other draft a refresh can land under. It is the harder
// one to keep: the add composer only holds text nobody has ever stored, while this
// buffer is a replacement for something that exists, and the panel has to be able
// to say so without either writing it or throwing it away.
func openProjectCommentEditDraft(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: seededProjectComment},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
		{Type: UITestActionKey, Key: "e"},
	})
	panel := mountedProjectCommentsPanel(t, driver.App())
	if !panel.EditMode {
		t.Fatalf("e did not open the edit composer over the committed comment:\n%s", driver.Render())
	}
	panel.EditTextInput.SetValue(draft)
	assertDraftIntact(t, driver, draft)
}

// seededProjectComment is the one comment the edit case rewrites. It is committed
// through the panel's own keys, so what the edit draft replaces is a comment the
// stored document really holds.
const seededProjectComment = "Committed before the removal"

// panelDraft reads whichever composer the panel has open. One accessor serves both
// drafts because what the user can still reach is the buffer of the composer that
// is on screen, and a panel with neither open has no draft left to lose.
func panelDraft(panel *taskdetail.CommentsModel) (string, bool) {
	switch {
	case panel.AddMode:
		return panel.TextInput.Value(), true
	case panel.EditMode:
		return panel.EditTextInput.Value(), true
	default:
		return "", false
	}
}

// assertDraftIntact is the claim every refresh owes: the panel is still the one
// the user was typing into, it is still drawing their text, and the text is
// exactly what they typed. The rendered half is not redundant — a composer that
// holds the buffer and stops painting it is a draft the user cannot reach.
func assertDraftIntact(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	got, open := panelDraft(panel)
	if !open {
		t.Fatalf("the panel is not holding a draft:\n%s", driver.Render())
	}
	if got != draft {
		t.Fatalf("draft = %q, want the untouched %q", got, draft)
	}
	if screen := driver.Render(); !strings.Contains(screen, draft) {
		t.Fatalf("the panel holds the draft but does not draw it:\n%s", screen)
	}
}

// externalProjectCommentWriter opens a second application service over the same
// workspace directory, which is what another surface — or another machine — is
// from this shell's point of view: a writer whose commits this process learns
// about only by reloading.
func externalProjectCommentWriter(t *testing.T, workspace *uitestWorkspace) *app.Service {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(workspace.DataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", workspace.DataDir, err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", workspace.DataDir, err)
	}
	return newTUIAppService(workspace.DataDir, manager.RuntimeDependencies(), workspaceLedgerFromConfig(runtime))
}

// refreshUnderThePanel runs the one fan-out every committed mutation ends in,
// which is also the only path that can dismiss an open panel, and then delivers
// everything it queued.
//
// The delivery is the point. A refresh that wants the panel gone says so with a
// command carrying BackMsg, and the pop happens when the shell handles that
// message: a case that stopped at the returned slice would be reading a panel that
// is on its way out as though it had stayed. Running the commands through the shell
// is what makes the route afterwards the answer.
func refreshUnderThePanel(t *testing.T, driver *UITestDriver) {
	t.Helper()
	for _, cmd := range driver.App().refreshTaskViews(true, false) {
		runSession2AppCommand(t, driver.App(), cmd)
	}
}

// assertPanelIsStillMounted reads that answer. Only the route can give it: a
// popped panel leaves its model behind intact, so its own fields say nothing about
// whether the user can still see it.
func assertPanelIsStillMounted(t *testing.T, driver *UITestDriver) {
	t.Helper()
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("the refresh left the route on %s, want the Comments panel:\n%s", got, driver.Render())
	}
}

// removeProjectFromShell drops one project out of the live catalog the panel's
// adapter resolves through, which is what a committed delete — or an archive whose
// reload no longer lists it — leaves behind for an open panel.
func removeProjectFromShell(shell *AppModel, projectID string) {
	for id, project := range shell.domain.projects {
		if project != nil && project.ID == projectID {
			delete(shell.domain.projects, id)
		}
	}
}

// TestProjectCommentsPanelKeepsItsDraftAcrossAnExternalRefresh is the case the
// adapter's identity resolution exists for. Another writer's comment reaches the
// list, and the text the user has not saved is not what pays for it.
func TestProjectCommentsPanelKeepsItsDraftAcrossAnExternalRefresh(t *testing.T) {
	driver, workspace, _ := addCommentFixtureApp(t)
	const draft = "Half-typed, and not ready to save"
	openProjectCommentsDraft(t, driver, draft)

	owner := driver.App().projectByID(projectCommentFixtureSlug)
	if owner == nil {
		t.Fatal("the fixture project is not in the shell cache")
	}
	writer := externalProjectCommentWriter(t, workspace)
	if _, err := writer.AddProjectComment(owner.ID, "Written by another surface"); err != nil {
		t.Fatalf("the external writer could not comment: %v", err)
	}

	driver.App().reloadCrossProjectOutlinerProject(owner.ID)
	refreshUnderThePanel(t, driver)
	assertPanelIsStillMounted(t, driver)

	assertDraftIntact(t, driver, draft)
	if got := panelCommentTexts(mountedProjectCommentsPanel(t, driver.App())); !reflect.DeepEqual(
		got, []string{"Written by another surface"}) {
		t.Fatalf("the refreshed panel lists %v, want the external comment", got)
	}

	// And the draft is still a draft: saving it now appends beside the comment
	// that arrived rather than replacing it.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 2 || stored[0].Text != "Written by another surface" || stored[1].Text != draft {
		t.Fatalf("stored comments = %+v, want the external one then the draft", stored)
	}
}

// TestProjectCommentsPanelKeepsItsDraftWhenTheOwnerChanges walks the three
// remaining ways the workspace moves under an open draft. A rename, an archival,
// and a deletion all leave the panel where it is, because what the user has typed
// is only reachable there: the first two also leave the owner writable or refusing
// on its own terms, and the third refuses every write while still owing them the
// text and the reason.
func TestProjectCommentsPanelKeepsItsDraftWhenTheOwnerChanges(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		draft    string
		perturb  func(t *testing.T, shell *AppModel, owner *models.Project)
		expects  string
		refusing bool
	}{
		{
			name:    "renamed under the panel",
			draft:   "Typed before the rename",
			perturb: func(_ *testing.T, _ *AppModel, owner *models.Project) { owner.Name = "Renamed Under The Draft" },
			expects: "Renamed Under The Draft",
		},
		{
			name:  "archived under the panel",
			draft: "Typed before the archival",
			// The archive is committed rather than flipped in memory, because the
			// refusal below is the application layer's and it rules on the catalog
			// it loads, not on the flag this shell happens to be holding.
			perturb: func(t *testing.T, shell *AppModel, owner *models.Project) {
				handled, cmd := shell.handleProjectMessages(projecthub.ProjectArchivedMsg{Project: owner})
				if !handled {
					t.Fatal("the shell did not handle the archive")
				}
				runSession2AppCommand(t, shell, cmd)
				if reloaded := shell.projectByID(owner.ID); reloaded == nil || !reloaded.Archived {
					t.Fatalf("the archive did not stick: %+v", reloaded)
				}
			},
			expects:  checklistFixtureProjectName,
			refusing: true,
		},
		{
			name:  "deleted under the panel",
			draft: "Typed before the deletion",
			perturb: func(_ *testing.T, shell *AppModel, owner *models.Project) {
				removeProjectFromShell(shell, owner.ID)
			},
			// The owner is gone rather than merely unwritable, so the panel names that
			// instead of the project: the draft is still the user's, and the sentence
			// beside it is what says why nothing will accept it.
			expects:  "no longer available",
			refusing: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			driver, workspace, _ := addCommentFixtureApp(t)
			openProjectCommentsDraft(t, driver, testCase.draft)

			owner := driver.App().projectByID(projectCommentFixtureSlug)
			if owner == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			testCase.perturb(t, driver.App(), owner)

			refreshUnderThePanel(t, driver)
			assertPanelIsStillMounted(t, driver)
			assertDraftIntact(t, driver, testCase.draft)
			if !strings.Contains(driver.Render(), testCase.expects) {
				t.Fatalf("the panel does not name %q:\n%s", testCase.expects, driver.Render())
			}
			if !testCase.refusing {
				return
			}

			// An unwritable owner — archived, or gone outright — stays readable and
			// refuses the write, and the refusal is what must not cost the draft either.
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "ctrl+s"},
				{Type: UITestActionWait},
			})
			if got := projectCommentsOnDisk(t, workspace); len(got) != 0 {
				t.Fatalf("a refused owner still stored %+v", got)
			}
			assertDraftIntact(t, driver, testCase.draft)
		})
	}
}

// TestProjectCommentsPanelRetainsDraftAfterOwnerRemoval is the removal case in
// full, and it is the one refresh that used to end the panel outright.
//
// A committed delete leaves the shell holding a catalog the panel's project is not
// in, and the refresh that follows every commit is what the panel learns it from.
// Nobody asked for anything at that moment, so there is no refusal to report and
// nothing on screen to explain a pop: the view is simply gone, the discard question
// the composer owes is never asked, and the buffer goes with it. The two drafts
// below are therefore held to all three halves at once — the panel is still
// reachable, it still draws the exact text, and it says why that text will not be
// written — while the document keeps saying exactly what it said before.
//
// The keys pressed after the refresh are part of that claim rather than extra
// coverage. The shell also unwinds to the Outliner root when the project it has
// loaded goes missing, and it does that on the next keystroke, so a draft the
// refresh kept but the following Ctrl+S threw away would be kept for nothing.
func TestProjectCommentsPanelRetainsDraftAfterOwnerRemoval(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		draft  string
		open   func(t *testing.T, driver *UITestDriver, draft string)
		stored []string
	}{
		{
			name:  "add_draft",
			draft: "Typed into the add composer",
			open:  openProjectCommentsDraft,
		},
		{
			name:   "edit_draft",
			draft:  seededProjectComment + ", rewritten",
			open:   openProjectCommentEditDraft,
			stored: []string{seededProjectComment},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			driver, workspace, _ := addCommentFixtureApp(t)
			testCase.open(t, driver, testCase.draft)
			assertStoredCommentTexts(t, workspace, "before the removal", testCase.stored)

			owner := driver.App().projectByID(projectCommentFixtureSlug)
			if owner == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			removeProjectFromShell(driver.App(), owner.ID)

			refreshUnderThePanel(t, driver)
			assertPanelIsStillMounted(t, driver)
			assertDraftIntact(t, driver, testCase.draft)
			assertMissingOwnerReported(t, driver)
			// Reading the removed owner is not resolving it: the catalog the shell
			// serves must still be missing the project the panel is drawing.
			if resolved := driver.App().projectByID(projectCommentFixtureSlug); resolved != nil {
				t.Fatalf("the refresh put the removed project back: %+v", resolved)
			}
			assertStoredCommentTexts(t, workspace, "the refresh", testCase.stored)

			// Saving is refused for the same reason, and the refusal costs neither the
			// draft nor the document.
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "ctrl+s"},
				{Type: UITestActionWait},
			})
			assertPanelIsStillMounted(t, driver)
			assertDraftIntact(t, driver, testCase.draft)
			assertMissingOwnerReported(t, driver)
			assertStoredCommentTexts(t, workspace, "a refused save", testCase.stored)

			// And the draft still leaves the one way a draft leaves: Esc asks, and only
			// the answer discards it.
			runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "esc"}})
			panel := mountedProjectCommentsPanel(t, driver.App())
			if !panel.IsConfirmActive() {
				t.Fatalf("Esc over the retained draft did not ask before discarding it:\n%s", driver.Render())
			}
			runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "y"}})
			if _, open := panelDraft(mountedProjectCommentsPanel(t, driver.App())); open {
				t.Fatalf("a confirmed discard kept the composer:\n%s", driver.Render())
			}
			assertStoredCommentTexts(t, workspace, "a confirmed discard", testCase.stored)
		})
	}
}

// assertMissingOwnerReported holds the half a dismissal used to swallow: the panel
// carries the missing-owner refusal and the user can read it beside their text.
// The wording is the panel's own, so the assertion asks the model for it rather
// than restating a sentence that lives in another package.
func assertMissingOwnerReported(t *testing.T, driver *UITestDriver) {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	if panel.Err == nil {
		t.Fatalf("the panel reports no refusal for the removed owner:\n%s", driver.Render())
	}
	if !strings.Contains(panel.Err.Error(), "no longer available") {
		t.Fatalf("the refusal %q does not name the missing owner", panel.Err)
	}
	if screen := driver.Render(); !strings.Contains(screen, panel.Err.Error()) {
		t.Fatalf("the refusal %q is not on screen:\n%s", panel.Err, screen)
	}
}

// assertStoredCommentTexts names the stored collection by text in stored order.
// Not a count: a draft that landed and a comment that was rewritten both keep the
// count the document already had.
func assertStoredCommentTexts(t *testing.T, workspace *uitestWorkspace, after string, want []string) {
	t.Helper()
	stored := projectCommentsOnDisk(t, workspace)
	got := make([]string, 0, len(stored))
	for _, comment := range stored {
		if comment != nil {
			got = append(got, comment.Text)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("after %s the stored comments are %q, want %q", after, got, want)
	}
}

// projectCommentTextsOnDisk reads one named project's stored comments, so a run
// that renames a project or moves a task out of one can read either document.
func projectCommentTextsOnDisk(t *testing.T, workspace *uitestWorkspace, slug string) []string {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", slug, "tasks.md")
	content, err := os.ReadFile(path) //nolint:gosec // The path is this test's own workspace.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	texts := make([]string, 0, len(document.Comments))
	for _, comment := range document.Comments {
		if comment != nil {
			texts = append(texts, comment.Text)
		}
	}
	return texts
}

// selectProjectPickerRow walks the open value picker onto the row the caller
// names, which is how the `%` move chooses a destination without depending on
// the fixture's project order.
func selectProjectPickerRow(t *testing.T, driver *UITestDriver, label string) {
	t.Helper()
	for range 32 {
		for _, line := range strings.Split(driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
			if strings.Contains(line, "\u203a") && strings.Contains(line, label) {
				return
			}
		}
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "down"}})
	}
	t.Fatalf("no picker row named %q:\n%s", label, driver.Render())
}

// seatCursorOnProjectHeaderFor parks the cursor on one project's header in the
// cross-project view, which is where the project half of `e` answers.
func seatCursorOnProjectHeaderFor(t *testing.T, driver *UITestDriver, projectID string) {
	t.Helper()
	view := driver.App().crossOutliner
	// A grouping projection draws the header only once the group is folded, and a
	// scoped one leads with a focused header no fold produces, so the fold is
	// what a miss spends rather than what the seat opens with. The launch's
	// Backlog fold can hide the header too, so the first miss opens every fold.
	for attempt := range 3 {
		switch attempt {
		case 1:
			view.ClearAllFolds()
		case 2:
			foldEveryProjectGroup(t, view)
		}
		for index, row := range view.Rows {
			if row.IsProjectHeader && row.ProjectID == projectID {
				view.Cursor = index
				return
			}
		}
		if attempt > 0 && len(view.VisibleProjectGroups()) == 0 {
			break
		}
	}
	t.Fatalf("no project header for %q:\n%s", projectID, driver.Render())
}

// TestProjectCommentsSurviveTheOrdinaryTaskWorkBesideThem is the document half.
// Every act below rewrites the whole of the tasks.md the comments live in, each
// through a different call, and none of those calls is about comments. One of
// them dropping the block would stay invisible until the next launch.
func TestProjectCommentsSurviveTheOrdinaryTaskWorkBesideThem(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	shell := driver.App()

	const (
		sourceSlug  = "client-alpha"
		targetSlug  = "client-beta"
		sourceTask  = "Alpha migration kickoff"
		rescheduled = sourceTask + " rescheduled"
	)
	source := shell.projectByID(sourceSlug)
	if source == nil {
		t.Fatal("the fixture's source project is not in the shell cache")
	}
	want := []string{"Kickoff moved to Monday", "Room four is booked"}
	for _, text := range want {
		seatCursorOnProjectHeaderFor(t, driver, source.ID)
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "C"},
			{Type: UITestActionText, Text: text},
			{Type: UITestActionKey, Key: "ctrl+s"},
			{Type: UITestActionWait},
		})
	}
	if got := projectCommentTextsOnDisk(t, workspace, sourceSlug); !reflect.DeepEqual(got, want) {
		t.Fatalf("seeded comments = %v, want %v", got, want)
	}

	assertCommentsIntact := func(t *testing.T, slug, after string) {
		t.Helper()
		if got := projectCommentTextsOnDisk(t, workspace, slug); !reflect.DeepEqual(got, want) {
			t.Fatalf("after %s the stored comments are %v, want %v", after, got, want)
		}
	}

	// An inline task edit rewrites the document through the single task-line
	// path, which is the narrowest write the product has.
	seatCursorOnTaskTitled(t, shell.crossOutliner, sourceTask)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: " !1 "},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	assertCommentsIntact(t, sourceSlug, "an inline task edit")

	// An outline plan rewrites the whole task list at once, which is the widest.
	// The buffer travels the production path from the dump to the apply; only the
	// editor process in the middle is missing, because the harness has none.
	project, revision, buffer, scopeTaskIDs, err := shell.outlineDumpSource(source.ID, "")
	if err != nil {
		t.Fatalf("dump the outline: %v", err)
	}
	edited := strings.Replace(buffer, sourceTask, rescheduled, 1)
	if edited == buffer {
		t.Fatalf("the outline buffer does not name %q:\n%s", sourceTask, buffer)
	}
	bufferPath := filepath.Join(t.TempDir(), "outline.md")
	if err = os.WriteFile(bufferPath, []byte(edited), 0o600); err != nil {
		t.Fatalf("stage the outline buffer: %v", err)
	}
	runSession2AppCommand(t, shell, shell.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: revision, ScopeTaskIDs: scopeTaskIDs,
		Path: bufferPath, Buffer: edited,
	}))
	assertCommentsIntact(t, sourceSlug, "an outline edit")

	// A cross-project move commits both documents, and the comments belong to
	// neither task: the source keeps them and the destination gains none.
	seatCursorOnTaskTitled(t, shell.crossOutliner, rescheduled)
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "%"}})
	selectProjectPickerRow(t, driver, "Client Beta")
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	if got := projectCommentTextsOnDisk(t, workspace, targetSlug); len(got) != 0 {
		t.Fatalf("the move carried project comments into the destination: %v", got)
	}
	assertCommentsIntact(t, sourceSlug, "a cross-project move")

	// And a rename rewrites the document at a new path entirely, which is the one
	// act that moves the block rather than rewriting it in place.
	//
	// The move left the source holding no tasks, so All prints no group header for
	// it any more and the rename is reached the way the product reaches an empty
	// project at all: scope onto it, and edit the one focused header row it leads
	// with.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "P"},
		{Type: UITestActionKey, Key: "/"},
		{Type: UITestActionText, Text: "alpha"},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	seatCursorOnProjectHeaderFor(t, driver, source.ID)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: " Renamed"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	assertCommentsIntact(t, "client-alpha-renamed", "a project rename")
}
