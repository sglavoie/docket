package tui

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

const machineDomainRepairRetryDelay = time.Second

type machineProjectReference struct {
	ID   string
	Slug string
}

// machineDomainRepair is an idempotent patch, not a captured MachineState
// snapshot. Retrying it against a freshly loaded machine domain preserves
// navigation, history, and preferences written after the original lifecycle
// operation.
type machineDomainRepair struct {
	removeProjects         []machineProjectReference
	clearCurrentProjectIDs []string
	removeTaskLocators     []config.ProjectTaskLocator
}

func machineRepairForArchivedProject(projectID string, locators []config.ProjectTaskLocator) machineDomainRepair {
	return machineDomainRepair{
		clearCurrentProjectIDs: []string{projectID},
		removeTaskLocators:     slices.Clone(locators),
	}
}

func machineRepairForRemovedProject(projectID, slug string) machineDomainRepair {
	return machineDomainRepair{
		removeProjects: []machineProjectReference{{ID: projectID, Slug: slug}},
	}
}

func machineRepairForArchivedTasks(locators []config.ProjectTaskLocator) machineDomainRepair {
	return machineDomainRepair{removeTaskLocators: slices.Clone(locators)}
}

func (r machineDomainRepair) empty() bool {
	return len(r.removeProjects) == 0 &&
		len(r.clearCurrentProjectIDs) == 0 &&
		len(r.removeTaskLocators) == 0
}

func (r machineDomainRepair) apply(machine *config.MachineState) {
	if machine == nil {
		return
	}
	for _, project := range r.removeProjects {
		machine.RemoveProjectReferences(project.ID, project.Slug)
	}
	for _, projectID := range r.clearCurrentProjectIDs {
		if machine.CurrentProject == projectID {
			machine.CurrentProject = ""
		}
	}
	machine.RemoveMachineLocalTaskLocators(r.removeTaskLocators)
}

func (r *machineDomainRepair) merge(next machineDomainRepair) {
	if r == nil || next.empty() {
		return
	}
	for _, project := range next.removeProjects {
		if !slices.Contains(r.removeProjects, project) {
			r.removeProjects = append(r.removeProjects, project)
		}
	}
	for _, projectID := range next.clearCurrentProjectIDs {
		if projectID != "" && !slices.Contains(r.clearCurrentProjectIDs, projectID) {
			r.clearCurrentProjectIDs = append(r.clearCurrentProjectIDs, projectID)
		}
	}
	for _, locator := range next.removeTaskLocators {
		if locator.ProjectID != "" && locator.TaskID != "" && !slices.Contains(r.removeTaskLocators, locator) {
			r.removeTaskLocators = append(r.removeTaskLocators, locator)
		}
	}
}

type retryMachineDomainRepairMsg struct{}

func retryMachineDomainRepairCmd() tea.Cmd {
	return tea.Tick(machineDomainRepairRetryDelay, func(time.Time) tea.Msg {
		return retryMachineDomainRepairMsg{}
	})
}

func (m *AppModel) queueMachineDomainRepair(repair machineDomainRepair) tea.Cmd {
	if m == nil || repair.empty() {
		return nil
	}
	m.pendingMachineDomainRepair.merge(repair)
	if m.machineDomainRepairRetryScheduled {
		return nil
	}
	m.machineDomainRepairRetryScheduled = true
	return retryMachineDomainRepairCmd()
}

func (m *AppModel) retryMachineDomainRepair() tea.Cmd {
	if m == nil {
		return nil
	}
	m.machineDomainRepairRetryScheduled = false
	if m.pendingMachineDomainRepair.empty() {
		return nil
	}
	if m.runtime == nil {
		logging.Warn("cannot retry machine-state cleanup: runtime state is unavailable")
		return m.rescheduleMachineDomainRepair()
	}

	if m.runtime.DataDir() != "" {
		if err := m.runtime.RefreshAfterCommit(); err != nil {
			logging.Warn("cannot refresh before machine-state cleanup retry: %v", err)
			return m.rescheduleMachineDomainRepair()
		}
	}
	machine := cloneMachineState(m.runtime.Machine())
	m.pendingMachineDomainRepair.apply(machine)

	var err error
	if m.runtime.DataDir() == "" {
		m.runtime.SetMachineInMemory(machine)
	} else {
		err = m.runtime.CommitMachine(machine)
	}
	if err != nil {
		logging.Warn("machine-state cleanup retry failed: %v", err)
		return m.rescheduleMachineDomainRepair()
	}

	m.pendingMachineDomainRepair = machineDomainRepair{}
	m.syncProjectCache()
	m.messageLog.Append("Machine-state cleanup completed", MessageLogLevelSuccess)
	return nil
}

func (m *AppModel) rescheduleMachineDomainRepair() tea.Cmd {
	if m == nil || m.pendingMachineDomainRepair.empty() {
		return nil
	}
	m.machineDomainRepairRetryScheduled = true
	return retryMachineDomainRepairCmd()
}

func machineRepairWarning(action string, err error) string {
	if err == nil {
		return action
	}
	logging.Warn("%s; machine-state cleanup will retry: %v", action, err)
	return fmt.Sprintf("%s — machine-state cleanup incomplete; retrying", action)
}
