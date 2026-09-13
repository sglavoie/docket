package app

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// CheckInSetupSource reads the entire active task workspace from fresh catalog
// and project documents. Any unreadable active owner fails the read, so setup
// never presents a partial census as complete. Returned models are owned copies.
type CheckInSetupSource interface {
	CheckInSetupCensus() (WorkspaceCensus, error)
}

// CheckInSetupCandidate is a stable, qualified identity plus display-only text.
type CheckInSetupCandidate struct {
	Locator     TaskLocator
	ProjectName string
	ProjectSlug string
	Title       string
	ShortID     int
}

func checkInSetupEligible(project *models.Project, task *models.Task) bool {
	return project != nil && !project.Archived && !models.ProjectTagsParked(project.Tags) &&
		task != nil && task.CheckIn().IsEmpty() &&
		(task.State() == models.StateDoing || task.State() == models.StateWaiting)
}

func (s *Service) checkInSetupCensus() (WorkspaceCensus, error) {
	source, ok := s.repo.(CheckInSetupSource)
	if !ok {
		return WorkspaceCensus{}, fmt.Errorf("check-in setup requires a complete active workspace read")
	}
	return source.CheckInSetupCensus()
}

// CheckInSetupCandidates freezes a deterministic pass independent of view
// scope, task folds, sorting, and lenses. Calling again starts a new census.
func (s *Service) CheckInSetupCandidates() ([]CheckInSetupCandidate, error) {
	census, err := s.checkInSetupCensus()
	if err != nil {
		return nil, err
	}
	var candidates []CheckInSetupCandidate
	seen := make(map[TaskLocator]bool)
	for _, entry := range census.Projects {
		p := entry.Project
		if p == nil {
			continue
		}
		for _, task := range p.Tasks {
			if !checkInSetupEligible(p, task) {
				continue
			}
			locator := TaskLocator{ProjectID: p.ID, TaskID: task.ID}
			if !locator.Valid() || seen[locator] {
				continue
			}
			seen[locator] = true
			candidates = append(candidates, CheckInSetupCandidate{locator, p.Name, p.Slug, task.Title, task.ShortID})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.ProjectSlug != b.ProjectSlug {
			return a.ProjectSlug < b.ProjectSlug
		}
		if a.Locator.ProjectID != b.Locator.ProjectID {
			return a.Locator.ProjectID < b.Locator.ProjectID
		}
		if a.ShortID != b.ShortID {
			return a.ShortID < b.ShortID
		}
		return a.Locator.TaskID < b.Locator.TaskID
	})
	return candidates, nil
}

func setupTarget(census WorkspaceCensus, locator TaskLocator) (CensusProject, *models.Task) {
	for _, entry := range census.Projects {
		if entry.Project == nil || entry.Project.ID != locator.ProjectID {
			continue
		}
		task := entry.Project.GetTaskByID(locator.TaskID)
		if checkInSetupEligible(entry.Project, task) {
			return entry, task
		}
	}
	return CensusProject{}, nil
}

// ResolveCheckInSetup refreshes one original identity. A missing/ineligible
// target is a skip; an unreadable workspace is an error, never a skip.
func (s *Service) ResolveCheckInSetup(locator TaskLocator) (CheckInSetupCandidate, bool, error) {
	census, err := s.checkInSetupCensus()
	if err != nil {
		return CheckInSetupCandidate{}, false, err
	}
	entry, task := setupTarget(census, locator)
	if task == nil {
		return CheckInSetupCandidate{}, false, nil
	}
	p := entry.Project
	return CheckInSetupCandidate{locator, p.Name, p.Slug, task.Title, task.ShortID}, true, nil
}

// SaveCheckInSetup writes exactly one task and one undo entry. The boolean
// reports a concurrent loss of eligibility, which advances without a write.
// Both task and catalog revisions are strict guards: a parked owner or a newly
// assigned schedule cannot be merged past the eligibility decision.
func (s *Service) SaveCheckInSetup(locator TaskLocator, schedule models.CheckInSchedule) (TaskResult, bool, error) {
	if !locator.Valid() {
		return TaskResult{}, false, invalidInput("task", "qualified task required")
	}
	if err := models.ValidateCheckInSchedule(schedule); err != nil {
		return TaskResult{}, false, err
	}
	if schedule.IsEmpty() {
		return TaskResult{}, false, invalidInput("check-in", "choose a date or Skip")
	}
	census, err := s.checkInSetupCensus()
	if err != nil {
		return TaskResult{}, false, err
	}
	entry, task := setupTarget(census, locator)
	if task == nil {
		return TaskResult{}, true, nil
	}
	if err := applyCheckInSchedule(task, schedule, s.clock.Now()); err != nil {
		return TaskResult{}, false, err
	}
	commits := []projectCommit{{project: entry.Project, expectedRevision: entry.Revision, base: entry.Document}}
	if err := attachReadSetRevisionGuards(commits); err != nil {
		return TaskResult{}, false, err
	}
	if census.CatalogPath != "" {
		commits[0].artifactPreconditions = []storage.ArtifactPrecondition{{Path: census.CatalogPath, ExpectedRevision: census.CatalogRevision}}
	}
	action, context := checkInUndoEntry(schedule)
	outcome, err := s.commitProjects(commits, action, context)
	if err != nil {
		return TaskResult{}, false, err
	}
	return newTaskResult(TaskRef{Task: task, Project: entry.Project, Base: entry.Document}, revisionAt(outcome.results, 0), transactionIDAt(outcome.results, 0)), false, nil
}
