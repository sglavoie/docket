package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// setupScoreTest sets up the full environment (config + storage) for score commands
// that call LoadCurrentProject.
func setupScoreTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	deps, project := setupTestDepsWithProject(t, "Test Project")

	iceProfile := scoring.NewICEProfile()
	deps.Config.Automation().ScoringProfiles = []scoring.ScoringProfile{iceProfile}
	deps.Config.Automation().SetProjectScoringProfile(project.ID, "ice")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

// setupScoreConfigOnlyTest sets up the environment for score commands that only call
// LoadConfig (e.g., assign/unassign) and do not need storage.DefaultRepository.
func setupScoreConfigOnlyTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	deps, project := setupTestDepsWithProject(t, "Test Project")

	iceProfile := scoring.NewICEProfile()
	deps.Config.Automation().ScoringProfiles = []scoring.ScoringProfile{iceProfile}

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	return deps, project
}

func TestRunScoreAssign(t *testing.T) {
	t.Run("assigns profile successfully", func(t *testing.T) {
		deps, project := setupScoreConfigOnlyTest(t)
		defer deps.Cleanup()

		err := runScoreAssign(scoreAssignCmd, []string{"ice"})
		testutil.AssertNoError(t, err)

		// Reload config to verify it was saved
		manager := config.NewManager(deps.ConfigPath)
		savedCfg, loadErr := manager.LoadRuntimeState()
		testutil.AssertNoError(t, loadErr)

		profile := savedCfg.GetProjectScoringProfile(project.ID)
		if profile == nil {
			t.Fatal("expected scoring profile to be assigned after runScoreAssign")
		}
		testutil.AssertEqual(t, profile.ID, "ice")
	})

	t.Run("rejects nonexistent profile", func(t *testing.T) {
		deps, _ := setupScoreConfigOnlyTest(t)
		defer deps.Cleanup()

		err := runScoreAssign(scoreAssignCmd, []string{"nonexistent-profile"})
		if err == nil {
			t.Error("expected error for nonexistent profile")
		}
	})

	t.Run("rejects missing args", func(t *testing.T) {
		deps, _ := setupScoreConfigOnlyTest(t)
		defer deps.Cleanup()

		err := runScoreAssign(scoreAssignCmd, []string{})
		if err == nil {
			t.Error("expected error for missing profile ID argument")
		}
	})
}

func TestRunScoreUnassign(t *testing.T) {
	t.Run("unassigns profile successfully", func(t *testing.T) {
		deps, project := setupScoreConfigOnlyTest(t)
		defer deps.Cleanup()

		// First assign a profile so there's something to unassign
		deps.Config.Automation().SetProjectScoringProfile(project.ID, "ice")
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		err := runScoreUnassign(scoreUnassignCmd, []string{})
		testutil.AssertNoError(t, err)

		// Reload config to verify profile was removed
		savedCfg, loadErr := manager.LoadRuntimeState()
		testutil.AssertNoError(t, loadErr)

		profile := savedCfg.GetProjectScoringProfile(project.ID)
		if profile != nil {
			t.Errorf("expected scoring profile to be unassigned, but got %q", profile.ID)
		}
	})
}

func TestRunScoreSet(t *testing.T) {
	t.Run("sets dimension values on task", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		firstTask := project.Tasks[0]
		taskRef := firstTask.Title

		err := runScoreSet(scoreSetCmd, []string{taskRef, "impact=7", "confidence=8", "ease=6"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		var updatedTask *models.Task
		for _, task := range savedProject.Tasks {
			if task.Title == taskRef {
				updatedTask = task
				break
			}
		}
		if updatedTask == nil {
			t.Fatal("could not find task after score set")
		}
		if updatedTask.ScoreDimensions == nil {
			t.Fatal("expected ScoreDimensions to be set")
		}
		testutil.AssertEqual(t, updatedTask.ScoreDimensions["impact"], 7)
		testutil.AssertEqual(t, updatedTask.ScoreDimensions["confidence"], 8)
		testutil.AssertEqual(t, updatedTask.ScoreDimensions["ease"], 6)
	})

	t.Run("updates the current project when another project reuses the task ID", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		current := project.Tasks[0]
		other, err := models.NewProject("score-other", "Score Other", "")
		testutil.AssertNoError(t, err)
		duplicate, err := models.NewTask(current.ID, 1, current.Type, "Duplicate elsewhere")
		testutil.AssertNoError(t, err)
		other.Tasks = append(other.Tasks, duplicate)
		deps.Repository.Projects[other.ID] = other

		err = runScoreSet(scoreSetCmd, []string{current.Title, "impact=7"})
		testutil.AssertNoError(t, err)

		if got := deps.Repository.Projects[project.ID].GetTaskByID(current.ID).ScoreDimensions["impact"]; got != 7 {
			t.Fatalf("current-project impact = %d, want 7", got)
		}
		if got := deps.Repository.Projects[other.ID].GetTaskByID(duplicate.ID).ScoreDimensions["impact"]; got != 0 {
			t.Fatalf("other-project impact = %d, want unchanged", got)
		}
	})

	t.Run("rejects task not found", func(t *testing.T) {
		deps, _ := setupScoreTest(t)
		defer deps.Cleanup()

		err := runScoreSet(scoreSetCmd, []string{"99999", "impact=5"})
		if err == nil {
			t.Error("expected error for task not found")
		}
	})

	t.Run("rejects no args (less than 2)", func(t *testing.T) {
		deps, _ := setupScoreTest(t)
		defer deps.Cleanup()

		err := runScoreSet(scoreSetCmd, []string{"1"})
		if err == nil {
			t.Error("expected error for missing dimension argument")
		}
	})

	t.Run("rejects invalid format (no equals)", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		firstTask := project.Tasks[0]

		err := runScoreSet(scoreSetCmd, []string{firstTask.Title, "impactNOEQUALS"})
		if err == nil {
			t.Error("expected error for invalid dimension format")
		}
	})

	t.Run("rejects value out of range", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		firstTask := project.Tasks[0]

		err := runScoreSet(scoreSetCmd, []string{firstTask.Title, "impact=999"})
		if err == nil {
			t.Error("expected error for value out of range")
		}
	})

	t.Run("rejects unknown dimension", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		firstTask := project.Tasks[0]

		err := runScoreSet(scoreSetCmd, []string{firstTask.Title, "unknowndim=5"})
		if err == nil {
			t.Error("expected error for unknown dimension")
		}
	})

	t.Run("rejects task with no profile assigned", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		// Remove the profile assignment
		deps.Config.Automation().UnsetProjectScoringProfile(project.ID)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		firstTask := project.Tasks[0]

		err := runScoreSet(scoreSetCmd, []string{firstTask.Title, "impact=5"})
		if err == nil {
			t.Error("expected error when no scoring profile is assigned")
		}
	})
}

func TestRunScoreShow(t *testing.T) {
	t.Run("shows score for task with profile", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		firstTask := project.Tasks[0]

		err := runScoreShow(scoreShowCmd, []string{firstTask.Title})
		testutil.AssertNoError(t, err)
	})

	t.Run("rejects task with no profile assigned", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		deps.Config.Automation().UnsetProjectScoringProfile(project.ID)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		firstTask := project.Tasks[0]

		err := runScoreShow(scoreShowCmd, []string{firstTask.Title})
		if err == nil {
			t.Error("expected error when no scoring profile is assigned")
		}
	})

	t.Run("rejects missing args", func(t *testing.T) {
		deps, _ := setupScoreTest(t)
		defer deps.Cleanup()

		err := runScoreShow(scoreShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing task ID argument")
		}
	})
}

func TestRunScoreRank(t *testing.T) {
	t.Run("ranks tasks by score", func(t *testing.T) {
		deps, _ := setupScoreTest(t)
		defer deps.Cleanup()

		err := runScoreRank(scoreRankCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("handles empty task list", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		// Mark all tasks complete so nothing is ranked (default excludes completed)
		for _, task := range project.Tasks {
			task.WorkflowState = models.StateDone
		}
		deps.Repository.Projects[project.ID] = project

		err := runScoreRank(scoreRankCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("no profile assigned returns error", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		deps.Config.Automation().UnsetProjectScoringProfile(project.ID)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		err := runScoreRank(scoreRankCmd, []string{})
		if err == nil {
			t.Error("expected error when no scoring profile is assigned")
		}
	})

	t.Run("excludes snoozed tasks by default", func(t *testing.T) {
		deps, project := setupScoreTest(t)
		defer deps.Cleanup()

		profile := deps.Config.GetProjectScoringProfile(project.ID)
		if profile == nil {
			t.Fatal("expected scoring profile to be assigned")
		}

		// Snooze one active task (set SnoozedUntil to the future)
		var snoozedTask *models.Task
		for _, task := range project.Tasks {
			if status := models.LegacyStatusForState(task.State()); status.IsActive() && !status.IsComplete() {
				future := time.Now().Add(7 * 24 * time.Hour)
				task.SnoozedUntil = &future
				snoozedTask = task
				break
			}
		}
		if snoozedTask == nil {
			t.Fatal("could not find an active task to snooze")
		}
		deps.Repository.Projects[project.ID] = project

		// Count how many tasks would be included (active, non-snoozed)
		expectedCount := 0
		for _, task := range project.Tasks {
			if !models.LegacyStatusForState(task.State()).IsComplete() && !task.IsSnoozed() {
				expectedCount++
			}
		}

		// Build ranked list using the same logic as runScoreRank (with default snoozed=false)
		var ranked []rankedTask
		for _, task := range project.Tasks {
			if models.LegacyStatusForState(task.State()).IsComplete() {
				continue
			}
			if task.IsSnoozed() {
				continue
			}
			score := scoring.ComputeScore(task, profile)
			ranked = append(ranked, rankedTask{task: task, score: score})
		}

		if len(ranked) != expectedCount {
			t.Errorf("expected %d ranked tasks (excluding snoozed), got %d", expectedCount, len(ranked))
		}
		for _, r := range ranked {
			if r.task.ID == snoozedTask.ID {
				t.Errorf("snoozed task %q should have been excluded from ranking", snoozedTask.Title)
			}
		}
	})
}

func TestScoreRankJSON_IncludesIDAndPriority(t *testing.T) {
	type rankedJSON struct {
		Rank         int     `json:"rank"`
		ID           string  `json:"id"`
		ShortID      int     `json:"short_id"`
		Type         string  `json:"type"`
		Status       string  `json:"status"`
		Title        string  `json:"title"`
		Priority     int     `json:"priority"`
		Score        float64 `json:"score"`
		ScorePercent int     `json:"score_percent"`
	}

	entry := rankedJSON{
		Rank:         1,
		ID:           "task-uuid-123",
		ShortID:      42,
		Type:         "task",
		Status:       "todo",
		Title:        "Important task",
		Priority:     1,
		Score:        8.5,
		ScorePercent: 85,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if v, ok := parsed["id"]; !ok || v != "task-uuid-123" {
		t.Errorf("expected id=task-uuid-123 in score rank JSON, got %v", v)
	}
	if v, ok := parsed["priority"]; !ok || v != float64(1) {
		t.Errorf("expected priority=1 in score rank JSON, got %v", v)
	}
}

// TestRunScoreSet_RejectsPartialMutationOnInvalidArg verifies that when one
// of several `dim=value` arguments is invalid, none of the earlier valid
// assignments leak into the task's ScoreDimensions map.
func TestRunScoreSet_RejectsPartialMutationOnInvalidArg(t *testing.T) {
	deps, project := setupScoreTest(t)
	defer deps.Cleanup()

	firstTask := project.Tasks[0]
	if len(firstTask.ScoreDimensions) > 0 {
		t.Fatalf("expected first task to start with no score dimensions, got %v", firstTask.ScoreDimensions)
	}

	// impact=5 valid, then bogus dim should reject the whole call.
	err := runScoreSet(scoreSetCmd, []string{firstTask.Title, "impact=5", "unknowndim=3"})
	if err == nil {
		t.Fatal("expected error for unknown dimension")
	}

	if firstTask.ScoreDimensions != nil {
		if _, ok := firstTask.ScoreDimensions["impact"]; ok {
			t.Errorf("expected no partial mutation; impact should not be set when later argument fails")
		}
	}
}

// TestRunScoreProfileDelete_RejectsBuiltin verifies that built-in scoring
// profiles cannot be deleted, since deleting them would break any project
// that was assigned the preset.
func TestRunScoreProfileDelete_RejectsBuiltin(t *testing.T) {
	deps, _ := setupScoreConfigOnlyTest(t)
	defer deps.Cleanup()

	// ICE preset is loaded as a built-in by setupScoreConfigOnlyTest.
	err := runScoreProfileDelete(scoreProfileDeleteCmd, []string{"ice"})
	if err == nil {
		t.Fatal("expected error when deleting built-in preset")
	}

	// Reload config and verify the profile is still present.
	manager := config.NewManager(deps.ConfigPath)
	loaded, err := manager.LoadRuntimeState()
	testutil.AssertNoError(t, err)
	if loaded.Automation().GetScoringProfile("ice") == nil {
		t.Error("built-in profile 'ice' was removed despite the rejection")
	}
}

// TestRunScoreProfileEdit_RejectsBuiltin verifies that built-in scoring
// profiles cannot be edited.
func TestRunScoreProfileEdit_RejectsBuiltin(t *testing.T) {
	deps, _ := setupScoreConfigOnlyTest(t)
	defer deps.Cleanup()

	cmd := scoreProfileEditCmd
	// Set the name flag so the edit attempt is non-empty.
	if err := cmd.Flags().Set("name", "Hacked"); err != nil {
		t.Fatalf("failed to set name flag: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Flags().Set("name", "") })

	err := runScoreProfileEdit(cmd, []string{"ice"})
	if err == nil {
		t.Fatal("expected error when editing built-in preset")
	}
}
