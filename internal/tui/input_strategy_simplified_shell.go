package tui

import "github.com/sglavoie/tt/internal/tui/operation"

// SimplifiedInputStrategy re-exports the accepted daily-loop grammar from the
// operation layer that now declares it. The shell keeps the name because it is
// one of the five identifiers `internal/tui`'s two external importers compile
// against, and a package's exported surface is a contract independent of which
// file inside the subtree happens to hold the declaration.
func SimplifiedInputStrategy() operation.InputStrategy {
	return operation.SimplifiedInputStrategy()
}
