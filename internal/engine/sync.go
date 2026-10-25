package engine

import (
	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/transaction"
)

// LiveSchemas are the artifact schemas this engine writes, as a sync manifest
// and a device advertisement name them. From catalog schema 23 an enrolled
// workspace's catalog is at the current schema too; workspaceSchemas still
// reads the loaded catalog's own schema rather than assuming it.
func LiveSchemas() map[string]int {
	return map[string]int{
		synclog.SchemaDomainEnvelope: config.CurrentDomainSchemaVersion,
		synclog.SchemaCatalog:        config.CurrentConfigVersion,
		synclog.SchemaTaskMarkdown:   storage.CurrentTaskSchemaVersion,
	}
}

// workspaceSchemas are the schemas a workspace's history records at: the live
// schemas, with the catalog at the schema the loaded catalog declares, so an
// enrolled workspace publishes at the enrolled schema and a device that reads
// only the older one holds its transactions rather than misreading them.
func workspaceSchemas(state *config.RuntimeState) map[string]int {
	schemas := LiveSchemas()
	if catalog := state.Catalog(); catalog != nil && catalog.SchemaVersion > 0 {
		schemas[synclog.SchemaCatalog] = catalog.SchemaVersion
	}
	return schemas
}

// syncDeviceID is the identity a working copy's sync history records as: the
// configured device, else the one the working copy already records as, else
// this machine's identity when it has recorded nothing. Only that last case
// reads the hostname, so it names a device once and never renames one.
//
// The transaction owner is deliberately not this value. Recovery asks which
// machine is running now and recomputes it per call; sync asks whose history
// this is, which must outlive a renamed host.
func syncDeviceID(dataDir, owner string) string {
	if owner != "" {
		return owner
	}
	if recorded := synclog.RecordedDeviceID(dataDir); recorded != "" {
		return recorded
	}
	return transaction.DefaultMachineID()
}

// syncHistory is the device's sync history over a rooted workspace, recording
// as syncDeviceID names it. An unrooted runtime, which commits nothing durable,
// has none.
func syncHistory(dataDir, owner string, clock app.Clock, schemas map[string]int) *synclog.History {
	if dataDir == "" {
		return nil
	}
	history, err := synclog.New(synclog.Config{
		DataDir:  dataDir,
		DeviceID: syncDeviceID(dataDir, owner),
		Schemas:  schemas,
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
