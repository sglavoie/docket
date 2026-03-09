package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetCycleProjects(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "p1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "p2", Name: "Active 2", Archived: false}
	archived := &models.Project{ID: "p3", Name: "Archived", Archived: true}

	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.AddProject(archived)

	cfg.SetCycleProjects([]string{"p1", "p2", "p3"})

	projects := cfg.GetCycleProjects()

	// Should only return 2 (active projects, archived filtered out)
	if len(projects) != 2 {
		t.Errorf("expected 2 cycle projects, got %d", len(projects))
	}

	// Order should be preserved
	if projects[0].ID != "p1" || projects[1].ID != "p2" {
		t.Error("cycle projects order not preserved")
	}
}

func TestGetCycleProjects_FiltersNonExistent(t *testing.T) {
	cfg := NewConfig()

	active := &models.Project{ID: "p1", Name: "Active", Archived: false}
	cfg.AddProject(active)

	cfg.SetCycleProjects([]string{"p1", "nonexistent", "also-missing"})

	projects := cfg.GetCycleProjects()

	if len(projects) != 1 {
		t.Errorf("expected 1 cycle project, got %d", len(projects))
	}
	if projects[0].ID != "p1" {
		t.Error("expected p1 in cycle projects")
	}
}

func TestGetCycleProjects_Empty(t *testing.T) {
	cfg := NewConfig()

	projects := cfg.GetCycleProjects()

	// Nil and empty slice are equivalent in Go for range/len/append
	if len(projects) != 0 {
		t.Errorf("expected 0 cycle projects, got %d", len(projects))
	}
}

func TestSetCycleProjects(t *testing.T) {
	cfg := NewConfig()

	ids := []string{"p1", "p2", "p3"}
	cfg.SetCycleProjects(ids)

	if len(cfg.CycleProjects) != 3 {
		t.Errorf("expected 3 cycle project IDs, got %d", len(cfg.CycleProjects))
	}
}

func TestGetCycleProjectIndex(t *testing.T) {
	cfg := NewConfig()

	active1 := &models.Project{ID: "p1", Name: "Active 1", Archived: false}
	active2 := &models.Project{ID: "p2", Name: "Active 2", Archived: false}
	cfg.AddProject(active1)
	cfg.AddProject(active2)
	cfg.SetCycleProjects([]string{"p1", "p2"})

	if idx := cfg.GetCycleProjectIndex("p1"); idx != 0 {
		t.Errorf("expected index 0 for p1, got %d", idx)
	}
	if idx := cfg.GetCycleProjectIndex("p2"); idx != 1 {
		t.Errorf("expected index 1 for p2, got %d", idx)
	}
}

func TestGetCycleProjectIndex_NotFound(t *testing.T) {
	cfg := NewConfig()

	active := &models.Project{ID: "p1", Name: "Active", Archived: false}
	cfg.AddProject(active)
	cfg.SetCycleProjects([]string{"p1"})

	if idx := cfg.GetCycleProjectIndex("nonexistent"); idx != -1 {
		t.Errorf("expected -1 for non-existent project, got %d", idx)
	}
}

func TestGetCycleProjectIndex_Empty(t *testing.T) {
	cfg := NewConfig()

	if idx := cfg.GetCycleProjectIndex("p1"); idx != -1 {
		t.Errorf("expected -1 for empty cycle list, got %d", idx)
	}
}

func TestGetInfoPanelMode_DefaultInfoCards(t *testing.T) {
	cfg := NewConfig()

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1), got %d", mode)
	}
}

func TestGetInfoPanelMode_LegacyFalse(t *testing.T) {
	cfg := NewConfig()
	f := false
	cfg.ShowInfoPanel = &f

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelOff {
		t.Errorf("expected InfoPanelOff (0), got %d", mode)
	}
}

func TestGetInfoPanelMode_LegacyTrue(t *testing.T) {
	cfg := NewConfig()
	tr := true
	cfg.ShowInfoPanel = &tr

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1), got %d", mode)
	}
}

func TestGetInfoPanelMode_ExplicitMode(t *testing.T) {
	cfg := NewConfig()
	m := int(InfoPanelProjectSummary)
	cfg.InfoPanelMode = &m

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelProjectSummary {
		t.Errorf("expected InfoPanelProjectSummary (2), got %d", mode)
	}
}

func TestGetInfoPanelMode_ClampOutOfRange(t *testing.T) {
	cfg := NewConfig()
	m := 99
	cfg.InfoPanelMode = &m

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1) for out-of-range, got %d", mode)
	}
}

func TestGetInfoPanelMode_ClampNegative(t *testing.T) {
	cfg := NewConfig()
	m := -1
	cfg.InfoPanelMode = &m

	mode := cfg.GetInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1) for negative, got %d", mode)
	}
}

func TestCycleInfoPanelMode(t *testing.T) {
	cfg := NewConfig()

	// Default is InfoPanelInfoCards (1), cycle -> 2
	mode := cfg.CycleInfoPanelMode()
	if mode != InfoPanelProjectSummary {
		t.Errorf("expected InfoPanelProjectSummary (2), got %d", mode)
	}

	// 2 -> 0
	mode = cfg.CycleInfoPanelMode()
	if mode != InfoPanelOff {
		t.Errorf("expected InfoPanelOff (0), got %d", mode)
	}

	// 0 -> 1
	mode = cfg.CycleInfoPanelMode()
	if mode != InfoPanelInfoCards {
		t.Errorf("expected InfoPanelInfoCards (1), got %d", mode)
	}
}

func TestSetInfoPanelMode_ClearsLegacy(t *testing.T) {
	cfg := NewConfig()
	tr := true
	cfg.ShowInfoPanel = &tr

	cfg.SetInfoPanelMode(InfoPanelProjectSummary)

	if cfg.ShowInfoPanel != nil {
		t.Error("SetInfoPanelMode should clear legacy ShowInfoPanel")
	}
	if cfg.InfoPanelMode == nil || *cfg.InfoPanelMode != int(InfoPanelProjectSummary) {
		t.Error("InfoPanelMode should be set to ProjectSummary")
	}
}

func TestGetShowInfoPanel_DelegatesToMode(t *testing.T) {
	cfg := NewConfig()
	cfg.SetInfoPanelMode(InfoPanelOff)
	if cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return false when mode is Off")
	}

	cfg.SetInfoPanelMode(InfoPanelInfoCards)
	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return true when mode is InfoCards")
	}

	cfg.SetInfoPanelMode(InfoPanelProjectSummary)
	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return true when mode is ProjectSummary")
	}
}

func TestGetShowInfoPanel_DefaultTrue(t *testing.T) {
	cfg := NewConfig()

	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should default to true")
	}
}

func TestGetShowInfoPanel_ExplicitFalse(t *testing.T) {
	cfg := NewConfig()
	cfg.SetShowInfoPanel(false)

	if cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return false when set to false")
	}
}

func TestGetShowInfoPanel_ExplicitTrue(t *testing.T) {
	cfg := NewConfig()
	cfg.SetShowInfoPanel(true)

	if !cfg.GetShowInfoPanel() {
		t.Error("GetShowInfoPanel should return true when set to true")
	}
}

func TestSetShowInfoPanel(t *testing.T) {
	cfg := NewConfig()

	cfg.SetShowInfoPanel(false)
	if cfg.ShowInfoPanel == nil {
		t.Error("ShowInfoPanel should not be nil after SetShowInfoPanel")
	}
	if *cfg.ShowInfoPanel {
		t.Error("ShowInfoPanel should be false")
	}

	cfg.SetShowInfoPanel(true)
	if !*cfg.ShowInfoPanel {
		t.Error("ShowInfoPanel should be true")
	}
}

func TestGetRecentPaletteItems_Empty(t *testing.T) {
	cfg := NewConfig()

	items := cfg.GetRecentPaletteItems()

	if items == nil {
		t.Error("GetRecentPaletteItems should return empty slice, not nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestSetRecentPaletteItems(t *testing.T) {
	cfg := NewConfig()

	items := []RecentPaletteItem{
		{Category: "Commands", ID: "task-add"},
		{Category: "Projects", ID: "my-project"},
	}
	cfg.SetRecentPaletteItems(items)

	result := cfg.GetRecentPaletteItems()
	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
	if result[0].Category != "Commands" || result[0].ID != "task-add" {
		t.Error("first item mismatch")
	}
}

func TestGetLastPaletteCategory(t *testing.T) {
	cfg := NewConfig()

	if category := cfg.GetLastPaletteCategory(); category != "" {
		t.Errorf("expected empty string, got '%s'", category)
	}

	cfg.SetLastPaletteCategory(":task ")
	if category := cfg.GetLastPaletteCategory(); category != ":task " {
		t.Errorf("expected ':task ', got '%s'", category)
	}
}

func TestSetLastPaletteCategory(t *testing.T) {
	cfg := NewConfig()

	cfg.SetLastPaletteCategory(":project ")
	if cfg.LastPaletteCategory != ":project " {
		t.Errorf("expected ':project ', got '%s'", cfg.LastPaletteCategory)
	}
}

func TestGetPaletteSearchHistory_Empty(t *testing.T) {
	cfg := NewConfig()

	history := cfg.GetPaletteSearchHistory()

	if history == nil {
		t.Error("GetPaletteSearchHistory should return empty slice, not nil")
	}
	if len(history) != 0 {
		t.Errorf("expected 0 items, got %d", len(history))
	}
}

func TestSetPaletteSearchHistory(t *testing.T) {
	cfg := NewConfig()

	history := []string{"task add", "project list", "help"}
	cfg.SetPaletteSearchHistory(history)

	result := cfg.GetPaletteSearchHistory()
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
	if result[0] != "task add" {
		t.Errorf("expected 'task add', got '%s'", result[0])
	}
}

func TestGetPinnedCommands_Empty(t *testing.T) {
	cfg := NewConfig()

	commands := cfg.GetPinnedCommands()

	if commands == nil {
		t.Error("GetPinnedCommands should return empty slice, not nil")
	}
	if len(commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(commands))
	}
}

func TestSetPinnedCommands(t *testing.T) {
	cfg := NewConfig()

	commands := []string{"task-add", "project-switch"}
	cfg.SetPinnedCommands(commands)

	result := cfg.GetPinnedCommands()
	if len(result) != 2 {
		t.Errorf("expected 2 commands, got %d", len(result))
	}
}

func TestIsCommandPinned(t *testing.T) {
	cfg := NewConfig()
	cfg.PinnedCommands = []string{"task-add", "project-list"}

	if !cfg.IsCommandPinned("task-add") {
		t.Error("task-add should be pinned")
	}
	if !cfg.IsCommandPinned("project-list") {
		t.Error("project-list should be pinned")
	}
	if cfg.IsCommandPinned("nonexistent") {
		t.Error("nonexistent should not be pinned")
	}
}

func TestIsCommandPinned_EmptyList(t *testing.T) {
	cfg := NewConfig()

	if cfg.IsCommandPinned("anything") {
		t.Error("nothing should be pinned on empty list")
	}
}

func TestTogglePinnedCommand_Pin(t *testing.T) {
	cfg := NewConfig()

	pinned := cfg.TogglePinnedCommand("task-add")

	if !pinned {
		t.Error("TogglePinnedCommand should return true when pinning")
	}
	if !cfg.IsCommandPinned("task-add") {
		t.Error("task-add should be pinned after toggle")
	}
}

func TestTogglePinnedCommand_Unpin(t *testing.T) {
	cfg := NewConfig()
	cfg.PinnedCommands = []string{"task-add", "project-list"}

	pinned := cfg.TogglePinnedCommand("task-add")

	if pinned {
		t.Error("TogglePinnedCommand should return false when unpinning")
	}
	if cfg.IsCommandPinned("task-add") {
		t.Error("task-add should not be pinned after toggle")
	}
	if !cfg.IsCommandPinned("project-list") {
		t.Error("project-list should still be pinned")
	}
}

func TestTogglePinnedCommand_MultipleToggles(t *testing.T) {
	cfg := NewConfig()

	// Pin
	if !cfg.TogglePinnedCommand("cmd") {
		t.Error("first toggle should pin")
	}
	// Unpin
	if cfg.TogglePinnedCommand("cmd") {
		t.Error("second toggle should unpin")
	}
	// Pin again
	if !cfg.TogglePinnedCommand("cmd") {
		t.Error("third toggle should pin again")
	}
}

func TestGetSavedSearches_Empty(t *testing.T) {
	cfg := NewConfig()

	searches := cfg.GetSavedSearches()

	if searches == nil {
		t.Error("GetSavedSearches should return empty map, not nil")
	}
	if len(searches) != 0 {
		t.Errorf("expected 0 searches, got %d", len(searches))
	}
}

func TestSaveSearch(t *testing.T) {
	cfg := NewConfig()

	cfg.SaveSearch("urgent", "priority:1 status:in_progress")

	query := cfg.GetSavedSearch("urgent")
	if query != "priority:1 status:in_progress" {
		t.Errorf("expected 'priority:1 status:in_progress', got '%s'", query)
	}
}

func TestSaveSearch_Overwrite(t *testing.T) {
	cfg := NewConfig()

	cfg.SaveSearch("my-search", "old query")
	cfg.SaveSearch("my-search", "new query")

	query := cfg.GetSavedSearch("my-search")
	if query != "new query" {
		t.Errorf("expected 'new query', got '%s'", query)
	}
}

func TestSaveSearch_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SavedSearches = nil

	cfg.SaveSearch("test", "query")

	if cfg.SavedSearches == nil {
		t.Error("SavedSearches should be initialized")
	}
	if cfg.SavedSearches["test"] != "query" {
		t.Error("search should be saved")
	}
}

func TestDeleteSavedSearch(t *testing.T) {
	cfg := NewConfig()
	cfg.SaveSearch("to-delete", "query")
	cfg.SaveSearch("to-keep", "other query")

	deleted := cfg.DeleteSavedSearch("to-delete")

	if !deleted {
		t.Error("DeleteSavedSearch should return true for existing search")
	}
	if cfg.GetSavedSearch("to-delete") != "" {
		t.Error("deleted search should return empty string")
	}
	if cfg.GetSavedSearch("to-keep") != "other query" {
		t.Error("to-keep should still exist")
	}
}

func TestDeleteSavedSearch_NonExistent(t *testing.T) {
	cfg := NewConfig()

	deleted := cfg.DeleteSavedSearch("nonexistent")

	if deleted {
		t.Error("DeleteSavedSearch should return false for non-existent search")
	}
}

func TestDeleteSavedSearch_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SavedSearches = nil

	deleted := cfg.DeleteSavedSearch("test")

	if deleted {
		t.Error("DeleteSavedSearch should return false with nil map")
	}
}

func TestGetSavedSearch_NotFound(t *testing.T) {
	cfg := NewConfig()
	cfg.SaveSearch("exists", "query")

	query := cfg.GetSavedSearch("nonexistent")

	if query != "" {
		t.Errorf("expected empty string for non-existent search, got '%s'", query)
	}
}

func TestGetSavedSearch_NilMap(t *testing.T) {
	cfg := &Config{}
	cfg.SavedSearches = nil

	query := cfg.GetSavedSearch("test")

	if query != "" {
		t.Errorf("expected empty string with nil map, got '%s'", query)
	}
}

func TestGetCommandHistory_Empty(t *testing.T) {
	cfg := NewConfig()

	history := cfg.GetCommandHistory()

	if history == nil {
		t.Error("GetCommandHistory should return empty slice, not nil")
	}
	if len(history) != 0 {
		t.Errorf("expected 0 items, got %d", len(history))
	}
}

func TestAddCommandToHistory(t *testing.T) {
	cfg := NewConfig()

	cfg.AddCommandToHistory("task-add")

	history := cfg.GetCommandHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 item, got %d", len(history))
	}
	if history[0] != "task-add" {
		t.Errorf("expected 'task-add', got '%s'", history[0])
	}
}

func TestAddCommandToHistory_MoveToFront(t *testing.T) {
	cfg := NewConfig()

	cfg.AddCommandToHistory("first")
	cfg.AddCommandToHistory("second")
	cfg.AddCommandToHistory("third")
	cfg.AddCommandToHistory("first") // Add first again

	history := cfg.GetCommandHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 items (no duplicates), got %d", len(history))
	}
	if history[0] != "first" {
		t.Error("'first' should be at the front after re-adding")
	}
	if history[1] != "third" {
		t.Error("'third' should be second")
	}
	if history[2] != "second" {
		t.Error("'second' should be third")
	}
}

func TestAddCommandToHistory_MaxLimit(t *testing.T) {
	cfg := NewConfig()

	// Add 12 commands
	for i := range 12 {
		cfg.AddCommandToHistory("cmd" + string(rune('a'+i)))
	}

	history := cfg.GetCommandHistory()
	if len(history) != 10 {
		t.Errorf("expected max 10 items, got %d", len(history))
	}

	// Most recent should be at front
	if history[0] != "cmdl" {
		t.Errorf("expected 'cmdl' (most recent) at front, got '%s'", history[0])
	}
	// Oldest within limit should be at back
	if history[9] != "cmdc" {
		t.Errorf("expected 'cmdc' at back, got '%s'", history[9])
	}
}

func TestAddCommandToHistory_Deduplication(t *testing.T) {
	cfg := NewConfig()

	cfg.AddCommandToHistory("cmd")
	cfg.AddCommandToHistory("cmd")
	cfg.AddCommandToHistory("cmd")

	history := cfg.GetCommandHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 item (deduplicated), got %d", len(history))
	}
}

func TestAddCommandToHistory_MaxWithDuplicates(t *testing.T) {
	cfg := NewConfig()

	// Add 10 unique commands
	for i := range 10 {
		cfg.AddCommandToHistory("cmd" + string(rune('a'+i)))
	}

	// Add first command again (should move to front, not increase count)
	cfg.AddCommandToHistory("cmda")

	history := cfg.GetCommandHistory()
	if len(history) != 10 {
		t.Errorf("expected 10 items, got %d", len(history))
	}
	if history[0] != "cmda" {
		t.Error("'cmda' should be at front after re-adding")
	}
}
