// Package engine is the frontend-independent workspace runtime: the one
// composition of repository construction, loading, crash recovery, planning
// integration, application-service wiring, and query evaluation that every
// frontend drives. The desktop TUI and a headless caller open the same Runtime;
// neither rebuilds the durable side for itself.
//
// Nothing here reads the process environment on its own initiative. Where the
// workspace lives, which machine owns its local state and transactions, what
// time it is, and how identities are minted all arrive through Config, so a
// frontend without desktop home-directory conventions names each of them.
package engine

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/sglavoie/tt/internal/app"
)

// ErrWorkspaceNotFound reports an Open without CreateIfMissing against a data
// directory that holds no workspace.
var ErrWorkspaceNotFound = errors.New("no tt workspace at the configured data directory")

// ErrInvalidConfig marks a Config that names no usable workspace or machine.
// Every validation failure matches it with errors.Is and keeps its own message.
var ErrInvalidConfig = errors.New("engine: invalid configuration")

type configError struct{ message string }

func (e configError) Error() string        { return e.message }
func (e configError) Is(target error) bool { return target == ErrInvalidConfig }

func invalidConfig(format string, args ...any) error {
	return configError{message: fmt.Sprintf(format, args...)}
}

// ErrMigrationRequired reports an Open with RefuseMigration against a workspace
// whose schema is older than this engine, which opening would migrate.
var ErrMigrationRequired = errors.New("the workspace schema is older than this engine and opening it would migrate")

// Config is everything a Runtime depends on. Only DataDir and Machine are
// required; a nil Clock or IDs means the system clock and random UUIDs, which
// are values rather than environment and are what every frontend uses outside
// tests.
type Config struct {
	// DataDir is the workspace root holding .tt/ and projects/. A frontend with
	// no meaningful working directory passes an absolute path; the desktop passes
	// the path its flags resolved, unchanged, because the machine-local process
	// lock is keyed by that spelling and workspace relocation must find it.
	DataDir string
	// Machine names whose machine-local state and transactions this runtime
	// speaks for.
	Machine Machine
	// Clock is the application clock. Nil is the system clock.
	Clock app.Clock
	// IDs mints task and note identities. Nil is random UUIDs.
	IDs app.IDGenerator
	// CreateIfMissing bootstraps a fresh workspace at DataDir when none exists.
	// Without it, Open refuses a directory holding no workspace.
	CreateIfMissing bool
	// RefuseMigration fails Open with ErrMigrationRequired, before anything is
	// recovered or written, when the workspace schema is older than this engine,
	// and with a *config.SchemaTooNewError when it is newer. Without it, Open
	// activates pending migrations as the desktop always has.
	RefuseMigration bool
}

// Machine is the identity a runtime commits as. Exactly one of two forms is
// valid, and the zero value is neither, so a caller cannot reach the process
// environment by omission:
//
//   - an explicit machine, naming the directory machine-local state lives
//     beneath and the stable device identity stamped on every transaction;
//   - ProcessMachine, the desktop's deliberate choice to follow the running
//     process's home directory and hostname, recomputed on every journal.
type Machine struct {
	// Home is the root machine-local state resolves beneath, as
	// <Home>/.tt/runtime/<workspace-id>/machine.json.
	Home string
	// DeviceID is the stable transaction owner recovery compares against.
	DeviceID string

	followProcess bool
}

// ExplicitMachine names a machine by its local-state root and device identity.
func ExplicitMachine(home, deviceID string) Machine {
	return Machine{Home: home, DeviceID: deviceID}
}

// ProcessMachine is the desktop machine: machine-local state beneath the
// process's own home, owned by the identity derived from that home and the
// hostname. It is chosen, never defaulted to.
func ProcessMachine() Machine {
	return Machine{followProcess: true}
}

// deviceIDPattern bounds a device identity to a short token that is safe in a
// manifest, a file name, and a log line.
var deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func (m Machine) validate() error {
	if m.followProcess {
		if m.Home != "" || m.DeviceID != "" {
			return invalidConfig("engine: the process machine names no home or device identity")
		}
		return nil
	}
	if m.Home == "" || m.DeviceID == "" {
		return invalidConfig("engine: a machine home and a device identity are required")
	}
	if !filepath.IsAbs(m.Home) {
		return invalidConfig("engine: machine home %q is not absolute", m.Home)
	}
	if !deviceIDPattern.MatchString(m.DeviceID) {
		return invalidConfig("engine: device identity %q must be 1-64 letters, digits, '.', '_' or '-'", m.DeviceID)
	}
	return nil
}

func (c Config) validate() error {
	if c.DataDir == "" {
		return invalidConfig("engine: a data directory is required")
	}
	return c.Machine.validate()
}
