package tui

import (
	"strings"
	"testing"
)

func TestSavedFilterCapturesAndAppliesParked(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.projectTree = NewProjectTreeModel(project, app.configManager, app.runtime)
	app.projectTree.filter.Parked = true
	treeFilter := app.captureProjectTreeFilter("Tree Parked", "tree-filter")
	if !treeFilter.FilterParked {
		t.Fatal("project tree saved filter should capture parked filter")
	}
	app.projectTree.filter.Parked = false
	app.applySavedFilterToTree(treeFilter)
	if !app.projectTree.filter.Parked {
		t.Fatal("project tree saved filter should restore parked filter")
	}

	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.filter.Parked = true
	outlinerFilter := app.captureOutlinerFilter("Outliner Parked", "outliner-filter")
	if !outlinerFilter.FilterParked {
		t.Fatal("outliner saved filter should capture parked filter")
	}
	app.outliner.filter.Parked = false
	app.applySavedFilterToOutliner(outlinerFilter)
	if !app.outliner.filter.Parked {
		t.Fatal("outliner saved filter should restore parked filter")
	}

	crossApp := newTestAppModel()
	crossApp.crossOutliner = NewOutlinerModel(nil, crossApp.configManager, crossApp.runtime)
	crossApp.crossOutliner.filter.Parked = true
	crossFilter := crossApp.captureOutlinerFilterFrom("Cross Parked", "cross-filter", crossApp.crossOutliner)
	if !crossFilter.FilterParked {
		t.Fatal("cross-outliner saved filter should capture parked filter")
	}
	crossApp.crossOutliner.filter.Parked = false
	crossApp.applySavedFilterToCrossOutliner(crossFilter)
	if !crossApp.crossOutliner.filter.Parked {
		t.Fatal("cross-outliner saved filter should restore parked filter")
	}
}

func TestParkedFilterViewStatePersistence(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.projectTree = NewProjectTreeModel(project, app.configManager, app.runtime)
	app.projectTree.filter.Parked = true
	app.saveProjectTreeState()
	if !app.runtime.Machine().ProjectViewStates[project.ID].FilterParked {
		t.Fatal("project tree state should persist parked filter")
	}
	app.projectTree.filter.Parked = false
	app.restoreProjectTreeState()
	if !app.projectTree.filter.Parked {
		t.Fatal("project tree state should restore parked filter")
	}

	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.filter.Parked = true
	app.saveOutlinerFilterState()
	if !app.runtime.Machine().ProjectViewStates[project.ID].OutlinerFilterParked {
		t.Fatal("outliner state should persist parked filter")
	}
	app.outliner.filter.Parked = false
	app.restoreOutlinerFilterState()
	if !app.outliner.filter.Parked {
		t.Fatal("outliner state should restore parked filter")
	}
}

func TestCrossOutlinerParkedFilterPersistence(t *testing.T) {
	app := newTestAppModel()
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.filter.Parked = true

	app.saveCrossOutlinerState()

	if !app.runtime.Machine().CrossOutlinerFilterParked {
		t.Fatal("cross-outliner config should persist parked filter")
	}

	restored := NewOutlinerModel(nil, app.configManager, app.runtime)
	if !restored.filter.Parked {
		t.Fatal("cross-outliner config should restore parked filter")
	}
}

func TestSharedViewStatePreservesParkedFilter(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.projectTree = NewProjectTreeModel(project, app.configManager, app.runtime)
	app.projectTree.filter.Parked = true
	treeState := app.extractSharedStateFromTree()
	if treeState == nil || !treeState.Filter.Parked {
		t.Fatal("shared state should capture project tree parked filter")
	}

	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.applySharedStateToOutliner(treeState)
	if !app.outliner.filter.Parked {
		t.Fatal("shared state should apply parked filter to outliner")
	}

	app.outliner.filter.Parked = true
	outlinerState := app.extractSharedStateFromOutliner()
	app.projectTree.filter.Parked = false
	app.applySharedStateToTree(outlinerState)
	if !app.projectTree.filter.Parked {
		t.Fatal("shared state should apply parked filter to project tree")
	}
}

func TestBuildCurrentViewSummaryUsesCrossProjectOutlinerFilters(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.configManager, app.runtime)
	app.projectTree.filter.GitHubIssue = true
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.filter.Parked = true
	app.pushView(ViewCrossProjectOutliner)

	got := app.buildCurrentViewSummary()
	if !strings.Contains(got, "parked") {
		t.Fatalf("summary = %q, want cross-project parked filter", got)
	}
	if strings.Contains(got, "issue") {
		t.Fatalf("summary = %q, should not fall back to project tree filters", got)
	}
}
