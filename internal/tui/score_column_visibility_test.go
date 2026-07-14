package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

func TestOutlinerScoreColumnVisibility(t *testing.T) {
	task := mustNewScoreTestTask(t)

	t.Run("shows score when visible and profile exists", func(t *testing.T) {
		cfg := scoreTestConfig("proj-a")
		m := &OutlinerModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			columnConfig:      NewColumnConfig(map[string]bool{ColScore: true}, nil, DefaultOutlinerColumns, ""),
		}

		got := ansi.Strip(m.renderTaskContentGroup(task, &models.Project{ID: "proj-a"}))
		if !strings.Contains(got, "[S:") {
			t.Fatalf("expected score badge in outliner content group, got %q", got)
		}
	})

	t.Run("hides score when column hidden", func(t *testing.T) {
		cfg := scoreTestConfig("proj-a")
		m := &OutlinerModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			columnConfig:      NewColumnConfig(map[string]bool{ColScore: false}, nil, DefaultOutlinerColumns, ""),
		}

		got := ansi.Strip(m.renderTaskContentGroup(task, &models.Project{ID: "proj-a"}))
		if strings.Contains(got, "[S:") {
			t.Fatalf("expected no score badge when score column hidden, got %q", got)
		}
	})

	t.Run("cross-project uses owning project profile", func(t *testing.T) {
		cfg := scoreTestConfig("proj-a")
		m := &OutlinerModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			columnConfig:      NewColumnConfig(map[string]bool{ColScore: true}, nil, DefaultOutlinerColumns, ""),
		}

		withProfile := ansi.Strip(m.renderTaskContentGroup(task, &models.Project{ID: "proj-a"}))
		if !strings.Contains(withProfile, "[S:") {
			t.Fatalf("expected score badge for project with assigned profile, got %q", withProfile)
		}

		withoutProfile := ansi.Strip(m.renderTaskContentGroup(task, &models.Project{ID: "proj-b"}))
		if strings.Contains(withoutProfile, "[S:") {
			t.Fatalf("expected no score badge for project without profile, got %q", withoutProfile)
		}
	})
}

func TestProjectTreeScoreColumnVisibility(t *testing.T) {
	task := mustNewScoreTestTask(t)

	t.Run("shows score when visible and profile exists", func(t *testing.T) {
		cfg := scoreTestConfig("proj-a")
		m := &ProjectTreeModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			project:           &models.Project{ID: "proj-a"},
			sidebar:           NewSidebarComponent(),
			columnConfig:      NewColumnConfig(map[string]bool{ColScore: true}, nil, DefaultTreeColumns, ""),
		}

		got := ansi.Strip(m.nodeContentGroup(task))
		if !strings.Contains(got, "[S:") {
			t.Fatalf("expected score badge in project tree content group, got %q", got)
		}
	})

	t.Run("hides score when no profile assigned", func(t *testing.T) {
		cfg := scoreTestConfig()
		m := &ProjectTreeModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			project:           &models.Project{ID: "proj-a"},
			sidebar:           NewSidebarComponent(),
			columnConfig:      NewColumnConfig(map[string]bool{ColScore: true}, nil, DefaultTreeColumns, ""),
		}

		got := ansi.Strip(m.nodeContentGroup(task))
		if strings.Contains(got, "[S:") {
			t.Fatalf("expected no score badge without assigned profile, got %q", got)
		}
	})
}

func TestProjectTreeDescriptionColumnVisibility(t *testing.T) {
	task := mustNewScoreTestTask(t)
	task.Description = "Has details"
	descIndicator := ansi.Strip(renderDescriptionIndicator(task))
	if descIndicator == "" {
		t.Fatal("expected description indicator for task with description")
	}

	t.Run("shows description indicator when visible", func(t *testing.T) {
		m := &ProjectTreeModel{
			sidebar:      NewSidebarComponent(),
			columnConfig: NewColumnConfig(map[string]bool{ColDescription: true}, nil, DefaultTreeColumns, ""),
		}

		got := ansi.Strip(m.nodeContentGroup(task))
		if !strings.Contains(got, descIndicator) {
			t.Fatalf("expected description indicator when visible, got %q", got)
		}
	})

	t.Run("hides description indicator when column hidden", func(t *testing.T) {
		m := &ProjectTreeModel{
			sidebar:      NewSidebarComponent(),
			columnConfig: NewColumnConfig(map[string]bool{ColDescription: false}, nil, DefaultTreeColumns, ""),
		}

		got := ansi.Strip(m.nodeContentGroup(task))
		if strings.Contains(got, descIndicator) {
			t.Fatalf("expected description indicator hidden, got %q", got)
		}
	})
}

func scoreTestConfig(projectIDs ...string) *config.RuntimeState {
	cfg := newTestConfig()
	profile := scoring.NewICEProfile()
	cfg.Automation().ScoringProfiles = []scoring.ScoringProfile{profile}
	for _, projectID := range projectIDs {
		cfg.Automation().SetProjectScoringProfile(projectID, profile.ID)
	}
	return cfg
}

func mustNewScoreTestTask(t *testing.T) *models.Task {
	t.Helper()
	task, err := models.NewTask("score-test-task", 1, models.TaskTypeTask, "Score Task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	return task
}
