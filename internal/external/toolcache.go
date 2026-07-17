package external

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/sglavoie/tt/internal/aiharness"
)

const cacheTTL = 5 * time.Minute

// cachedPath holds a TTL-aware cached result of exec.LookPath.
type cachedPath struct {
	mu        sync.Mutex
	path      string
	err       error
	checkedAt time.Time
}

func (c *cachedPath) lookup(name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.checkedAt.IsZero() || time.Since(c.checkedAt) > cacheTTL {
		c.path, c.err = exec.LookPath(name)
		c.checkedAt = time.Now()
	}
	return c.path, c.err
}

var (
	fzfCache     cachedPath
	glowCache    cachedPath
	codeCache    cachedPath
	claudeCache  cachedPath
	codexCache   cachedPath
	copilotCache cachedPath
)

// CachedFzfPath returns the TTL-cached result of exec.LookPath("fzf").
func CachedFzfPath() (string, error) {
	return fzfCache.lookup("fzf")
}

// CachedGlowPath returns the TTL-cached result of exec.LookPath("glow").
func CachedGlowPath() (string, error) {
	return glowCache.lookup("glow")
}

// CachedCodePath returns the TTL-cached result of exec.LookPath("code").
func CachedCodePath() (string, error) {
	return codeCache.lookup("code")
}

// CachedHarnessPath returns the TTL-cached executable path for a supported AI
// harness.
func CachedHarnessPath(harness aiharness.Name) (string, error) {
	switch harness {
	case aiharness.Claude:
		return claudeCache.lookup(harness.Executable())
	case aiharness.Codex:
		return codexCache.lookup(harness.Executable())
	case aiharness.Copilot:
		return copilotCache.lookup(harness.Executable())
	default:
		return "", fmt.Errorf("resolve AI harness executable: %w", aiharness.ErrUnknownHarness)
	}
}
