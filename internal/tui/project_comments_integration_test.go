package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/projecthub"
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
