package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

func searchRecoveryDriver(t *testing.T, query string, submit bool) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureUXAcceptance, 60, 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	pressMenuBarKey(t, driver, "g", "i", "/")
	if _, err := driver.ApplyAction(UITestAction{Type: "text", Text: query}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if submit {
		pressMenuBarKey(t, driver, "enter")
	}
	return driver, driver.App().crossOutliner
}

func TestSearchRecoveryPickerAppliesEmptyAndPartialResultsAtTerminalFloor(t *testing.T) {
	for _, tc := range []struct {
		query, title, token string
		submit              bool
	}{
		{"retired staging", uxAcceptanceDoneTitle, "?completed", false},
		{"storage cleanup", uxAcceptanceDeferredTitle, "?deferred", true},
		{"the", uxAcceptanceDoneTitle, "?completed", true},
	} {
		t.Run(tc.query, func(t *testing.T) {
			driver, out := searchRecoveryDriver(t, tc.query, tc.submit)
			if tc.query == "the" && !out.HasContentRows() {
				t.Fatal("partial-result test has no visible results")
			}
			if !tc.submit {
				pressMenuBarKey(t, driver, "left", "left")
			}
			before := out.CaptureSearchInput()
			caret := out.SearchInputCursor()
			undo, redo := out.HistoryPreview()
			pressMenuBarKey(t, driver, "ctrl+g")
			assertHelpScreen(t, driver, "Search Recovery", "Explain Current View", "Enter run", "Esc close")
			assertHelpScreen(t, driver, "1 · "+tc.token)
			if sheet := driver.App().operations.ActionSheet; sheet == nil || len(sheet.OperationIDs()) < 2 {
				t.Fatal("recovery did not offer population and explanation actions")
			}
			pressMenuBarKey(t, driver, "esc")
			if !reflect.DeepEqual(before, out.CaptureSearchInput()) || out.SearchInputCursor() != caret {
				t.Fatal("cancel changed the query or input focus")
			}
			pressMenuBarKey(t, driver, "ctrl+g", "enter")
			if driver.App().operations.ActionSheet != nil || out.SearchQuery() != tc.query+" "+tc.token || out.IsSearchMode() {
				t.Fatalf("recovery did not apply: %q\n%s", out.SearchQuery(), driver.Render())
			}
			assertHelpScreen(t, driver, tc.token)
			if task, _ := out.GetTaskRef(); task == nil || task.Title != tc.title {
				t.Fatalf("recovery did not land on %q", tc.title)
			}
			if nextUndo, nextRedo := out.HistoryPreview(); nextUndo != undo || nextRedo != redo {
				t.Fatal("search recovery changed workspace history")
			}
			pressMenuBarKey(t, driver, "esc")
			if out.HasSearchProjection() || out.ActiveOutlinerMode() != chrome.ModeInbox {
				t.Fatal("Esc failed to restore the original Inbox")
			}
		})
	}
}

func TestSearchRecoveryExplainWorksWithoutWithheldMatches(t *testing.T) {
	driver, out := searchRecoveryDriver(t, "zzqx", false)
	before := out.CaptureSearchInput()
	pressMenuBarKey(t, driver, "ctrl+g")
	sheet := driver.App().operations.ActionSheet
	if sheet == nil || !slices.Equal(sheet.OperationIDs(), []operation.OperationID{operation.OperationCoreQueryReadout}) {
		t.Fatal("no-match recovery promised a population it cannot prove")
	}
	pressMenuBarKey(t, driver, "enter")
	if !out.QueryReadout.Expanded {
		t.Fatal("recovery did not open Explain Current View")
	}
	assertHelpScreen(t, driver, "zzqx")
	pressMenuBarKey(t, driver, "esc")
	if out.QueryReadout.Expanded || !reflect.DeepEqual(before, out.CaptureSearchInput()) {
		t.Fatal("explanation did not return to the search input")
	}
}

func TestSearchRecoveryActionsShareFindMenuAndHelpExecution(t *testing.T) {
	for _, route := range []string{"menu", "help"} {
		t.Run(route, func(t *testing.T) {
			driver, out := searchRecoveryDriver(t, "retired staging", true)
			if route == "help" {
				searchOutlinerHelp(t, driver, "Show Finished Matches")
				assertHelpScreen(t, driver, "Show Finished Matches", "Find", "Enter run")
			} else {
				pressMenuBarKey(t, driver, "f10", "right", "right", "right", "right")
				menus := openMenuBarMenusForTest(t, out)
				if menus[out.MenuBar.MenuIndex].ID != outliner.MenuIDFind {
					t.Fatal("did not reach Find")
				}
				assertHelpScreen(t, driver, "Show Finished Matches", "Search Recovery")
				pressMenuBarKey(t, driver, "down", "down")
			}
			pressMenuBarKey(t, driver, "enter")
			if out.SearchQuery() != "retired staging ?completed" {
				t.Fatalf("%s did not use recovery: %q", route, out.SearchQuery())
			}
			assertHelpScreen(t, driver, "?completed")
			pressMenuBarKey(t, driver, "esc")
			if out.HasSearchProjection() || out.ActiveOutlinerMode() != chrome.ModeInbox {
				t.Fatal("recovery overwrote the search's return frame")
			}
		})
	}
}

func TestSearchRecoveryDoesNotStealInputFromNestedControls(t *testing.T) {
	driver, out := searchRecoveryDriver(t, "retired staging", true)
	pressMenuBarKey(t, driver, "Q")
	if !out.QueryReadout.Expanded {
		t.Fatal("readout did not open")
	}
	pressMenuBarKey(t, driver, "ctrl+g")
	// An unlisted readout key closes it and is consumed, without falling
	// through to a root action beneath it.
	if out.QueryReadout.Expanded || driver.App().operations.ActionSheet != nil || out.SearchQuery() != "retired staging" {
		t.Fatal("recovery stole input from the readout")
	}
	pressMenuBarKey(t, driver, "/", "ctrl+r")
	if out.SearchInputOwnsInput() {
		t.Fatal("history did not take input")
	}
	pressMenuBarKey(t, driver, "ctrl+g")
	if out.SearchInputOwnsInput() || driver.App().operations.ActionSheet != nil {
		t.Fatal("recovery stole input from history")
	}
	pressMenuBarKey(t, driver, "esc", "ctrl+g")
	if driver.App().operations.ActionSheet == nil {
		t.Fatal("recovery remained inaccessible after closing history")
	}
}

func TestSearchRecoveryHelpExplainsAvailabilityAndRunsDeferredAction(t *testing.T) {
	driver, out := searchRecoveryDriver(t, "storage cleanup", true)
	searchOutlinerHelp(t, driver, "Show Deferred Matches")
	assertHelpScreen(t, driver, "Show Deferred Matches", "Find", "Enter run")
	pressMenuBarKey(t, driver, "enter")
	if out.SearchQuery() != "storage cleanup ?deferred" {
		t.Fatalf("help did not apply deferred recovery: %q", out.SearchQuery())
	}
	pressMenuBarKey(t, driver, "esc")
	searchOutlinerHelp(t, driver, "Show Finished Matches")
	assertHelpScreen(t, driver, "Enter a ranked task search with / first.")
	pressMenuBarKey(t, driver, "enter")
	assertHelpScreen(t, driver, "Show Finished Matches", "Enter a ranked task search with / first.")
}
