package tui

import "testing"

func TestSavedFilterCapturesAndAppliesGitHubIssue(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.filter.GitHubIssue = true
	outlinerFilter := app.captureOutlinerFilter("Outliner Issues", "outliner-filter")
	if !outlinerFilter.FilterGitHubIssue {
		t.Fatal("outliner saved filter should capture GitHub issue filter")
	}
	app.outliner.filter.GitHubIssue = false
	app.applySavedFilterToOutliner(outlinerFilter)
	if !app.outliner.filter.GitHubIssue {
		t.Fatal("outliner saved filter should restore GitHub issue filter")
	}

	crossApp := newTestAppModel()
	crossApp.crossOutliner = NewOutlinerModel(nil, crossApp.configManager, crossApp.runtime)
	crossApp.crossOutliner.filter.GitHubIssue = true
	crossFilter := crossApp.captureOutlinerFilterFrom("Cross Issues", "cross-filter", crossApp.crossOutliner)
	if !crossFilter.FilterGitHubIssue {
		t.Fatal("cross-outliner saved filter should capture GitHub issue filter")
	}
	crossApp.crossOutliner.filter.GitHubIssue = false
	crossApp.applySavedFilterToCrossOutliner(crossFilter)
	if !crossApp.crossOutliner.filter.GitHubIssue {
		t.Fatal("cross-outliner saved filter should restore GitHub issue filter")
	}
}

func TestGitHubIssueFilterViewStatePersistence(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.filter.GitHubIssue = true
	app.saveOutlinerFilterState()
	if !app.runtime.Machine().ProjectViewStates[project.ID].OutlinerFilterGitHubIssue {
		t.Fatal("outliner state should persist GitHub issue filter")
	}
	app.outliner.filter.GitHubIssue = false
	app.restoreOutlinerFilterState()
	if !app.outliner.filter.GitHubIssue {
		t.Fatal("outliner state should restore GitHub issue filter")
	}
}

func TestCrossOutlinerGitHubIssueFilterPersistence(t *testing.T) {
	app := newTestAppModel()
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.filter.GitHubIssue = true

	app.saveCrossOutlinerState()

	if !app.runtime.Machine().CrossOutlinerFilterGitHubIssue {
		t.Fatal("cross-outliner config should persist GitHub issue filter")
	}

	restored := NewOutlinerModel(nil, app.configManager, app.runtime)
	if !restored.filter.GitHubIssue {
		t.Fatal("cross-outliner config should restore GitHub issue filter")
	}
}
