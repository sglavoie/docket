package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
)

// shellDeps is the domain-native dependency bundle held by the long-lived TUI
// shell. It carries the three composition-root capabilities shared with views:
//
//   - Runtime: the startup-loaded snapshot of the five configuration domains
//     (catalog, planning, settings, automation, machine). Views read domain data
//     through its narrow accessors with no render-time disk load, and a
//     config-bearing mutation commits only the changed domain(s) through its
//     revision-checked CommitCatalog/CommitPlanning/CommitSettings/
//     CommitAutomation/CommitMachine seams — never by rebuilding or saving
//     a monolithic configuration aggregate.
//   - Service: the transactional application service shared with the CLI, so a
//     migrated view routes a task mutation through one atomic, ledger-undoable
//     ChangeSet instead of a direct model save.
//   - Manager: the config manager retained for the transaction-folded config
//     artifact bridge (configDomainArtifacts) and as the backing for the domain
//     committers.
type shellDeps struct {
	Runtime *config.RuntimeState
	Service *app.Service
	Manager *config.Manager
}

// newShellDeps assembles the shell dependency bundle from the runtime snapshot,
// application service, and config manager built at composition time.
func newShellDeps(runtime *config.RuntimeState, service *app.Service, manager *config.Manager) *shellDeps {
	return &shellDeps{Runtime: runtime, Service: service, Manager: manager}
}
