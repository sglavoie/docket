package tui

import "testing"

func TestSavedFilterCapturesAndAppliesGitHubIssue(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)
	app.projectTree.filter.GitHubIssue = true
	treeFilter := app.captureProjectTreeFilter("Tree Issues", "tree-filter")
	if !treeFilter.FilterGitHubIssue {
		t.Fatal("project tree saved filter should capture GitHub issue filter")
	}
	app.projectTree.filter.GitHubIssue = false
	app.applySavedFilterToTree(treeFilter)
	if !app.projectTree.filter.GitHubIssue {
		t.Fatal("project tree saved filter should restore GitHub issue filter")
	}

	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
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
	crossApp.crossOutliner = NewOutlinerModel(nil, crossApp.config, crossApp.configManager)
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

	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)
	app.projectTree.filter.GitHubIssue = true
	app.saveProjectTreeState()
	if !app.config.ProjectViewStates[project.ID].FilterGitHubIssue {
		t.Fatal("project tree state should persist GitHub issue filter")
	}
	app.projectTree.filter.GitHubIssue = false
	app.restoreProjectTreeState()
	if !app.projectTree.filter.GitHubIssue {
		t.Fatal("project tree state should restore GitHub issue filter")
	}

	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.filter.GitHubIssue = true
	app.saveOutlinerFilterState()
	if !app.config.ProjectViewStates[project.ID].OutlinerFilterGitHubIssue {
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
	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.filter.GitHubIssue = true

	app.saveCrossOutlinerState()

	if !app.config.CrossOutlinerFilterGitHubIssue {
		t.Fatal("cross-outliner config should persist GitHub issue filter")
	}

	restored := NewOutlinerModel(nil, app.config, app.configManager)
	if !restored.filter.GitHubIssue {
		t.Fatal("cross-outliner config should restore GitHub issue filter")
	}
}

func TestSharedViewStatePreservesGitHubIssueFilter(t *testing.T) {
	app, project := newTestAppModelWithProject()

	app.projectTree = NewProjectTreeModel(project, app.config, app.configManager)
	app.projectTree.filter.GitHubIssue = true
	treeState := app.extractSharedStateFromTree()
	if treeState == nil || !treeState.Filter.GitHubIssue {
		t.Fatal("shared state should capture project tree GitHub issue filter")
	}

	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.applySharedStateToOutliner(treeState)
	if !app.outliner.filter.GitHubIssue {
		t.Fatal("shared state should apply GitHub issue filter to outliner")
	}

	app.outliner.filter.GitHubIssue = true
	outlinerState := app.extractSharedStateFromOutliner()
	app.projectTree.filter.GitHubIssue = false
	app.applySharedStateToTree(outlinerState)
	if !app.projectTree.filter.GitHubIssue {
		t.Fatal("shared state should apply GitHub issue filter to project tree")
	}
}
