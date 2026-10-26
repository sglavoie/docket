package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// cacheTestProject saves a two-task project and returns a constructor for an
// unloaded copy of it, the shape every reader starts from.
func cacheTestProject(t *testing.T) func() *models.Project {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects", "cache")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	var tasks []*models.Task
	for i, title := range []string{"Alpha task", "Bravo task"} {
		task, err := models.NewGenericTask("cache-"+title, i+1, title)
		if err != nil {
			t.Fatal(err)
		}
		tasks = append(tasks, task)
	}
	if err := SaveProject(&models.Project{ID: "cache", Slug: "cache", Name: "cache", Path: path, Tasks: tasks}); err != nil {
		t.Fatal(err)
	}
	return func() *models.Project { return &models.Project{ID: "cache", Slug: "cache", Name: "cache", Path: path} }
}

func loadCached(t *testing.T, cache *ProjectDocumentCache, project *models.Project) *ProjectDocument {
	t.Helper()
	document, err := LoadProjectWithCachedDocument(project, cache)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

// TestProjectDocumentCacheServesUnchangedBytesWithoutSharing proves a second
// load of unchanged bytes is answered from the retained parse, and that neither
// the document nor the project it hands out can write through to that parse.
func TestProjectDocumentCacheServesUnchangedBytesWithoutSharing(t *testing.T) {
	newProject := cacheTestProject(t)
	cache := NewProjectDocumentCache()
	first := loadCached(t, cache, newProject())

	// Marking the retained parse is how the next load can prove it was served
	// from the cache rather than parsed again.
	path := tasksFilePath(newProject().Path)
	cache.entries[path].document.Tasks[0].Title = "Served from cache"

	project := newProject()
	second := loadCached(t, cache, project)
	if got := second.Tasks[0].Title; got != "Served from cache" {
		t.Fatalf("second load title = %q, want the retained parse", got)
	}
	if second.Revision != first.Revision {
		t.Fatalf("revision moved across a cache hit: %s != %s", second.Revision, first.Revision)
	}
	if second.Tasks[0] == cache.entries[path].document.Tasks[0] || project.Tasks[0] == second.Tasks[0] {
		t.Fatal("a cache hit handed out a task the cache or the document still holds")
	}

	second.Tasks[0].Title = "Edited document"
	project.Tasks[0].Title = "Edited project"
	project.BuildTaskTree()
	third := loadCached(t, cache, newProject())
	if got := third.Tasks[0].Title; got != "Served from cache" {
		t.Fatalf("a caller's edit reached the cache: third load title = %q", got)
	}
}

// TestProjectDocumentCacheRereadsChangedBytesOfEqualSizeAndTime is the case a
// modification-time key misses: an edit that keeps the file's size and its
// timestamp. The bytes are the key, so the edit is seen.
func TestProjectDocumentCacheRereadsChangedBytesOfEqualSizeAndTime(t *testing.T) {
	newProject := cacheTestProject(t)
	cache := NewProjectDocumentCache()
	before := loadCached(t, cache, newProject())

	path := tasksFilePath(newProject().Path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(content), "Alpha task", "Delta task", 1)
	if len(edited) != len(content) || edited == string(content) {
		t.Fatal("the edit must change the bytes and keep their length")
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	after := loadCached(t, cache, newProject())
	if got := after.Tasks[0].Title; got != "Delta task" {
		t.Fatalf("title after an equal-size, equal-time edit = %q, want the edit", got)
	}
	if after.Revision == before.Revision {
		t.Fatal("the revision did not move with the edit")
	}
}

// TestNilProjectDocumentCacheParsesEveryLoad keeps LoadProjectWithDocument's
// uncached behaviour: a nil cache is valid and retains nothing.
func TestNilProjectDocumentCacheParsesEveryLoad(t *testing.T) {
	newProject := cacheTestProject(t)
	first := loadCached(t, nil, newProject())
	second := loadCached(t, nil, newProject())
	if first.Revision != second.Revision || first.Tasks[0] == second.Tasks[0] {
		t.Fatal("a nil cache must parse each load afresh to the same revision")
	}
}
