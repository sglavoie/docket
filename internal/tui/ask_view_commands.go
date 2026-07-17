package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AskViewModel) applyPlan(prepared *app.PreparedActionPlan, destructive bool) tea.Cmd {
	operationID := nextAskOperationID()
	service := m.service
	runtime := m.runtime
	var pendingDomains *config.Domains
	var runtimeErr error
	if runtime != nil {
		pendingDomains, runtimeErr = runtime.CloneDomains()
	}
	m.applyID = operationID
	m.state = askStateApplying
	applyCmd := func() tea.Msg {
		if service == nil {
			return AskActionPlanAppliedMsg{OperationID: operationID, Err: fmt.Errorf("application service is unavailable")}
		}
		result, err := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{
			ConfirmSafe:        true,
			ConfirmDestructive: destructive,
			DeletedTaskArtifacts: func(deletedTaskIDs []string) ([]storage.ArtifactChange, error) {
				if runtime == nil {
					return nil, fmt.Errorf("runtime state is unavailable")
				}
				if runtimeErr != nil {
					return nil, runtimeErr
				}
				if pendingDomains == nil {
					return nil, fmt.Errorf("runtime domains are unavailable")
				}
				if pendingDomains.Planning.RemoveDurableTaskRefs(askActionPlanTaskIDSet(deletedTaskIDs)) == 0 || runtime.DataDir() == "" {
					return nil, nil
				}
				artifact, artifactErr := runtime.CheckedPlanningArtifact(pendingDomains.Planning)
				if artifactErr != nil {
					return nil, artifactErr
				}
				return []storage.ArtifactChange{artifact.StorageChange()}, nil
			},
		})
		if err != nil {
			return AskActionPlanAppliedMsg{OperationID: operationID, Err: err}
		}
		return AskActionPlanAppliedMsg{OperationID: operationID, Result: result, PendingDomains: pendingDomains}
	}
	return tea.Batch(applyCmd, m.spinner.Tick())
}

func repairAskActionPlanTaskRefs(runtime *config.RuntimeState, deletedTaskIDs []string, pendingDomains *config.Domains) error {
	if len(deletedTaskIDs) == 0 {
		return nil
	}
	if runtime == nil {
		return fmt.Errorf("runtime state is unavailable")
	}
	deleted := askActionPlanTaskIDSet(deletedTaskIDs)
	if runtime.DataDir() == "" {
		if pendingDomains == nil {
			var err error
			pendingDomains, err = runtime.CloneDomains()
			if err != nil {
				return err
			}
		}
		pendingDomains.Machine.RemoveMachineLocalTaskRefs(deleted)
		runtime.SetDomainsInMemory(pendingDomains)
		return nil
	}

	var warnings []error
	if err := runtime.RefreshAfterCommit(); err != nil {
		warnings = append(warnings, fmt.Errorf("refresh planning state: %w", err))
	}
	domains, err := runtime.CloneDomains()
	if err != nil {
		warnings = append(warnings, err)
		return errors.Join(warnings...)
	}
	if domains.Machine.RemoveMachineLocalTaskRefs(deleted) > 0 {
		if err := runtime.CommitMachine(domains.Machine); err != nil {
			warnings = append(warnings, fmt.Errorf("update recent visits: %w", err))
		}
	}
	return errors.Join(warnings...)
}

func askActionPlanTaskIDSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
