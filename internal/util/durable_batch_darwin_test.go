package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchFullFlushFollowsAllDirectorySyncs(t *testing.T) {
	root := t.TempDir()
	dirs := []string{root, filepath.Join(root, "child")}
	if err := os.Mkdir(dirs[1], 0700); err != nil {
		t.Fatal(err)
	}
	synced, flushed := 0, 0
	err := syncBatchDirectoriesWith(dirs, func(*os.File) error {
		synced++
		return nil
	}, func(*os.File) error {
		if synced != len(dirs) {
			t.Fatal("full flush ran before all directory fsyncs")
		}
		flushed++
		return nil
	})
	if err != nil || flushed != 1 {
		t.Fatalf("same filesystem should need one full flush: %d, %v", flushed, err)
	}
}

func TestBatchSyncAndFullFlushFailuresPropagate(t *testing.T) {
	for _, failure := range []string{"directory", "device"} {
		t.Run(failure, func(t *testing.T) {
			injected := errors.New("injected durability failure")
			err := syncBatchDirectoriesWith([]string{t.TempDir()}, func(*os.File) error {
				if failure == "directory" {
					return injected
				}
				return nil
			}, func(*os.File) error {
				if failure == "directory" {
					t.Fatal("full flush followed a failed directory sync")
				}
				return injected
			})
			if !errors.Is(err, injected) {
				t.Fatalf("durability failure hidden: %v", err)
			}
		})
	}
}
