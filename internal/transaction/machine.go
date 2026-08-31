package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// Machine identity exists because a workspace can live inside a synced
// directory, which means two machines can see the same transactions directory
// while only one of them is actually driving a given transaction. Recovery
// cannot tell "a transaction my machine was interrupted mid-apply" from "a
// transaction the other machine is applying right now" by looking at the
// manifest state alone: both read as prepared or committing. The originating
// machine plus a heartbeat is what separates them.
//
// Identity is the same thing it is everywhere else in tt: the machine home
// directory, paired with the hostname so two machines that happen to use the
// same home path are still distinct. It is hashed rather than stored verbatim
// so a manifest inside a synced tree does not carry a user's hostname and
// absolute home path around.
//
// This package deliberately does not import internal/config, which imports it,
// so the home directory arrives as a plain string: the empty string means the
// running process's own home.

// machineIDLength is how many hex characters of the identity hash are kept. It
// is long enough that a collision between two machines sharing one workspace is
// not a practical concern and short enough to read in doctor output.
const machineIDLength = 16

// DefaultMachineID returns the identity of the machine this process is running
// on, derived from its hostname and home directory.
//
// It is recomputed on every call rather than cached. A process whose home
// directory changes is a process pretending to be a different machine, which is
// exactly what the cross-machine gate and the scenario harness do, and a cached
// identity would quietly make both machines the same one.
func DefaultMachineID() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return machineIDFor(home)
}

// MachineIDForHome returns the stable identity of the machine whose home
// directory is home. An empty home resolves the running process's home, which is
// what makes "" mean "this machine" everywhere an identity is threaded through.
//
// The value is deterministic across runs on one machine and differs between two
// machines that sync one workspace, which is the only property recovery needs.
func MachineIDForHome(home string) string {
	if home == "" {
		return DefaultMachineID()
	}
	return machineIDFor(home)
}

func machineIDFor(home string) string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	sum := sha256.Sum256([]byte(hostname + "\x00" + filepath.Clean(home)))
	return hex.EncodeToString(sum[:])[:machineIDLength]
}

// resolveMachineID maps the injectable empty string onto this machine's
// identity, so every constructor can accept "" and mean the running process.
func resolveMachineID(machineID string) string {
	if machineID == "" {
		return DefaultMachineID()
	}
	return machineID
}
