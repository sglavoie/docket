// Package ttbridge is the gomobile binding surface: exactly the two functions
// docs/mobile/bridge-contract.md names, over one process-wide bridge. Every
// other export would cross the boundary as a generated type, so there are
// none; the protocol itself lives in internal/bridge.
package ttbridge

import (
	// An iOS process has no zoneinfo the Go runtime can read, so the time
	// zone workspace.open names is resolved from the embedded database.
	_ "time/tzdata"

	"github.com/sglavoie/docket/internal/bridge"
)

var shared = bridge.New(bridge.Options{})

// BridgeVersion is the highest bridge_version this engine speaks.
func BridgeVersion() int {
	return bridge.Version
}

// Call answers one JSON request with one JSON response. It never panics and
// never returns an empty string.
func Call(request string) string {
	return shared.Call(request)
}
