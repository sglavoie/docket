package nofsync

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/util"
)

func TestEnableSetsTheSkipSwitch(t *testing.T) {
	t.Setenv(util.SkipFsyncEnv, "")
	Enable()
	if got := os.Getenv(util.SkipFsyncEnv); got != "1" {
		t.Fatalf("%s = %q, want \"1\"", util.SkipFsyncEnv, got)
	}
}
