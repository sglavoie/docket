package engine

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/transaction"
)

// LiveSchemas are the artifact schemas this engine writes, as a sync manifest
// and a device advertisement name them.
func LiveSchemas() map[string]int {
	return map[string]int{
		synclog.SchemaDomainEnvelope: config.CurrentDomainSchemaVersion,
		synclog.SchemaCatalog:        config.CurrentConfigVersion,
		synclog.SchemaTaskMarkdown:   storage.CurrentTaskSchemaVersion,
	}
}

// syncHistory is the device's sync history over a rooted workspace, recording
// as the transaction owner — the configured device, or this machine's identity
// when the runtime follows the process. An unrooted runtime, which commits
// nothing durable, has none.
func syncHistory(dataDir, owner string, clock app.Clock) *synclog.History {
	if dataDir == "" {
		return nil
	}
	deviceID := owner
	if deviceID == "" {
		deviceID = transaction.DefaultMachineID()
	}
	history, err := synclog.New(synclog.Config{
		DataDir:  dataDir,
		DeviceID: deviceID,
		Schemas:  LiveSchemas(),
		Now:      clock.Now,
	})
	if err != nil {
		// Machine validation admits only identities synclog accepts, so this is
		// unreachable short of a corrupted hostname hash; commits still succeed.
		logging.Warn("sync history disabled: %v", err)
		return nil
	}
	return history
}

// SyncCommits is the commit boundary the sync exchange publishes from and
// imports through: the same service every mutation commits with, carrying the
// device's sync history and merger. A runtime with no history (an unrooted
// workspace) returns a service whose SyncHistory is nil.
func (r *Runtime) SyncCommits() *storage.CommitService { return r.repo.svc }
