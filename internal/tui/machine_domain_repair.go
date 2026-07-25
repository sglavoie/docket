package tui

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
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
	removeProjects       []machineProjectReference
	clearCurrentProjects []machineProjectReference
	removeTaskLocators   []config.ProjectTaskLocator
}

func machineRepairForArchivedProject(projectID, slug string, locators []config.ProjectTaskLocator) machineDomainRepair {
	return machineDomainRepair{
		clearCurrentProjects: []machineProjectReference{{ID: projectID, Slug: slug}},
		removeTaskLocators:   slices.Clone(locators),
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
		len(r.clearCurrentProjects) == 0 &&
		len(r.removeTaskLocators) == 0
}

func machineProjectReferenceMatches(ref string, project machineProjectReference) bool {
	if ref == "" {
		return false
	}
	return ref == project.ID || (project.Slug != "" && ref == project.Slug)
}

func isNormalWorkProject(project *models.Project) bool {
	return project != nil && !project.Archived && !project.Sandbox && !project.Internal
}

// applyMachineDomainRepair validates every cleanup target against the current
// authoritative workspace before mutating the fresh machine snapshot. A project
// or task restored after the cleanup was queued is therefore retained. When an
// active project's task data cannot be loaded, validation fails closed and the
// whole patch remains queued for a later retry.
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
	for _, project := range repair.clearCurrentProjects {
		if machineProjectReferenceMatches(machine.CurrentProject, project) &&
			!isNormalWorkProject(m.runtime.GetProject(machine.CurrentProject)) {
			machine.CurrentProject = ""
			changed = true
		}
	}

	loaded := make(map[string]*models.Project)
	var invalidLocators []config.ProjectTaskLocator
	for _, locator := range repair.removeTaskLocators {
		live, err := m.machineTaskLocatorIsLive(locator, loaded)
		if err != nil {
			return false, err
		}
		if !live {
			invalidLocators = append(invalidLocators, locator)
		}
	}
	if machine.RemoveMachineLocalTaskLocators(invalidLocators) > 0 {
		changed = true
	}
	return changed, nil
}

func (m *AppModel) machineTaskLocatorIsLive(locator config.ProjectTaskLocator, loaded map[string]*models.Project) (bool, error) {
	if locator.ProjectID == "" || locator.TaskID == "" {
		return false, nil
	}
	project := m.runtime.GetProject(locator.ProjectID)
	if !isNormalWorkProject(project) {
		return false, nil
	}
	if loadedProject, ok := loaded[project.ID]; ok {
		return loadedProject.GetTaskByID(locator.TaskID) != nil, nil
	}
	if m.persister == nil {
		return false, fmt.Errorf("validate machine-state task cleanup for project %s: project storage is unavailable", project.ID)
	}
	if err := m.persister.Load(project); err != nil {
		return false, fmt.Errorf("validate machine-state task cleanup for project %s: %w", project.ID, err)
	}
	loaded[project.ID] = project
	return project.GetTaskByID(locator.TaskID) != nil, nil
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
	for _, project := range next.clearCurrentProjects {
		if (project.ID != "" || project.Slug != "") && !slices.Contains(r.clearCurrentProjects, project) {
			r.clearCurrentProjects = append(r.clearCurrentProjects, project)
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
	m.retainMachineDomainRepair(repair)
	if m.machineDomainRepairRetryScheduled {
		return nil
	}
	m.machineDomainRepairRetryScheduled = true
	return retryMachineDomainRepairCmd()
}

func (m *AppModel) retainMachineDomainRepair(repair machineDomainRepair) {
	if m == nil || repair.empty() {
		return
	}
	m.pendingMachineDomainRepair.merge(repair)
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
	m.messageLog.Append("Machine-state cleanup completed", MessageLogLevelSuccess)
	return nil
}

func (m *AppModel) commitMachineDomainRepair(repair machineDomainRepair) error {
	if m == nil || m.runtime == nil || repair.empty() {
		return nil
	}
	machine := cloneMachineState(m.runtime.Machine())
	changed, err := m.applyMachineDomainRepair(repair, machine)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if m.runtime.DataDir() == "" {
		m.runtime.SetMachineInMemory(machine)
	} else if err := m.runtime.CommitMachine(machine); err != nil {
		return err
	}
	m.syncProjectCache()
	return nil
}

// reconcileStartupMachineReferences clears a current-project selection that can
// no longer resolve through the authoritative catalog. Missing custom-context
// members are intentionally retained: the Context Hub reports them as stale
// references and only an explicit custom-context edit or delete may discard
// them. Recent task visits are reconciled separately by hydrateRecentVisits.
//
// If machine.json is still unavailable, the patch is retained for Init to
// schedule; if the process exits first, the same repair is derived again on the
// next startup.
func (m *AppModel) reconcileStartupMachineReferences() error {
	if m == nil || m.runtime == nil {
		return nil
	}
	machine := m.runtime.Machine()
	var repair machineDomainRepair
	if current := machine.CurrentProject; current != "" {
		project := m.runtime.GetProject(current)
		switch {
		case project == nil:
			repair.merge(machineDomainRepair{
				clearCurrentProjects: []machineProjectReference{{ID: current}},
			})
		case !isNormalWorkProject(project):
			repair.merge(machineDomainRepair{
				clearCurrentProjects: []machineProjectReference{{ID: project.ID, Slug: project.Slug}},
			})
		}
	}
	if repair.empty() {
		return nil
	}
	if err := m.commitMachineDomainRepair(repair); err != nil {
		m.retainMachineDomainRepair(repair)
		return err
	}
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
