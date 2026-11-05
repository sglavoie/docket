//go:build darwin

package handoff

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sglavoie/docket/internal/logging"
)

// commandTimeout bounds each ps or open call, so a wedged LaunchServices cannot
// hold a launch that would otherwise have refused at once.
const commandTimeout = 5 * time.Second

// System is the macOS Platform: ps for the parent walk, open -a for activation,
// and SIGUSR1 for the nudge.
func System() Platform { return darwinPlatform{} }

type darwinPlatform struct{}

func (darwinPlatform) Parent(pid int) (int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	// comm is the executable's full path on macOS, spaces included.
	out, err := exec.CommandContext(ctx, "/bin/ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "", err
	}
	return ParsePS(string(out))
}

func (darwinPlatform) Activate(app string) error {
	logging.Warn("handoff: activating %s with /usr/bin/open -a", app)
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	// open -a on a running application activates it and sends it the reopen
	// event, which is what makes Docket.app show its window.
	if out, err := exec.CommandContext(ctx, "/usr/bin/open", "-a", app).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (darwinPlatform) Nudge(pid int) error {
	logging.Warn("handoff: sending SIGUSR1 to %d", pid)
	return syscall.Kill(pid, syscall.SIGUSR1)
}

// ListenForNudges starts receiving the nudge a later launch sends this
// terminal session. It must run before the session lock advertises
// util.SessionTerminal: SIGUSR1's default action ends the process. stop ends
// the subscription.
func ListenForNudges() (nudges <-chan os.Signal, stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	return ch, func() { signal.Stop(ch) }
}
