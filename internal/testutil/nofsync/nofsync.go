// Package nofsync switches off fsync for the test binary that calls it.
package nofsync

import (
	"os"

	"github.com/sglavoie/tt/internal/util"
)

// Enable sets util.SkipFsyncEnv for this process and every child it starts, so
// a TestMain calling it first stops paying a full disk flush for each throwaway
// file its tests write. Tests that assert durability hooks stub them directly
// and are unaffected.
func Enable() {
	if err := os.Setenv(util.SkipFsyncEnv, "1"); err != nil {
		panic(err)
	}
}
