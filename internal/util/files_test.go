package util

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithFileLock_ExclusiveSerializesConcurrentAccess(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithFileLock(lockPath, true, func() error {
				n := concurrent.Add(1)
				// Track max concurrency observed
				for {
					old := maxConcurrent.Load()
					if n <= old || maxConcurrent.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				concurrent.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if maxConcurrent.Load() != 1 {
		t.Errorf("expected max concurrency of 1, got %d", maxConcurrent.Load())
	}
}

func TestWithFileLock_SharedAllowsConcurrentReads(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithFileLock(lockPath, false, func() error {
				n := concurrent.Add(1)
				for {
					old := maxConcurrent.Load()
					if n <= old || maxConcurrent.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				concurrent.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected shared locks to allow concurrency > 1, got %d", maxConcurrent.Load())
	}
}

func TestWithFileLock_CreatesLockFile(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	err := WithFileLock(lockPath, true, func() error {
		_, statErr := os.Stat(lockPath)
		if statErr != nil {
			t.Errorf("lock file should exist during fn: %v", statErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithFileLock_PropagatesFnError(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	err := WithFileLock(lockPath, true, func() error {
		return os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected ErrPermission, got %v", err)
	}
}
