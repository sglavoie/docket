package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Build a historical schema-19 workspace through frozen codecs, then cross the
// real activation boundary before the UI launches. No live workspace is downgraded.
func buildCheckInFixture(w *uitestWorkspace) error {
	_, err := w.addProject("Check-in Acceptance", func(p *models.Project) {
		waiting := newFixtureTask(p, "Migrated vendor wait")
		waiting.ApplyWorkflowState(models.StateWaiting)
		due := time.Date(2020, 4, 1, 15, 30, 0, 0, time.Local)
		waiting.DueDate = &due
		waiting.Description = "Keep the original timed deadline."
		active := newFixtureTask(p, "Setup active work")
		active.ApplyWorkflowState(models.StateDoing)
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		active.DueDate = &today
		newFixtureTask(p, "Setup waiting work").ApplyWorkflowState(models.StateWaiting)
		stale := newFixtureTask(p, "Dormant cleanup")
		stale.ApplyWorkflowState(models.StateBacklog)
		stale.CreatedAt, stale.UpdatedAt, stale.StateChangedAt = due, due, due
	})
	if err != nil {
		return err
	}
	if err = w.persistSyncedDomains(); err != nil {
		return err
	}
	for _, project := range w.Runtime.Catalog().Projects {
		path := storage.TasksFilePath(project.CatalogPath(w.DataDir))
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		current, parseErr := storage.ParseV19ProjectDocument(string(raw))
		if parseErr != nil {
			return parseErr
		}
		legacy := storage.V17ProjectDocument{ProjectName: current.ProjectName, Comments: current.Comments}
		for _, task := range current.Tasks {
			legacy.Tasks = append(legacy.Tasks, task.V16TaskRecord)
		}
		body, writeErr := storage.WriteV17ProjectDocument(legacy)
		if writeErr != nil {
			return writeErr
		}
		if err = os.WriteFile(path, []byte(body), storage.FilePermissions); err != nil {
			return err
		}
	}
	path := config.WorkspaceCatalogPath(w.DataDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var catalog map[string]any
	if err = json.Unmarshal(raw, &catalog); err != nil {
		return err
	}
	data, ok := catalog["data"].(map[string]any)
	if !ok {
		return fmt.Errorf("legacy fixture catalog has no data envelope")
	}
	data["schema_version"] = 19
	raw, err = json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(path, raw, storage.FilePermissions); err != nil {
		return err
	}
	if _, err = w.Manager.LoadDomains(); err != nil {
		return fmt.Errorf("activate legacy check-in fixture: %w", err)
	}
	w.Runtime, err = w.Manager.LoadRuntimeState()
	return err
}
