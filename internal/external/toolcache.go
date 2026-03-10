package external

import (
	"os/exec"
	"sync"
)

var (
	fzfOnce    sync.Once
	fzfPath    string
	fzfErr     error
	glowOnce   sync.Once
	glowPath   string
	glowErr    error
	codeOnce   sync.Once
	codePath   string
	codeErr    error
	claudeOnce sync.Once
	claudePath string
	claudeErr  error
)

func cachedLookPath(name string, once *sync.Once, path *string, pathErr *error) (string, error) {
	once.Do(func() { *path, *pathErr = exec.LookPath(name) })
	return *path, *pathErr
}

// CachedFzfPath returns the cached result of exec.LookPath("fzf").
func CachedFzfPath() (string, error) {
	return cachedLookPath("fzf", &fzfOnce, &fzfPath, &fzfErr)
}

// CachedGlowPath returns the cached result of exec.LookPath("glow").
func CachedGlowPath() (string, error) {
	return cachedLookPath("glow", &glowOnce, &glowPath, &glowErr)
}

// CachedCodePath returns the cached result of exec.LookPath("code").
func CachedCodePath() (string, error) {
	return cachedLookPath("code", &codeOnce, &codePath, &codeErr)
}

// CachedClaudePath returns the cached result of exec.LookPath("claude").
func CachedClaudePath() (string, error) {
	return cachedLookPath("claude", &claudeOnce, &claudePath, &claudeErr)
}
