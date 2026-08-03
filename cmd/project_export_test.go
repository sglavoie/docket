package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

func TestRunProjectExportRefusesMigrationSnapshotDestination(t *testing.T) {
	dataDir := t.TempDir()
	setCommandDataDir(t, dataDir)
	if _, err := LoadCLIWorkspace(); err != nil {
		t.Fatalf("bootstrap workspace: %v", err)
	}

	backupsDir := config.PreSimplificationV5BackupsDir(dataDir)
	if err := os.MkdirAll(backupsDir, 0o700); err != nil {
		t.Fatalf("create backups dir: %v", err)
	}
	archivePath := filepath.Join(backupsDir, "pre-simplification-v5-20260729T010203Z.tar.gz")
	want := []byte("immutable rollback archive")
	if err := os.WriteFile(archivePath, want, 0o400); err != nil {
		t.Fatalf("write protected archive: %v", err)
	}

	previous := projectExportFile
	projectExportFile = archivePath
	t.Cleanup(func() { projectExportFile = previous })

	err := runProjectExport(nil, []string{config.SystemUnfiledSlug})
	if !errors.Is(err, config.ErrImmutablePreSimplificationV5Snapshot) {
		t.Fatalf("project export error = %v, want immutable snapshot error", err)
	}
	got, readErr := os.ReadFile(archivePath)
	if readErr != nil {
		t.Fatalf("read protected archive after export: %v", readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("project export changed the protected rollback archive")
	}
}
