package tui

import (
	"fmt"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/storage"
)

// newTUIAppService builds the shared runtime's application service over one
// workspace, without the default planning builder, for tests that compose the
// shell's durable side piece by piece. Production composes it through
// engine.Open or engine.Attach.
func newTUIAppService(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger) *app.Service {
	return engine.NewService(engine.NewRepository(dataDir, deps, ledger, ""), nil, nil)
}

// newTUIAppRepository is the shared runtime's repository on the process
// machine, for tests that drive it under their own clock or a storage double.
func newTUIAppRepository(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger) *engine.Repository {
	return engine.NewRepository(dataDir, deps, ledger, "")
}

// newAppModelFromSnapshot builds the shell the way a launch does — recover, compose the
// shared runtime, construct — over a snapshot and manager the test built.
func newAppModelFromSnapshot(runtime *config.RuntimeState, manager *config.Manager, opts ...AppOption) (*AppModel, error) {
	recovered, err := engine.Recover(runtime, manager)
	if err != nil {
		return nil, fmt.Errorf("recover workspace before TUI startup: %w", err)
	}
	return NewAppModel(engine.Attach(recovered, manager, nil, nil), opts...)
}
