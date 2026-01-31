package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestLoadBlockersEmptyFile(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{Path: dir}

	blockers, err := LoadBlockers(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blockers) != 0 {
		t.Errorf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestSaveAndLoadBlockers(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{Path: dir}

	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	blocker := &models.Blocker{
		ID:          "test-uuid-123",
		Title:       "Waiting for API access",
		Description: "Need credentials from vendor",
		CreatedAt:   created,
	}

	err := SaveBlockers(project, []*models.Blocker{blocker})
	if err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadBlockers(project)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(loaded))
	}

	b := loaded[0]
	if b.ID != "test-uuid-123" {
		t.Errorf("expected ID 'test-uuid-123', got %q", b.ID)
	}
	if b.Title != "Waiting for API access" {
		t.Errorf("expected title 'Waiting for API access', got %q", b.Title)
	}
	if b.Description != "Need credentials from vendor" {
		t.Errorf("expected description 'Need credentials from vendor', got %q", b.Description)
	}
	if !b.CreatedAt.Equal(created) {
		t.Errorf("expected created at %v, got %v", created, b.CreatedAt)
	}
	if b.IsResolved() {
		t.Error("expected blocker to not be resolved")
	}
}

func TestSaveAndLoadResolvedBlocker(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{Path: dir}

	created := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 1, 16, 14, 30, 0, 0, time.UTC)
	blocker := &models.Blocker{
		ID:         "resolved-uuid",
		Title:      "Blocked by deployment",
		CreatedAt:  created,
		ResolvedAt: &resolved,
	}

	err := SaveBlockers(project, []*models.Blocker{blocker})
	if err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadBlockers(project)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(loaded))
	}

	b := loaded[0]
	if !b.IsResolved() {
		t.Error("expected blocker to be resolved")
	}
	if !b.ResolvedAt.Equal(resolved) {
		t.Errorf("expected resolved at %v, got %v", resolved, *b.ResolvedAt)
	}
}

func TestSaveMultipleBlockers(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{Path: dir}

	blockers := []*models.Blocker{
		{ID: "b1", Title: "First", CreatedAt: time.Now()},
		{ID: "b2", Title: "Second", Description: "With description", CreatedAt: time.Now()},
		{ID: "b3", Title: "Third", CreatedAt: time.Now()},
	}

	err := SaveBlockers(project, blockers)
	if err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadBlockers(project)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 blockers, got %d", len(loaded))
	}

	if loaded[0].Title != "First" || loaded[1].Title != "Second" || loaded[2].Title != "Third" {
		t.Error("blocker order not preserved")
	}
	if loaded[1].Description != "With description" {
		t.Errorf("expected description 'With description', got %q", loaded[1].Description)
	}
}

func TestParseBlockersMarkdown(t *testing.T) {
	content := `# Blockers

## API Access
<!-- id: uuid-1, created: 2026-01-15T10:30:00Z -->

Need vendor credentials.

## Deploy Pipeline
<!-- id: uuid-2, created: 2026-01-16T08:00:00Z, resolved: 2026-01-17T12:00:00Z -->

Was blocked by CI.
`
	blockers, err := parseBlockersMarkdown(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}

	if blockers[0].ID != "uuid-1" {
		t.Errorf("first blocker ID: expected 'uuid-1', got %q", blockers[0].ID)
	}
	if blockers[0].Title != "API Access" {
		t.Errorf("first blocker title: expected 'API Access', got %q", blockers[0].Title)
	}
	if blockers[0].Description != "Need vendor credentials." {
		t.Errorf("first blocker description: expected 'Need vendor credentials.', got %q", blockers[0].Description)
	}
	if blockers[0].IsResolved() {
		t.Error("expected first blocker to not be resolved")
	}

	if blockers[1].ID != "uuid-2" {
		t.Errorf("second blocker ID: expected 'uuid-2', got %q", blockers[1].ID)
	}
	if !blockers[1].IsResolved() {
		t.Error("expected second blocker to be resolved")
	}
}

func TestBlockersPathLocation(t *testing.T) {
	project := &models.Project{Path: "/tmp/test-project"}
	path := BlockersPath(project)
	expected := filepath.Join("/tmp/test-project", "blockers.md")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestGetActiveBlockerCount(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{Path: dir}

	resolved := time.Now()
	blockers := []*models.Blocker{
		{ID: "b1", Title: "Active 1", CreatedAt: time.Now()},
		{ID: "b2", Title: "Resolved", CreatedAt: time.Now(), ResolvedAt: &resolved},
		{ID: "b3", Title: "Active 2", CreatedAt: time.Now()},
	}
	if err := SaveBlockers(project, blockers); err != nil {
		t.Fatalf("save error: %v", err)
	}

	count := GetActiveBlockerCount(project)
	if count != 2 {
		t.Errorf("expected 2 active blockers, got %d", count)
	}
}

func TestGetActiveBlockerCountNoFile(t *testing.T) {
	dir := t.TempDir()
	project := &models.Project{Path: dir}

	count := GetActiveBlockerCount(project)
	if count != 0 {
		t.Errorf("expected 0 active blockers, got %d", count)
	}
}

func TestFindBlockerByPrefix(t *testing.T) {
	blockers := []*models.Blocker{
		{ID: "abc-123-def"},
		{ID: "xyz-456-ghi"},
	}

	found := FindBlockerByPrefix(blockers, "abc")
	if found == nil || found.ID != "abc-123-def" {
		t.Error("expected to find blocker with prefix 'abc'")
	}

	found = FindBlockerByPrefix(blockers, "xyz")
	if found == nil || found.ID != "xyz-456-ghi" {
		t.Error("expected to find blocker with prefix 'xyz'")
	}

	found = FindBlockerByPrefix(blockers, "nope")
	if found != nil {
		t.Error("expected nil for non-matching prefix")
	}
}

func TestSaveBlockersCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "nested", "path")
	project := &models.Project{Path: nestedPath}

	blocker := &models.Blocker{ID: "b1", Title: "Test", CreatedAt: time.Now()}
	err := SaveBlockers(project, []*models.Blocker{blocker})
	if err != nil {
		t.Fatalf("save error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(nestedPath, "blockers.md")); os.IsNotExist(err) {
		t.Error("expected blockers.md to be created")
	}
}
