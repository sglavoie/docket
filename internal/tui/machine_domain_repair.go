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

// machineDomainRepair removes deleted projects from retained custom contexts.
type machineDomainRepair struct {
	removeProjects []machineProjectReference
}

func machineRepairForArchivedProject(string, string) machineDomainRepair {
	return machineDomainRepair{}
}

func (r machineDomainRepair) empty() bool { return len(r.removeProjects) == 0 }

func (m *AppModel) applyMachineDomainRepair(repair machineDomainRepair, machine *config.MachineState) (bool, error) {
	if machine == nil || m == nil || m.runtime == nil {
		return false, nil
	}
	changed := false
	for _, project := range repair.removeProjects {
		for _, ref := range []string{project.ID, project.Slug} {
			if ref == "" || m.runtime.GetProject(ref) != nil {
				continue
			}
			changed = machine.RemoveProjectReferences(ref, "") || changed
		}
	}
	return changed, nil
}

func (r *machineDomainRepair) merge(next machineDomainRepair) {
	if r == nil {
		return
	}
	for _, project := range next.removeProjects {
		if !slices.Contains(r.removeProjects, project) {
			r.removeProjects = append(r.removeProjects, project)
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
	m.retainMachineDomainRepair(repair)
	if m.machineDomainRepairRetryScheduled {
		return nil
	}
	m.machineDomainRepairRetryScheduled = true
	return retryMachineDomainRepairCmd()
}

func (m *AppModel) retainMachineDomainRepair(repair machineDomainRepair) {
	if m != nil && !repair.empty() {
		m.pendingMachineDomainRepair.merge(repair)
	}
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
	if err := m.commitMachineDomainRepair(m.pendingMachineDomainRepair); err != nil {
		logging.Warn("machine-state cleanup retry failed: %v", err)
		return m.rescheduleMachineDomainRepair()
	}
	m.pendingMachineDomainRepair = machineDomainRepair{}
	m.syncProjectCache()
	m.setSuccessStatus("Machine-state cleanup completed")
	return nil
}

func (m *AppModel) commitMachineDomainRepair(repair machineDomainRepair) error {
	if m == nil || m.runtime == nil || repair.empty() {
		return nil
	}
	machine := cloneMachineState(m.runtime.Machine())
	changed, err := m.applyMachineDomainRepair(repair, machine)
	if err != nil || !changed {
		return err
	}
	if m.runtime.DataDir() == "" {
		m.runtime.SetMachineInMemory(machine)
	} else if err := m.runtime.CommitMachine(machine); err != nil {
		return err
	}
	m.syncProjectCache()
	return nil
}

func (m *AppModel) reconcileStartupMachineReferences() error { return nil }

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
