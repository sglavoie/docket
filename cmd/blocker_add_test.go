package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBlockerLifecycle(t *testing.T) {
	t.Run("new blocker has correct initial state", func(t *testing.T) {
		blocker := models.NewBlocker("Waiting for API key")

		if blocker.ID == "" {
			t.Error("expected blocker to have a non-empty ID")
		}
		if blocker.Title != "Waiting for API key" {
			t.Errorf("expected title 'Waiting for API key', got %q", blocker.Title)
		}
		if blocker.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
		if blocker.IsResolved() {
			t.Error("expected new blocker to not be resolved")
		}
		if blocker.ResolvedAt != nil {
			t.Error("expected ResolvedAt to be nil for new blocker")
		}
	})

	t.Run("resolving blocker sets resolved state", func(t *testing.T) {
		blocker := models.NewBlocker("Blocked by deployment")

		blocker.Resolve()

		if !blocker.IsResolved() {
			t.Error("expected blocker to be resolved after Resolve()")
		}
		if blocker.ResolvedAt == nil {
			t.Error("expected ResolvedAt to be set after Resolve()")
		}
	})

	t.Run("unresolving blocker clears resolved state", func(t *testing.T) {
		blocker := models.NewBlocker("Blocked by review")
		blocker.Resolve()

		blocker.Unresolve()

		if blocker.IsResolved() {
			t.Error("expected blocker to not be resolved after Unresolve()")
		}
		if blocker.ResolvedAt != nil {
			t.Error("expected ResolvedAt to be nil after Unresolve()")
		}
	})
}

func TestBlockerResolveIdempotent(t *testing.T) {
	t.Run("resolving an already resolved blocker does not panic", func(t *testing.T) {
		blocker := models.NewBlocker("Already resolved")
		blocker.Resolve()
		firstResolvedAt := blocker.ResolvedAt

		// Resolve again - should not panic
		blocker.Resolve()

		if !blocker.IsResolved() {
			t.Error("expected blocker to still be resolved")
		}
		// ResolvedAt may be updated to a new time, but should still be set
		if blocker.ResolvedAt == nil {
			t.Error("expected ResolvedAt to still be set")
		}
		_ = firstResolvedAt // used to verify no panic on second resolve
	})
}

func TestBlockerDescription(t *testing.T) {
	t.Run("description can be set and retrieved", func(t *testing.T) {
		blocker := models.NewBlocker("Need credentials")

		if blocker.Description != "" {
			t.Errorf("expected empty description for new blocker, got %q", blocker.Description)
		}

		blocker.Description = "Waiting for the ops team to provision service account credentials"

		if blocker.Description != "Waiting for the ops team to provision service account credentials" {
			t.Errorf("expected description to be set, got %q", blocker.Description)
		}
	})
}
