package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateArchiveIfNeeded(t *testing.T) {
	t.Run("does nothing when no archive exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := rotateArchiveIfNeeded(tmpDir, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		archivePath := filepath.Join(tmpDir, "completed.md")
		if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
			t.Error("should not create archive file when none exists")
		}
	})

	t.Run("does nothing when archive is under threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "completed.md")

		content := "# Small archive content"
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		err := rotateArchiveIfNeeded(tmpDir, 1024)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			t.Error("archive file should still exist")
		}

		rotatedPath := filepath.Join(tmpDir, "completed.1.md")
		if _, err := os.Stat(rotatedPath); !os.IsNotExist(err) {
			t.Error("should not rotate when under threshold")
		}
	})

	t.Run("rotates archive when over threshold", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "completed.md")

		content := strings.Repeat("x", 200)
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		err := rotateArchiveIfNeeded(tmpDir, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
			t.Error("original archive should be removed after rotation")
		}

		rotatedPath := filepath.Join(tmpDir, "completed.1.md")
		rotatedContent, err := os.ReadFile(rotatedPath)
		if err != nil {
			t.Fatalf("failed to read rotated archive: %v", err)
		}
		if string(rotatedContent) != content {
			t.Error("rotated archive content should match original")
		}
	})

	t.Run("shifts existing rotated files when rotating", func(t *testing.T) {
		tmpDir := t.TempDir()
		archivePath := filepath.Join(tmpDir, "completed.md")
		rotated1 := filepath.Join(tmpDir, "completed.1.md")
		rotated2 := filepath.Join(tmpDir, "completed.2.md")

		if err := os.WriteFile(archivePath, []byte(strings.Repeat("a", 200)), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}
		if err := os.WriteFile(rotated1, []byte("rotated1-content"), 0644); err != nil {
			t.Fatalf("failed to write rotated.1: %v", err)
		}
		if err := os.WriteFile(rotated2, []byte("rotated2-content"), 0644); err != nil {
			t.Fatalf("failed to write rotated.2: %v", err)
		}

		err := rotateArchiveIfNeeded(tmpDir, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content1, _ := os.ReadFile(rotated1)
		if !strings.HasPrefix(string(content1), "aaa") {
			t.Error("completed.1.md should contain the original archive content")
		}

		content2, _ := os.ReadFile(rotated2)
		if string(content2) != "rotated1-content" {
			t.Error("completed.2.md should contain previous completed.1.md content")
		}

		rotated3 := filepath.Join(tmpDir, "completed.3.md")
		content3, _ := os.ReadFile(rotated3)
		if string(content3) != "rotated2-content" {
			t.Error("completed.3.md should contain previous completed.2.md content")
		}
	})
}