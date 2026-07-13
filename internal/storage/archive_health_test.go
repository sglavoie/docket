package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestRebuildArchiveIndexPersistsUnhealthySource verifies that a canonical parse
// failure is recorded as an unhealthy source (with a ParseError detail) in the
// persisted index rather than silently skipped, while the good sources stay
// indexed.
func TestRebuildArchiveIndexPersistsUnhealthySource(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
		archiveIndexTestTask("arch-good", 1, models.TaskTypeTask, "Good archive", completedAt),
	})
	badContent := "### Task: Bad archive\n" + strings.Repeat("x", scannerMaxBufSize+1)
	if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(badContent), FilePermissions); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	if err := RebuildArchiveIndex(project); err != nil {
		t.Fatalf("RebuildArchiveIndex returned error: %v", err)
	}

	index := readArchiveIndexTestFile(t, project)
	if len(index.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(index.Sources))
	}
	good := index.Sources[0]
	if good.FileName != archiveFileName || good.Unhealthy() || good.SHA256 == "" {
		t.Fatalf("good source should be healthy with a hash: %+v", good)
	}
	bad := index.Sources[1]
	if bad.FileName != "completed.1.md" || !bad.Unhealthy() || bad.ParseError == "" {
		t.Fatalf("corrupt source should be unhealthy with a parse error: %+v", bad)
	}
	if bad.SHA256 == "" {
		t.Fatalf("even an unhealthy source records its content hash: %+v", bad)
	}
}

// TestInspectArchiveIndexReportsUnhealthySource verifies doctor's inspection
// distinguishes corrupt canonical source markdown from a merely stale index.
func TestInspectArchiveIndexReportsUnhealthySource(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
		archiveIndexTestTask("arch-good", 1, models.TaskTypeTask, "Good archive", completedAt),
	})
	badContent := "### Task: Bad archive\n" + strings.Repeat("x", scannerMaxBufSize+1)
	if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(badContent), FilePermissions); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}
	if err := RebuildArchiveIndex(project); err != nil {
		t.Fatalf("RebuildArchiveIndex returned error: %v", err)
	}

	status, err := InspectArchiveIndex(project)
	if err != nil {
		t.Fatalf("InspectArchiveIndex returned error: %v", err)
	}
	if status.Issue != ArchiveIndexIssueUnhealthySource || !status.NeedsRebuild {
		t.Fatalf("status = %+v, want unhealthy-source rebuild issue", status)
	}
	if status.UnhealthySources != 1 || !strings.Contains(status.Detail, "completed.1.md") {
		t.Fatalf("status detail should name the corrupt source: %+v", status)
	}
}

// TestInspectArchiveIndexDetectsHashMismatch verifies the deep integrity check
// catches an archive edit that preserved size and mtime but changed content,
// which the cheap size+mtime freshness check alone would miss.
func TestInspectArchiveIndexDetectsHashMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}
	completedAt := time.Date(2026, 6, 8, 10, 30, 0, 0, time.Local)

	writeArchiveIndexTestArchive(t, tmpDir, archiveFileName, []*models.Task{
		archiveIndexTestTask("arch-hash", 1, models.TaskTypeTask, "Hash source", completedAt),
	})
	if err := RebuildArchiveIndex(project); err != nil {
		t.Fatalf("RebuildArchiveIndex returned error: %v", err)
	}

	archivePath := filepath.Join(tmpDir, archiveFileName)
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	original, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	// Rewrite the file with the same byte length but different content, then reset
	// its mtime so only the content hash reveals the change.
	tampered := []byte(strings.Repeat("Z", len(original)))
	if err = os.WriteFile(archivePath, tampered, FilePermissions); err != nil {
		t.Fatalf("tamper archive: %v", err)
	}
	if err = os.Chtimes(archivePath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("reset mtime: %v", err)
	}

	status, err := InspectArchiveIndex(project)
	if err != nil {
		t.Fatalf("InspectArchiveIndex returned error: %v", err)
	}
	if !status.HashMismatch || !status.NeedsRebuild || status.Issue != ArchiveIndexIssueStale {
		t.Fatalf("status = %+v, want hash-mismatch stale rebuild", status)
	}
}

// TestBuildArchiveIndexFromContentsRefusesUnparseable verifies the transactional
// index builder fails closed on unparseable archive content.
func TestBuildArchiveIndexFromContentsRefusesUnparseable(t *testing.T) {
	badContent := "### Task: Bad\n" + strings.Repeat("x", scannerMaxBufSize+1)
	_, _, err := buildArchiveIndexFromContents([]inMemoryArchiveFile{
		{fileName: archiveFileName, content: []byte(badContent)},
	})
	if err == nil {
		t.Fatal("expected buildArchiveIndexFromContents to fail on unparseable content")
	}
}
