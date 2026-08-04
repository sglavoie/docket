package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

const (
	testCustomBoundaryID      = "44444444-4444-4444-8444-444444444444"
	testOtherCustomBoundaryID = "55555555-5555-4555-8555-555555555555"
)

// boundaryOperationCatalog seeds Active plus one custom boundary holding an
// active and an archived member, which is the shape every boundary operation
// has to keep consistent.
func boundaryOperationCatalog(t *testing.T) *WorkspaceCatalog {
	t.Helper()
	catalog := NewWorkspaceCatalog()
	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "On Hold"})
	archivedAt := time.Now()
	catalog.Projects = append(catalog.Projects,
		CatalogProject{ID: "live", Slug: "live", Name: "Live", ColorSlot: 1, BoundaryID: models.ActiveBoundaryID},
		CatalogProject{ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 2, BoundaryID: testCustomBoundaryID},
		CatalogProject{
			ID: "shelved", Slug: "shelved", Name: "Shelved", ColorSlot: 3,
			BoundaryID: testCustomBoundaryID, Archived: true, ArchivedAt: &archivedAt,
		},
	)
	if err := catalog.Validate(); err != nil {
		t.Fatalf("seed catalog must be valid: %v", err)
	}
	return catalog
}

func TestCreateBoundaryTrimsAndRejectsInvalidNames(t *testing.T) {
	catalog := boundaryOperationCatalog(t)

	created, err := catalog.CreateBoundary("  Someday  ")
	if err != nil {
		t.Fatalf("CreateBoundary: %v", err)
	}
	if created.Name != "Someday" {
		t.Fatalf("created name = %q, want %q", created.Name, "Someday")
	}
	if created.ID == "" || created.ID == models.ActiveBoundaryID {
		t.Fatalf("created boundary must carry its own durable id, got %q", created.ID)
	}
	if stored, ok := catalog.Boundary(created.ID); !ok || stored.Name != "Someday" {
		t.Fatalf("created boundary was not stored: %+v ok=%v", stored, ok)
	}
	if err = catalog.Validate(); err != nil {
		t.Fatalf("catalog must stay valid after a create: %v", err)
	}

	rejections := map[string]struct {
		name string
		want error
	}{
		"empty":             {name: "", want: ErrInvalidBoundaryName},
		"whitespace only":   {name: "   ", want: ErrInvalidBoundaryName},
		"too long":          {name: strings.Repeat("x", MaxBoundaryNameLength+1), want: ErrInvalidBoundaryName},
		"duplicate custom":  {name: "on hold", want: ErrBoundaryNameConflict},
		"duplicate active":  {name: "ACTIVE", want: ErrBoundaryNameConflict},
		"duplicate trimmed": {name: "  Someday", want: ErrBoundaryNameConflict},
	}
	for label, tc := range rejections {
		t.Run(label, func(t *testing.T) {
			before := len(catalog.Boundaries)
			if _, err := catalog.CreateBoundary(tc.name); !errors.Is(err, tc.want) {
				t.Fatalf("CreateBoundary(%q) = %v, want %v", tc.name, err, tc.want)
			}
			if len(catalog.Boundaries) != before {
				t.Fatal("a rejected create must not add a boundary")
			}
		})
	}
}

func TestRenameBoundaryKeepsMembershipAndProtectsActive(t *testing.T) {
	catalog := boundaryOperationCatalog(t)

	if err := catalog.RenameBoundary(testCustomBoundaryID, "  Paused  "); err != nil {
		t.Fatalf("RenameBoundary: %v", err)
	}
	renamed, _ := catalog.Boundary(testCustomBoundaryID)
	if renamed.Name != "Paused" {
		t.Fatalf("renamed name = %q, want %q", renamed.Name, "Paused")
	}
	if got := catalog.ProjectBoundaryID("parked"); got != testCustomBoundaryID {
		t.Fatalf("rename moved a member to %q", got)
	}

	// A rename that only repaints casing must not collide with itself.
	if err := catalog.RenameBoundary(testCustomBoundaryID, "PAUSED"); err != nil {
		t.Fatalf("case-only rename: %v", err)
	}

	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testOtherCustomBoundaryID, Name: "Someday"})
	rejections := map[string]struct {
		id, name string
		want     error
	}{
		"active rename":  {id: models.ActiveBoundaryID, name: "Live", want: ErrProtectedActiveBoundary},
		"unknown id":     {id: "no-such-boundary", name: "Fine", want: ErrBoundaryNotFound},
		"empty name":     {id: testCustomBoundaryID, name: "  ", want: ErrInvalidBoundaryName},
		"duplicate name": {id: testCustomBoundaryID, name: "someday", want: ErrBoundaryNameConflict},
	}
	for label, tc := range rejections {
		t.Run(label, func(t *testing.T) {
			before := slices.Clone(catalog.Boundaries)
			if err := catalog.RenameBoundary(tc.id, tc.name); !errors.Is(err, tc.want) {
				t.Fatalf("RenameBoundary(%q, %q) = %v, want %v", tc.id, tc.name, err, tc.want)
			}
			if !slices.Equal(before, catalog.Boundaries) {
				t.Fatal("a rejected rename must leave every boundary untouched")
			}
		})
	}
}

func TestDeleteBoundaryReassignsActiveAndArchivedMembers(t *testing.T) {
	catalog := boundaryOperationCatalog(t)

	moved, err := catalog.DeleteBoundary(testCustomBoundaryID, models.ActiveBoundaryID)
	if err != nil {
		t.Fatalf("DeleteBoundary: %v", err)
	}
	slices.Sort(moved)
	if want := []string{"parked", "shelved"}; !slices.Equal(moved, want) {
		t.Fatalf("moved = %v, want %v (archived members must move too)", moved, want)
	}
	if _, ok := catalog.Boundary(testCustomBoundaryID); ok {
		t.Fatal("deleted boundary is still defined")
	}
	for _, id := range []string{"parked", "shelved"} {
		if got := catalog.ProjectBoundaryID(id); got != models.ActiveBoundaryID {
			t.Fatalf("project %q boundary = %q, want %q", id, got, models.ActiveBoundaryID)
		}
	}
	if err = catalog.Validate(); err != nil {
		t.Fatalf("catalog must stay valid after a delete: %v", err)
	}
}

func TestDeleteBoundaryRejectionsLeaveCatalogUntouched(t *testing.T) {
	rejections := map[string]struct {
		id, destination string
		want            error
	}{
		"active boundary":     {id: models.ActiveBoundaryID, destination: testCustomBoundaryID, want: ErrProtectedActiveBoundary},
		"unknown boundary":    {id: "no-such-boundary", destination: models.ActiveBoundaryID, want: ErrBoundaryNotFound},
		"self destination":    {id: testCustomBoundaryID, destination: testCustomBoundaryID, want: ErrBoundaryDestinationInvalid},
		"unknown destination": {id: testCustomBoundaryID, destination: "no-such-boundary", want: ErrBoundaryNotFound},
		"empty destination":   {id: testCustomBoundaryID, destination: "", want: ErrBoundaryNotFound},
	}
	for label, tc := range rejections {
		t.Run(label, func(t *testing.T) {
			catalog := boundaryOperationCatalog(t)
			boundaries := slices.Clone(catalog.Boundaries)
			projects := slices.Clone(catalog.Projects)

			moved, err := catalog.DeleteBoundary(tc.id, tc.destination)
			if !errors.Is(err, tc.want) {
				t.Fatalf("DeleteBoundary(%q, %q) = %v, want %v", tc.id, tc.destination, err, tc.want)
			}
			if moved != nil {
				t.Fatalf("a rejected delete reported %v as moved", moved)
			}
			if !slices.Equal(boundaries, catalog.Boundaries) {
				t.Fatal("a rejected delete changed the boundary definitions")
			}
			if !slices.Equal(projects, catalog.Projects) {
				t.Fatal("a rejected delete changed project membership")
			}
		})
	}
}

func TestAssignProjectBoundaryReportsChangeAndFailsClosed(t *testing.T) {
	catalog := boundaryOperationCatalog(t)

	changed, err := catalog.AssignProjectBoundary("live", testCustomBoundaryID)
	if err != nil || !changed {
		t.Fatalf("AssignProjectBoundary = (%v, %v), want (true, nil)", changed, err)
	}
	if got := catalog.ProjectBoundaryID("live"); got != testCustomBoundaryID {
		t.Fatalf("assigned boundary = %q, want %q", got, testCustomBoundaryID)
	}

	changed, err = catalog.AssignProjectBoundary("live", testCustomBoundaryID)
	if err != nil || changed {
		t.Fatalf("reassigning the same boundary = (%v, %v), want (false, nil)", changed, err)
	}

	rejections := map[string]struct {
		projectID, boundaryID string
		want                  error
	}{
		"unknown project":  {projectID: "no-such-project", boundaryID: models.ActiveBoundaryID, want: ErrCatalogProjectNotFound},
		"unknown boundary": {projectID: "live", boundaryID: "no-such-boundary", want: ErrBoundaryNotFound},
		"empty boundary":   {projectID: "live", boundaryID: "", want: ErrBoundaryNotFound},
	}
	for label, tc := range rejections {
		t.Run(label, func(t *testing.T) {
			projects := slices.Clone(catalog.Projects)
			changed, err := catalog.AssignProjectBoundary(tc.projectID, tc.boundaryID)
			if !errors.Is(err, tc.want) {
				t.Fatalf("AssignProjectBoundary(%q, %q) = %v, want %v", tc.projectID, tc.boundaryID, err, tc.want)
			}
			if changed {
				t.Fatal("a rejected assignment reported a change")
			}
			if !slices.Equal(projects, catalog.Projects) {
				t.Fatal("a rejected assignment changed project membership")
			}
		})
	}
}

func TestAssignProjectBoundaryKeepsUnfiledInActive(t *testing.T) {
	catalog := boundaryOperationCatalog(t)
	unfiled, err := models.NewProject("unfiled-id", SystemUnfiledName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.RepairSystemUnfiledProject(unfiled); err != nil {
		t.Fatalf("RepairSystemUnfiledProject: %v", err)
	}

	changed, err := catalog.AssignProjectBoundary("unfiled-id", testCustomBoundaryID)
	if !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("parking Unfiled = %v, want ErrProtectedSystemUnfiled", err)
	}
	if changed {
		t.Fatal("a rejected assignment reported a change")
	}
	if got := catalog.ProjectBoundaryID("unfiled-id"); got != models.ActiveBoundaryID {
		t.Fatalf("Unfiled boundary = %q, want %q", got, models.ActiveBoundaryID)
	}
}

// A deleted boundary's members must survive the delete inside the destination,
// which is what keeps Unfiled reachable when its neighbors are reorganized.
func TestDeleteBoundaryMovesMembersBetweenCustomBoundaries(t *testing.T) {
	catalog := boundaryOperationCatalog(t)
	destination, err := catalog.CreateBoundary("Someday")
	if err != nil {
		t.Fatalf("CreateBoundary: %v", err)
	}

	moved, err := catalog.DeleteBoundary(testCustomBoundaryID, destination.ID)
	if err != nil {
		t.Fatalf("DeleteBoundary: %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("moved %d projects, want 2", len(moved))
	}
	if got := catalog.ProjectBoundaryID("parked"); got != destination.ID {
		t.Fatalf("member landed in %q, want %q", got, destination.ID)
	}
	if err = catalog.Validate(); err != nil {
		t.Fatalf("catalog must stay valid after a custom-to-custom delete: %v", err)
	}
}

func TestNewWorkspaceCatalogSeedsProtectedActiveBoundary(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	active, ok := catalog.ActiveBoundary()
	if !ok {
		t.Fatal("a new catalog must define the reserved Active boundary")
	}
	if active.Name != ActiveBoundaryName {
		t.Fatalf("Active boundary name = %q, want %q", active.Name, ActiveBoundaryName)
	}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("a freshly seeded catalog must validate: %v", err)
	}
}

// A workspace written before boundaries existed carries neither a boundaries
// list nor per-project boundary IDs. It must load as though every project were
// Active, and merely opening it must not rewrite the artifact.
func TestLegacyCatalogNormalizesToActiveWithoutWriting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	path := WorkspaceCatalogPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatal(err)
	}

	legacyPayload := `{
		"schema_version": ` + strconv.Itoa(CurrentConfigVersion) + `,
		"workspace_id": "` + testWorkspaceID + `",
		"projects": [{"id": "legacy", "slug": "legacy", "name": "Legacy", "context": "personal", "color_slot": 1}],
		"system_unfiled_project_id": "legacy"
	}`
	raw, err := json.Marshal(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   testWorkspaceID,
		Kind:          domainCatalog.name(),
		Data:          json.RawMessage(legacyPayload),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, FilePermissions); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := NewDomainFileStore(dataDir).LoadCatalog()
	if err != nil {
		t.Fatalf("legacy catalog must remain loadable: %v", err)
	}
	if _, ok := loaded.ActiveBoundary(); !ok {
		t.Fatal("load normalization did not restore the Active boundary")
	}
	if got := loaded.ProjectBoundaryID("legacy"); got != models.ActiveBoundaryID {
		t.Fatalf("legacy project boundary = %q, want %q", got, models.ActiveBoundaryID)
	}
	if err = loaded.Validate(); err != nil {
		t.Fatalf("normalized legacy catalog must satisfy strict validation: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("loading a legacy catalog rewrote the artifact; normalization must stay in memory")
	}
}

func TestCatalogBoundaryValidationFailsClosed(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*WorkspaceCatalog)
	}{
		{
			name:    "missing active boundary",
			corrupt: func(c *WorkspaceCatalog) { c.Boundaries = nil },
		},
		{
			name:    "renamed active boundary",
			corrupt: func(c *WorkspaceCatalog) { c.Boundaries[0].Name = "Live" },
		},
		{
			name: "duplicate boundary id",
			corrupt: func(c *WorkspaceCatalog) {
				c.Boundaries = append(c.Boundaries, ProjectBoundary{ID: models.ActiveBoundaryID, Name: "Other"})
			},
		},
		{
			name: "case-insensitively duplicate boundary name",
			corrupt: func(c *WorkspaceCatalog) {
				c.Boundaries = append(c.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "aCtIvE"})
			},
		},
		{
			name: "empty boundary id",
			corrupt: func(c *WorkspaceCatalog) {
				c.Boundaries = append(c.Boundaries, ProjectBoundary{Name: "On Hold"})
			},
		},
		{
			name: "empty boundary name",
			corrupt: func(c *WorkspaceCatalog) {
				c.Boundaries = append(c.Boundaries, ProjectBoundary{ID: testCustomBoundaryID})
			},
		},
		{
			name: "untrimmed boundary name",
			corrupt: func(c *WorkspaceCatalog) {
				c.Boundaries = append(c.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: " On Hold "})
			},
		},
		{
			name: "overlong boundary name",
			corrupt: func(c *WorkspaceCatalog) {
				c.Boundaries = append(c.Boundaries, ProjectBoundary{
					ID:   testCustomBoundaryID,
					Name: strings.Repeat("x", MaxBoundaryNameLength+1),
				})
			},
		},
		{
			name:    "project references unknown boundary",
			corrupt: func(c *WorkspaceCatalog) { c.Projects[0].BoundaryID = "no-such-boundary" },
		},
		{
			name:    "project has no boundary",
			corrupt: func(c *WorkspaceCatalog) { c.Projects[0].BoundaryID = "" },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog := NewWorkspaceCatalog()
			catalog.Projects = append(catalog.Projects, CatalogProject{
				ID: "a", Slug: "a", Name: "A", BoundaryID: models.ActiveBoundaryID, ColorSlot: 1,
			})
			if err := catalog.Validate(); err != nil {
				t.Fatalf("baseline catalog must be valid: %v", err)
			}
			tc.corrupt(catalog)
			if err := catalog.Validate(); err == nil {
				t.Fatal("Validate accepted malformed boundary data")
			}
		})
	}
}

// A catalog that arrives by hand edit, file sync, or a foreign writer must be
// judged by exactly the rules CreateBoundary and RenameBoundary enforce.
// Otherwise it can carry a custom identity creation could never mint —
// including one shaped like an internal presentation sentinel — or a name pair
// the mutation API refuses as duplicate.
func TestWorkspaceCatalogRejectsInvalidBoundaryIdentifiersAndEqualFoldCollisions(t *testing.T) {
	// Greek final sigma and sigma fold to each other but lowercase apart, which
	// is precisely the pair a strings.ToLower key admits and strings.EqualFold
	// refuses.
	const (
		sigmaName = "Σίσυφος"
		twinName  = "Σίσυφοσ"
	)
	loweredSigma := strings.ToLower(sigmaName)
	loweredTwin := strings.ToLower(twinName)
	if !strings.EqualFold(sigmaName, twinName) || loweredSigma == loweredTwin {
		t.Fatalf("fixture no longer separates EqualFold from ToLower: %q vs %q", loweredSigma, loweredTwin)
	}

	// The Project Hub keys its own filter controls in this namespace, so a
	// persisted boundary carrying one would hijack the control it collides with.
	const sentinelShapedID = "project-hub:filter:archived"

	t.Run("load rejects a sentinel-shaped custom boundary id", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: sentinelShapedID, Name: "Parked"})
		catalog.Projects = append(catalog.Projects, CatalogProject{
			ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 1, BoundaryID: sentinelShapedID,
		})
		assertCatalogArtifactFailsLoadUntouched(t, catalog)
	})

	t.Run("load rejects an EqualFold duplicate name pair", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		catalog.Boundaries = append(catalog.Boundaries,
			ProjectBoundary{ID: testCustomBoundaryID, Name: sigmaName},
			ProjectBoundary{ID: testOtherCustomBoundaryID, Name: twinName},
		)
		assertCatalogArtifactFailsLoadUntouched(t, catalog)
	})

	t.Run("only the canonical uuid spelling is a supported identity", func(t *testing.T) {
		malformed := []string{
			sentinelShapedID,
			"on-hold",
			"44444444444444448444444444444444",
			"{44444444-4444-4444-8444-444444444444}",
			"urn:uuid:44444444-4444-4444-8444-444444444444",
			"44444444-4444-4444-8444-44444444444A",
		}
		for _, id := range malformed {
			t.Run(id, func(t *testing.T) {
				catalog := NewWorkspaceCatalog()
				catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: id, Name: "On Hold"})
				if err := catalog.validateForLoad(); err == nil {
					t.Fatal("load validation accepted an unsupported boundary identity")
				}
				if err := catalog.Validate(); err == nil {
					t.Fatal("save validation accepted an unsupported boundary identity")
				}
				if got := catalog.Boundaries[1].ID; got != id {
					t.Fatalf("rejected validation rewrote the identity to %q", got)
				}
			})
		}
	})

	t.Run("create and rename refuse what load refuses", func(t *testing.T) {
		catalog := NewWorkspaceCatalog()
		created, err := catalog.CreateBoundary(sigmaName)
		if err != nil {
			t.Fatalf("CreateBoundary: %v", err)
		}
		if _, err = catalog.CreateBoundary(twinName); !errors.Is(err, ErrBoundaryNameConflict) {
			t.Fatalf("CreateBoundary(fold twin) = %v, want ErrBoundaryNameConflict", err)
		}
		other, err := catalog.CreateBoundary("On Hold")
		if err != nil {
			t.Fatalf("CreateBoundary: %v", err)
		}
		if err = catalog.RenameBoundary(other.ID, twinName); !errors.Is(err, ErrBoundaryNameConflict) {
			t.Fatalf("RenameBoundary(fold twin) = %v, want ErrBoundaryNameConflict", err)
		}

		// A boundary the supported API did mint must still load and save.
		if err = catalog.validateForLoad(); err != nil {
			t.Fatalf("created boundaries must pass load validation: %v", err)
		}
		if err = catalog.Validate(); err != nil {
			t.Fatalf("created boundaries must pass save validation: %v", err)
		}
		if err = validateCustomBoundaryID(created.ID); err != nil {
			t.Fatalf("CreateBoundary minted an identity load validation refuses: %v", err)
		}
	})
}

// assertCatalogArtifactFailsLoadUntouched writes catalog to a fresh workspace
// without validating it, then requires the real load path to reject it and
// leave the artifact byte-identical: malformed boundary data fails closed
// rather than being normalized or reassigned on the way in.
func assertCatalogArtifactFailsLoadUntouched(t *testing.T, catalog *WorkspaceCatalog) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	path := WorkspaceCatalogPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatal(err)
	}

	catalog.WorkspaceID = testWorkspaceID
	payload, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   testWorkspaceID,
		Kind:          domainCatalog.name(),
		Data:          json.RawMessage(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, FilePermissions); err != nil {
		t.Fatal(err)
	}

	if _, err = NewDomainFileStore(dataDir).LoadCatalog(); err == nil {
		t.Fatal("LoadCatalog accepted boundary data the mutation API would refuse")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(raw) {
		t.Fatal("a rejected load rewrote the catalog artifact")
	}
}

// Normalization fills in absent data only. A boundary ID that is present but
// dangling is corruption, so it must reach validation rather than be silently
// reassigned to Active.
func TestNormalizeBoundariesLeavesDanglingReferenceForValidation(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	catalog.Projects = append(catalog.Projects, CatalogProject{
		ID: "a", Slug: "a", Name: "A", BoundaryID: "no-such-boundary", ColorSlot: 1,
	})
	if catalog.NormalizeBoundaries() {
		t.Fatal("NormalizeBoundaries reported a change for a dangling reference")
	}
	if got := catalog.Projects[0].BoundaryID; got != "no-such-boundary" {
		t.Fatalf("dangling boundary was rewritten to %q", got)
	}
	if err := catalog.Validate(); err == nil {
		t.Fatal("a dangling boundary reference must fail validation")
	}
}

func TestUpsertProjectDefaultsAndRejectsUnknownBoundary(t *testing.T) {
	catalog := NewWorkspaceCatalog()

	project, err := models.NewProject("p1", "One", "")
	if err != nil {
		t.Fatal(err)
	}
	project.BoundaryID = ""
	if err := catalog.UpsertProject(project); err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if project.BoundaryID != models.ActiveBoundaryID {
		t.Fatalf("caller model boundary = %q, want %q", project.BoundaryID, models.ActiveBoundaryID)
	}
	if got := catalog.ProjectBoundaryID("p1"); got != models.ActiveBoundaryID {
		t.Fatalf("stored boundary = %q, want %q", got, models.ActiveBoundaryID)
	}

	project.BoundaryID = "no-such-boundary"
	if err := catalog.UpsertProject(project); err == nil {
		t.Fatal("UpsertProject accepted an unknown boundary")
	}
	if got := catalog.ProjectBoundaryID("p1"); got != models.ActiveBoundaryID {
		t.Fatalf("rejected upsert changed the stored boundary to %q", got)
	}
}

func TestSystemUnfiledStaysInActiveBoundary(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "On Hold"})

	unfiled, err := models.NewProject("unfiled-id", SystemUnfiledName, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.RepairSystemUnfiledProject(unfiled); err != nil {
		t.Fatalf("RepairSystemUnfiledProject: %v", err)
	}
	if got := catalog.ProjectBoundaryID("unfiled-id"); got != models.ActiveBoundaryID {
		t.Fatalf("repaired Unfiled boundary = %q, want %q", got, models.ActiveBoundaryID)
	}

	unfiled.BoundaryID = testCustomBoundaryID
	err = catalog.UpsertProject(unfiled)
	if !errors.Is(err, ErrProtectedSystemUnfiled) {
		t.Fatalf("moving Unfiled out of Active = %v, want ErrProtectedSystemUnfiled", err)
	}
	if got := catalog.ProjectBoundaryID("unfiled-id"); got != models.ActiveBoundaryID {
		t.Fatalf("rejected move changed the stored boundary to %q", got)
	}
}

// Bootstrap repair runs before SystemUnfiledProjectID is bound, so it must force
// Active rather than rely on the protected-project branch recognizing the ID.
func TestRepairSystemUnfiledForcesActiveBoundary(t *testing.T) {
	catalog := NewWorkspaceCatalog()
	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "On Hold"})

	unfiled, err := models.NewProject("unfiled-id", SystemUnfiledName, "")
	if err != nil {
		t.Fatal(err)
	}
	unfiled.BoundaryID = testCustomBoundaryID
	if err := catalog.RepairSystemUnfiledProject(unfiled); err != nil {
		t.Fatalf("RepairSystemUnfiledProject: %v", err)
	}
	if got := catalog.ProjectBoundaryID("unfiled-id"); got != models.ActiveBoundaryID {
		t.Fatalf("bootstrap repair boundary = %q, want %q", got, models.ActiveBoundaryID)
	}
}

func TestCustomBoundaryRoundTripsThroughStore(t *testing.T) {
	store := newIsolatedStore(t)

	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	catalog.Boundaries = append(catalog.Boundaries, ProjectBoundary{ID: testCustomBoundaryID, Name: "On Hold"})
	catalog.Projects = append(catalog.Projects, CatalogProject{
		ID: "parked", Slug: "parked", Name: "Parked", ColorSlot: 2, BoundaryID: testCustomBoundaryID,
	})
	if err = store.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	reloaded, err := store.LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	boundary, ok := reloaded.Boundary(testCustomBoundaryID)
	if !ok {
		t.Fatal("custom boundary did not survive the round trip")
	}
	if boundary.Name != "On Hold" {
		t.Fatalf("boundary name = %q, want %q", boundary.Name, "On Hold")
	}
	if got := reloaded.ProjectBoundaryID("parked"); got != testCustomBoundaryID {
		t.Fatalf("project boundary = %q, want %q", got, testCustomBoundaryID)
	}
}

func TestCatalogModelConversionPreservesBoundary(t *testing.T) {
	source := &models.Project{ID: "p1", Slug: "p1", Name: "P1", BoundaryID: testCustomBoundaryID}
	catalogProject := catalogProjectFromModel(source)
	if catalogProject.BoundaryID != testCustomBoundaryID {
		t.Fatalf("model-to-catalog boundary = %q, want %q", catalogProject.BoundaryID, testCustomBoundaryID)
	}
	if got := catalogProject.ToModel("/data").BoundaryID; got != testCustomBoundaryID {
		t.Fatalf("catalog-to-model boundary = %q, want %q", got, testCustomBoundaryID)
	}
}
