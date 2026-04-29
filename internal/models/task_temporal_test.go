package models_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

// parseRoundTrip writes then re-parses metadata for a task of the given type,
// returning the repopulated task.
func parseRoundTrip(t *testing.T, original *models.Task) *models.Task {
	t.Helper()
	meta := format.WriteMetadata(original)
	// Extract inner content from <!-- ... -->
	inner := ""
	if m := format.MetadataRegex().FindStringSubmatch(meta); len(m) == 2 {
		inner = m[1]
	}
	result := &models.Task{
		ID:   original.ID,
		Type: original.Type,
	}
	warnings := format.ParseMetadata(result, inner)
	for _, w := range warnings {
		t.Logf("parse warning: %s", w.Message)
	}
	return result
}

func TestTaskTemporal_StartDate_RoundTrip(t *testing.T) {
	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:        "t1",
		Type:      models.TaskTypeTask,
		Status:    models.StatusTodo,
		Priority:  3,
		CreatedAt: time.Now(),
		StartDate: &start,
	}

	result := parseRoundTrip(t, task)

	if result.StartDate == nil {
		t.Fatal("StartDate not parsed")
	}
	if !result.StartDate.Equal(start) {
		t.Errorf("StartDate: got %v, want %v", result.StartDate, start)
	}
}

func TestTaskTemporal_EstimatedDays_RoundTrip(t *testing.T) {
	task := &models.Task{
		ID:            "t2",
		Type:          models.TaskTypeTask,
		Status:        models.StatusTodo,
		Priority:      3,
		CreatedAt:     time.Now(),
		EstimatedDays: 2.5,
	}

	result := parseRoundTrip(t, task)

	if result.EstimatedDays != 2.5 {
		t.Errorf("EstimatedDays: got %v, want 2.5", result.EstimatedDays)
	}
}

func TestTaskTemporal_StartAndEstimated_RoundTrip(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:            "t3",
		Type:          models.TaskTypeTask,
		Status:        models.StatusTodo,
		Priority:      3,
		CreatedAt:     time.Now(),
		StartDate:     &start,
		EstimatedDays: 5.0,
	}

	result := parseRoundTrip(t, task)

	if result.StartDate == nil || !result.StartDate.Equal(start) {
		t.Errorf("StartDate: got %v, want %v", result.StartDate, start)
	}
	if result.EstimatedDays != 5.0 {
		t.Errorf("EstimatedDays: got %v, want 5.0", result.EstimatedDays)
	}
}

func TestTaskTemporal_StartDate_Unset_NoWrite(t *testing.T) {
	task := &models.Task{
		ID:        "t4",
		Type:      models.TaskTypeTask,
		Status:    models.StatusTodo,
		Priority:  3,
		CreatedAt: time.Now(),
	}

	result := parseRoundTrip(t, task)

	if result.StartDate != nil {
		t.Errorf("StartDate should be nil when unset, got %v", result.StartDate)
	}
	if result.EstimatedDays != 0 {
		t.Errorf("EstimatedDays should be 0 when unset, got %v", result.EstimatedDays)
	}
}

func TestTaskTemporal_EpicScopeBaseline_RoundTrip(t *testing.T) {
	baselined := time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local)
	task := &models.Task{
		ID:        "e1",
		Type:      models.TaskTypeEpic,
		Status:    models.StatusTodo,
		Priority:  3,
		CreatedAt: time.Now(),
		EpicAttrs: &models.EpicAttrs{
			InitialChildCount: 5,
			ScopeBaselinedAt:  &baselined,
		},
	}

	result := parseRoundTrip(t, task)

	if result.EpicAttrs == nil {
		t.Fatal("EpicAttrs is nil after round-trip")
	}
	if result.EpicAttrs.InitialChildCount != 5 {
		t.Errorf("InitialChildCount: got %d, want 5", result.EpicAttrs.InitialChildCount)
	}
	if result.EpicAttrs.ScopeBaselinedAt == nil {
		t.Fatal("ScopeBaselinedAt is nil after round-trip")
	}
	if !result.EpicAttrs.ScopeBaselinedAt.Equal(baselined) {
		t.Errorf("ScopeBaselinedAt: got %v, want %v", result.EpicAttrs.ScopeBaselinedAt, baselined)
	}
}

func TestTaskTemporal_EpicScopeBaseline_Zero_NoWrite(t *testing.T) {
	task := &models.Task{
		ID:        "e2",
		Type:      models.TaskTypeEpic,
		Status:    models.StatusTodo,
		Priority:  3,
		CreatedAt: time.Now(),
		EpicAttrs: &models.EpicAttrs{
			Scope: models.EpicScopeDefined,
		},
	}

	result := parseRoundTrip(t, task)

	if result.EpicAttrs == nil {
		t.Fatal("EpicAttrs is nil after round-trip")
	}
	if result.EpicAttrs.InitialChildCount != 0 {
		t.Errorf("InitialChildCount should be 0 when unset, got %d", result.EpicAttrs.InitialChildCount)
	}
	if result.EpicAttrs.ScopeBaselinedAt != nil {
		t.Errorf("ScopeBaselinedAt should be nil when unset, got %v", result.EpicAttrs.ScopeBaselinedAt)
	}
}

func TestTaskTemporal_ScopeDelta(t *testing.T) {
	baselined := time.Now()
	task := &models.Task{
		Type:      models.TaskTypeEpic,
		EpicAttrs: &models.EpicAttrs{InitialChildCount: 3, ScopeBaselinedAt: &baselined},
		Children:  []*models.Task{{}, {}, {}, {}}, // 4 children now
	}

	delta, ok := task.ScopeDelta()
	if !ok {
		t.Fatal("ScopeDelta should return true when baselined")
	}
	if delta != 1 {
		t.Errorf("ScopeDelta: got %d, want 1", delta)
	}
}

func TestTaskTemporal_ScopeDelta_NotBaselined(t *testing.T) {
	task := &models.Task{
		Type:      models.TaskTypeEpic,
		EpicAttrs: &models.EpicAttrs{},
		Children:  []*models.Task{{}, {}},
	}

	delta, ok := task.ScopeDelta()
	if ok {
		t.Error("ScopeDelta should return false when not baselined")
	}
	if delta != 0 {
		t.Errorf("ScopeDelta: got %d, want 0", delta)
	}
}
