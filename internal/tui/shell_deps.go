package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

// shellDeps is the domain-native dependency bundle held by the long-lived TUI
// shell. It carries the four composition-root capabilities shared with views:
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
//   - Ledger: the shared workspace history handle used by the mutation service
//     and already-open history-aware views. Isolated tests may leave it nil to
//     model unavailable history.
type shellDeps struct {
	Runtime *config.RuntimeState
	Service *app.Service
	Manager *config.Manager
	Ledger  *storage.WorkspaceLedger
}

// newShellDeps assembles the shell dependency bundle from the runtime snapshot,
// application service, config manager, and shared ledger built at composition
// time.
func newShellDeps(runtime *config.RuntimeState, service *app.Service, manager *config.Manager, ledger *storage.WorkspaceLedger) *shellDeps {
	return &shellDeps{Runtime: runtime, Service: service, Manager: manager, Ledger: ledger}
}

// historyLedger returns the shared history handle when this composition makes
// one available. A nil dependency bundle is valid in isolated view tests.
func (d *shellDeps) historyLedger() *storage.WorkspaceLedger {
	if d == nil {
		return nil
	}
	return d.Ledger
}
