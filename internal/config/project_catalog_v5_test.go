package config

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func v5CatalogProject(id string, createdAt time.Time) V5ProjectCandidate {
	return V5ProjectCandidate{
		ID: id, Slug: id, Name: id, Context: "personal",
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
}

func TestV5CatalogAutomaticColorSlotsAndLifecyclePreservation(t *testing.T) {
	createdAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	catalog := V5WorkspaceCatalogCandidate{SchemaVersion: V5ConfigSchemaVersion}

	for index := 1; index <= ProjectColorSlotCount+1; index++ {
		project := v5CatalogProject(fmt.Sprintf("project-%02d", index), createdAt.Add(time.Duration(index)*time.Minute))
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("UpsertProject(%d): %v", index, err)
		}
	}
	seen := make(map[int]bool, ProjectColorSlotCount)
	for index, project := range catalog.Projects {
		if !validProjectColorSlot(project.ColorSlot) {
			t.Fatalf("project %q received invalid slot %d", project.ID, project.ColorSlot)
		}
		if index < ProjectColorSlotCount {
			if seen[project.ColorSlot] {
				t.Fatalf("slot %d reused before palette exhaustion", project.ColorSlot)
			}
			seen[project.ColorSlot] = true
		}
	}
	if got := catalog.Projects[ProjectColorSlotCount].ColorSlot; got != 1 {
		t.Fatalf("first balanced reuse slot = %d, want 1", got)
	}

	edited := catalog.Projects[0]
	originalSlot := edited.ColorSlot
	edited.Description = "edited"
	edited.Context = "work"
	edited.Pinned = true
	pinnedAt := createdAt.Add(time.Hour)
	edited.PinnedAt = &pinnedAt
	edited.ColorSlot = ProjectColorSlotCount
	if err := catalog.UpsertProject(edited); err != nil {
		t.Fatalf("UpsertProject(edit): %v", err)
	}
	if got := catalog.Projects[0]; got.ColorSlot != originalSlot || !got.Pinned || got.Description != "edited" || got.Context != "work" {
		t.Fatalf("edit did not preserve lifecycle metadata and color: %+v", got)
	}

	archived := catalog.Projects[0]
	archived.Archived = true
	archivedAt := createdAt.Add(90 * time.Minute)
	archived.ArchivedAt = &archivedAt
	if err := catalog.UpsertProject(archived); err != nil {
		t.Fatalf("UpsertProject(archive): %v", err)
	}
	if got := catalog.Projects[0]; !got.Archived || got.ArchivedAt == nil || got.ColorSlot != originalSlot {
		t.Fatalf("archive did not preserve lifecycle metadata and color: %+v", got)
	}

	restored := catalog.Projects[0]
	restored.Archived = false
	restored.ArchivedAt = nil
	if err := catalog.UpsertProject(restored); err != nil {
		t.Fatalf("UpsertProject(restore): %v", err)
	}
	if got := catalog.Projects[0]; got.Archived || got.ArchivedAt != nil || got.ColorSlot != originalSlot {
		t.Fatalf("restore did not preserve lifecycle metadata and color: %+v", got)
	}

	if err := catalog.RemoveProject(edited.ID); err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}
	replacement := v5CatalogProject("replacement", createdAt.Add(2*time.Hour))
	if err := catalog.UpsertProject(replacement); err != nil {
		t.Fatalf("UpsertProject(replacement): %v", err)
	}
	if got := catalog.Projects[len(catalog.Projects)-1].ColorSlot; got != originalSlot {
		t.Fatalf("released slot = %d, want %d", got, originalSlot)
	}
}

func TestV5CatalogProtectedInboxLifecycle(t *testing.T) {
	createdAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	inbox := v5CatalogProject("inbox-id", createdAt)
	inbox.Slug = "inbox"
	inbox.Name = "Inbox"
	inbox.ColorSlot = 7
	catalog := V5WorkspaceCatalogCandidate{
		SchemaVersion:        V5ConfigSchemaVersion,
		WorkspaceID:          "11111111-1111-4111-8111-111111111111",
		Projects:             []V5ProjectCandidate{inbox},
		SystemInboxProjectID: inbox.ID,
	}

	rename := inbox
	rename.Name = "Renamed"
	rename.Slug = "renamed"
	if err := catalog.UpsertProject(rename); !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("UpsertProject(rename Inbox) error = %v, want ErrProtectedSystemUnfiled", err)
	}
	if got := catalog.Projects[0]; got.Name != "Inbox" || got.Slug != "inbox" {
		t.Fatalf("rejected rename mutated Inbox: %+v", got)
	}

	archive := inbox
	archive.Archived = true
	archivedAt := createdAt.Add(time.Hour)
	archive.ArchivedAt = &archivedAt
	if err := catalog.UpsertProject(archive); !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("UpsertProject(archive Inbox) error = %v, want ErrProtectedSystemUnfiled", err)
	}
	if err := catalog.RemoveProject(inbox.ID); !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("RemoveProject(Inbox) error = %v, want ErrProtectedSystemUnfiled", err)
	}

	edit := inbox
	edit.Description = "Workspace capture"
	edit.Context = "work"
	edit.Pinned = true
	pinnedAt := createdAt.Add(2 * time.Hour)
	edit.PinnedAt = &pinnedAt
	if err := catalog.UpsertProject(edit); err != nil {
		t.Fatalf("UpsertProject(allowed Inbox metadata): %v", err)
	}
	if got := catalog.Projects[0]; got.Description != edit.Description || got.Context != "work" || !got.Pinned || got.ColorSlot != 7 {
		t.Fatalf("allowed Inbox edit = %+v", got)
	}

	ordinary := v5CatalogProject("ordinary", createdAt.Add(3*time.Hour))
	ordinary.Name = "Inbox"
	if err := catalog.UpsertProject(ordinary); err != nil {
		t.Fatalf("display-name match was treated as protected identity: %v", err)
	}
	ordinary.Name = "Renamed Into Inbox"
	ordinary.Slug = "inbox"
	if err := catalog.UpsertProject(ordinary); err == nil {
		t.Fatal("ordinary project renamed into the protected Inbox slug")
	}
	if got := catalog.Projects[1]; got.Slug != "ordinary" {
		t.Fatalf("rejected protected-slug collision mutated ordinary project: %+v", got)
	}

	catalog.Projects[0].Archived = true
	catalog.Projects[0].ArchivedAt = &archivedAt
	if err := catalog.Validate(); !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("Validate(archived Inbox) error = %v, want ErrProtectedSystemUnfiled", err)
	}
}
