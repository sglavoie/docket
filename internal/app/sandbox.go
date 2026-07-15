package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// SandboxResetCounts is the presentation-neutral inventory cleared or restored
// to defaults by one sandbox reset transaction. SectionLayout and
// ShortIDCounter are zero-or-one state changes; SectionColors counts mappings.
type SandboxResetCounts struct {
	Tasks          int
	Notes          int
	Tags           int
	PinnedTags     int
	SectionLayout  int
	SectionColors  int
	ShortIDCounter int
}

// Total returns the total number of user-visible items represented by c.
func (c SandboxResetCounts) Total() int {
	return c.Tasks + c.Notes + c.Tags + c.PinnedTags + c.SectionLayout + c.SectionColors + c.ShortIDCounter
}

// Empty reports whether a sandbox reset would have no user-visible effect.
func (c SandboxResetCounts) Empty() bool { return c.Total() == 0 }

// SandboxResetPreview reports the current reset inventory without mutating it.
// Adapters use it for confirmation text, then trust the committed result's
// independently refreshed counts for success output.
type SandboxResetPreview struct {
	ProjectID   string
	ProjectName string
	Counts      SandboxResetCounts
}

// SandboxResetResult reports exactly what the committed transaction cleared.
type SandboxResetResult struct {
	ProjectID     string
	ProjectName   string
	Cleared       SandboxResetCounts
	Revision      string
	TransactionID string
}

type sandboxResetPlan struct {
	snapshot Snapshot
	project  *models.Project
	revision string
	notes    []*models.Note
	counts   SandboxResetCounts
}

// PreviewSandboxReset loads the same revision-aware project and note inventory
// ResetSandbox will use, without changing either. Unreadable note artifacts are
// an error rather than being silently omitted from a purported whole reset.
func (s *Service) PreviewSandboxReset(projectID string) (SandboxResetPreview, error) {
	plan, err := s.loadSandboxResetPlan(projectID)
	if err != nil {
		return SandboxResetPreview{}, err
	}
	return SandboxResetPreview{
		ProjectID:   plan.project.ID,
		ProjectName: plan.project.Name,
		Counts:      plan.counts,
	}, nil
}

// ResetSandbox clears every task-domain field and every observed sandbox note
// artifact in one revision-checked ChangeSet. Each note delete carries the raw
// revision loaded before planning; a concurrent note edit therefore aborts the
// entire transaction before tasks.md or any note changes. The project mutation
// is built on a copy so neither a rejected nor a successful background commit
// can mutate a presentation cache that happens to share the snapshot object.
func (s *Service) ResetSandbox(projectID string) (SandboxResetResult, error) {
	plan, err := s.loadSandboxResetPlan(projectID)
	if err != nil {
		return SandboxResetResult{}, err
	}

	location := ProjectNoteLocation(plan.project.ID)
	artifacts := make([]storage.ArtifactChange, 0, len(plan.notes))
	for _, note := range plan.notes {
		if note == nil || strings.TrimSpace(note.RawRevision) == "" {
			return SandboxResetResult{}, fmt.Errorf("app: sandbox note %q has no observed revision", noteFilename(note))
		}
		change, encodeErr := s.notes.EncodeNoteDelete(location, plan.project, note.Filename)
		if encodeErr != nil {
			return SandboxResetResult{}, encodeErr
		}
		expected := note.RawRevision
		change.ExpectedRevision = &expected
		artifacts = append(artifacts, change)
	}

	reset := *plan.project
	reset.Tasks = []*models.Task{}
	reset.NextShortID = 1
	reset.Sections = []string{models.SectionNext}
	reset.SectionColors = make(map[string]string)
	reset.Tags = []string{}
	reset.PinnedTags = []string{}

	results, err := s.commitProjects([]projectCommit{{
		project:          &reset,
		expectedRevision: plan.revision,
		base:             plan.snapshot.Document(plan.project.ID),
		artifacts:        artifacts,
	}}, "Reset Sandbox", plan.project.Name)
	if err != nil {
		return SandboxResetResult{}, err
	}

	result := SandboxResetResult{
		ProjectID:   reset.ID,
		ProjectName: reset.Name,
		Cleared:     plan.counts,
	}
	if len(results) > 0 && results[0] != nil {
		result.Revision = results[0].Revision
		result.TransactionID = results[0].TransactionID
	}
	return result, nil
}

func (s *Service) loadSandboxResetPlan(projectID string) (sandboxResetPlan, error) {
	snapshot, project, revision, err := s.resolveProject(projectID)
	if err != nil {
		return sandboxResetPlan{}, err
	}
	if !project.Sandbox {
		return sandboxResetPlan{}, invalidInput("sandbox", "project is not the workspace sandbox")
	}
	notes, err := s.notes.NotesForMutation(ProjectNoteLocation(project.ID), project)
	if err != nil {
		return sandboxResetPlan{}, fmt.Errorf("load sandbox notes: %w", err)
	}
	return sandboxResetPlan{
		snapshot: snapshot,
		project:  project,
		revision: revision,
		notes:    notes,
		counts: SandboxResetCounts{
			Tasks:          len(project.Tasks),
			Notes:          len(notes),
			Tags:           len(project.AllTags()),
			PinnedTags:     len(project.PinnedTagList()),
			SectionLayout:  sandboxSectionLayoutChange(project),
			SectionColors:  len(project.SectionColors),
			ShortIDCounter: boolCount(project.NextShortID != 1),
		},
	}, nil
}

func sandboxSectionLayoutChange(project *models.Project) int {
	return boolCount(len(project.Sections) != 1 || project.Sections[0] != models.SectionNext)
}

func boolCount(changed bool) int {
	if changed {
		return 1
	}
	return 0
}

func noteFilename(note *models.Note) string {
	if note == nil {
		return "<nil>"
	}
	return note.Filename
}
