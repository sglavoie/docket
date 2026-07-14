package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// refreshOnboardingProgress derives newly observed milestones from the durable
// workspace and persists only monotonic advances. This also upgrades existing
// workspaces on first launch without inventing a separate migration marker.
func (m *AppModel) refreshOnboardingProgress(startupLoads map[string]error) {
	if m == nil || m.runtime == nil {
		return
	}
	before := m.runtime.Settings().GetOnboardingProgress()
	after := before

	for _, project := range m.activeProjects() {
		if project == nil {
			continue
		}
		isInbox := m.runtime.IsSystemInboxProject(project.ID)
		if !isInbox {
			after.ProjectCreated = true
		}
		if m.persister != nil {
			if err := m.loadStartupProject(project, startupLoads); err != nil {
				logging.Warn("onboarding: failed to load project %s: %v", project.Name, err)
				continue
			}
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			after.TaskCaptured = true
			if !isInbox {
				after.InboxTriaged = true
			}
			if task.IsComplete() {
				after.TaskCompleted = true
			}
		}
	}
	if len(m.runtime.Planning().DailyPlan) > 0 {
		after.TaskPlanned = true
	}

	m.advanceOnboarding(func(progress *config.OnboardingProgress) {
		progress.ProjectCreated = progress.ProjectCreated || after.ProjectCreated
		progress.TaskCaptured = progress.TaskCaptured || after.TaskCaptured
		progress.InboxTriaged = progress.InboxTriaged || after.InboxTriaged
		progress.TaskPlanned = progress.TaskPlanned || after.TaskPlanned
		progress.TaskCompleted = progress.TaskCompleted || after.TaskCompleted
	})
}

// advanceOnboarding persists an event-owned monotonic milestone update without
// reloading project data. Mutation handlers use this path so their established
// persistence/load contracts remain unchanged.
func (m *AppModel) advanceOnboarding(mutate func(*config.OnboardingProgress)) {
	if m == nil || m.runtime == nil || mutate == nil {
		return
	}
	before := m.runtime.Settings().GetOnboardingProgress()
	after := before
	mutate(&after)
	if after == before {
		return
	}
	if err := m.commitSettingsMutation(func(settings *config.Settings) {
		current := settings.GetOnboardingProgress()
		current.ProjectCreated = current.ProjectCreated || after.ProjectCreated
		current.TaskCaptured = current.TaskCaptured || after.TaskCaptured
		current.InboxTriaged = current.InboxTriaged || after.InboxTriaged
		current.TaskPlanned = current.TaskPlanned || after.TaskPlanned
		current.TaskCompleted = current.TaskCompleted || after.TaskCompleted
		settings.SetOnboardingProgress(current)
	}); err != nil {
		logging.Warn("failed to persist onboarding progress: %v", err)
	}
}

// observeLoadedProjectMilestones advances milestones from projects a mutation
// handler has already reloaded, avoiding any additional persistence reads.
func (m *AppModel) observeLoadedProjectMilestones(projectIDs ...string) {
	if m == nil || m.runtime == nil {
		return
	}
	m.advanceOnboarding(func(progress *config.OnboardingProgress) {
		if len(m.runtime.Planning().DailyPlan) > 0 {
			progress.TaskPlanned = true
		}
		for _, projectID := range projectIDs {
			project := m.projectByID(projectID)
			if project == nil {
				continue
			}
			isInbox := m.runtime.IsSystemInboxProject(project.ID)
			if !isInbox {
				progress.ProjectCreated = true
			}
			for _, task := range project.Tasks {
				if task == nil {
					continue
				}
				progress.TaskCaptured = true
				if !isInbox {
					progress.InboxTriaged = true
				}
				if task.IsComplete() {
					progress.TaskCompleted = true
				}
			}
		}
	})
}

func onboardingStatus(progress config.OnboardingProgress) string {
	if progress.Complete() {
		return ""
	}
	steps := []struct {
		name string
		key  string
		done bool
	}{
		{name: "create", key: "n", done: progress.ProjectCreated},
		{name: "capture", key: "a/i/;i", done: progress.TaskCaptured},
		{name: "triage", key: "ctrl+i", done: progress.InboxTriaged},
		{name: "plan", key: ";q", done: progress.TaskPlanned},
		{name: "complete", key: "x", done: progress.TaskCompleted},
	}
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		mark := "○"
		if step.done {
			mark = "✓"
		}
		parts = append(parts, fmt.Sprintf("%s %s(%s)", mark, step.name, step.key))
	}
	return fmt.Sprintf("Setup %d/5 · %s", progress.CompletedCount(), strings.Join(parts, " · "))
}
