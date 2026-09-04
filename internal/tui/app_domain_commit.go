package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// This file holds the shell-level helpers that commit one owning configuration
// domain through RuntimeState. Each helper mutates a defensive copy and commits
// it under the runtime's optimistic-concurrency check.

// commitPlanningToRuntime commits a planning mutation through a runtime snapshot
// without touching the legacy config.Config facade. It is the view-level
// counterpart to AppModel.commitPlanningMutation for the focus views, which hold
// their own runtime reference and drive planning-owned reads/writes off it. A nil
// or non-writable runtime is a silent no-op so tests that never root a writable
// workspace do not attempt (or fail) a disk commit; the caller's in-memory shell
// state remains authoritative.
// commitCatalogProjectMutation is the ledger-aware catalog boundary for
// user-facing project pin and restore changes. The catalog rewrite and its undo
// record are one application transaction.
func (m *AppModel) commitCatalogProjectMutation(
	action string,
	context string,
	projectIDs []string,
	mutate func(*config.WorkspaceCatalog) error,
) (string, error) {
	if m.domain.runtime == nil {
		return "", nil
	}
	catalog := coordinator.CloneWorkspaceCatalog(m.domain.runtime.Catalog())
	if err := mutate(catalog); err != nil {
		return "", err
	}
	if !m.domain.writable() {
		m.domain.runtime.SetCatalogInMemory(catalog)
		m.syncProjectCache()
		return "", nil
	}
	artifacts, err := coordinator.CatalogDomainArtifact(m.domain.runtime, catalog)
	if err != nil {
		return "", err
	}
	transactionID, err := m.domain.service.CommitProjectArtifactsResult(storage.ArtifactCommitRequest{
		Artifacts:   artifacts,
		ProjectIDs:  projectIDs,
		UndoAction:  action,
		UndoContext: context,
	})
	if err != nil {
		return "", err
	}
	if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
		return "", err
	}
	m.syncProjectCache()
	return transactionID, nil
}

// commitCatalogToRuntime commits a catalog mutation through a runtime snapshot
// without touching the legacy config.Config facade. It is the view-level
// counterpart to AppModel.commitCatalogMutation for views (project creator/
// editor/cycle manager) that hold their own runtime reference and drive
// catalog-owned writes off it. A nil or non-writable runtime applies the
// mutation in memory so runtime.Catalog() observes it; the caller's in-memory
// shell state remains authoritative.
func cloneRuntimeDomains(runtime *config.RuntimeState) *config.Domains {
	if runtime == nil {
		return nil
	}
	return &config.Domains{
		Catalog:  coordinator.CloneWorkspaceCatalog(runtime.Catalog()),
		Planning: coordinator.ClonePlanningState(runtime.Planning()),
		Machine:  cloneMachineState(runtime.Machine()),
	}
}

// runtimeDomainsCommitResult distinguishes the authoritative synced workspace
// transaction from the machine-local follow-up, which cannot share the same
// transaction root. A caller must publish the workspace result when
// WorkspaceCommitted is true even if MachineErr is non-nil, then queue the
// idempotent machine repair for retry.
type runtimeDomainsCommitResult struct {
	MachineErr    error
	TransactionID string
}

func runtimeDomainsMachineFailure(result runtimeDomainsCommitResult, machineErr error) (runtimeDomainsCommitResult, error) {
	result.MachineErr = machineErr
	return result, nil
}

func (m *AppModel) commitRuntimeDomainsWithUndo(
	domains *config.Domains,
	action string,
	context string,
	projectIDs []string,
) (runtimeDomainsCommitResult, error) {
	if m == nil || m.domain.runtime == nil || domains == nil {
		return runtimeDomainsCommitResult{}, nil
	}
	if !m.domain.writable() {
		m.domain.runtime.SetDomainsInMemory(domains)
		m.syncProjectCache()
		return runtimeDomainsCommitResult{}, nil
	}
	arts, err := syncedDomainArtifacts(
		m.domain.runtime,
		domains.Catalog,
		domains.Planning,
	)
	if err != nil {
		return runtimeDomainsCommitResult{}, err
	}
	result := runtimeDomainsCommitResult{}
	if action == "" {
		if err := m.domain.service.CommitConfig(arts...); err != nil {
			return runtimeDomainsCommitResult{}, err
		}
	} else {
		transactionID, err := m.domain.service.CommitProjectArtifactsResult(storage.ArtifactCommitRequest{
			Artifacts:   arts,
			ProjectIDs:  projectIDs,
			UndoAction:  action,
			UndoContext: context,
		})
		if err != nil {
			return runtimeDomainsCommitResult{}, err
		}
		result.TransactionID = transactionID
	}
	if err := m.domain.runtime.CommitMachine(domains.Machine); err != nil {
		// CommitMachine may have failed before writing or only while refreshing
		// after a successful write. Either way, advance to the authoritative
		// synced transaction when possible; the queued repair is idempotent.
		_ = m.domain.runtime.RefreshAfterCommit()
		m.syncProjectCache()
		return runtimeDomainsMachineFailure(result, err)
	}
	m.syncProjectCache()
	return result, nil
}

// cloneMachineState returns a deep copy of the machine snapshot. The lens
// definitions carry nested state, tag, and project-tag slices, so a shallow
// copy would let an edit to the clone rewrite the runtime snapshot before the
// commit that is supposed to publish it.
func cloneMachineState(m *config.MachineState) *config.MachineState {
	cp := *m
	cp.NamedLenses = m.NamedLenses.Clone()
	return &cp
}
