package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
)

func TestPinnedOrderUsesProjectQualifiedIdentities(t *testing.T) {
	at := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	a := pinnedTask("shared", 1, "Alpha", at)
	b := pinnedTask("shared", 1, "Beta", at.Add(-time.Hour))
	repo := newFakeRepo(newProject("alpha", "Alpha", a), newProject("beta", "Beta", b))
	locA := app.TaskLocator{ProjectID: "alpha", TaskID: "shared"}
	locB := app.TaskLocator{ProjectID: "beta", TaskID: "shared"}
	result, err := newTestService(repo).ReorderPinnedTasksOrder(locA, []app.TaskLocator{locB, locA})
	if err != nil {
		t.Fatalf("distinct qualified pins were refused: %v", err)
	}
	if !b.PinnedAt.Equal(at) || !a.PinnedAt.Equal(at.Add(-time.Hour)) || len(result.ProjectIDs) != 2 || len(repo.commits) != 1 {
		t.Fatal("qualified pins did not reorder in one transaction over both projects")
	}
}

func TestPinnedOrderRefusesDifferentOwnerInPlaceOfSubject(t *testing.T) {
	at := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	subject := pinnedTask("shared", 1, "Subject", at)
	other := pinnedTask("shared", 1, "Other owner", at.Add(-time.Hour))
	neighbor := pinnedTask("neighbor", 2, "Neighbor", at.Add(-2*time.Hour))
	repo := newFakeRepo(newProject("alpha", "Alpha", subject), newProject("beta", "Beta", other, neighbor))
	_, err := newTestService(repo).ReorderPinnedTasksOrder(
		app.TaskLocator{ProjectID: "alpha", TaskID: subject.ID},
		locatorsFor("beta", neighbor.ID, other.ID),
	)
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("order omitting the qualified subject error = %v, want ErrInvalidInput", err)
	}
	if !subject.PinnedAt.Equal(at) || !other.PinnedAt.Equal(at.Add(-time.Hour)) ||
		!neighbor.PinnedAt.Equal(at.Add(-2*time.Hour)) || len(repo.commits) != 0 {
		t.Fatal("refused reorder changed timestamps or committed a transaction")
	}
}
