package aiharness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigDirSource identifies which precedence tier supplied an effective
// harness configuration directory.
type ConfigDirSource string

const (
	ConfigDirMachineOverride ConfigDirSource = "machine-override"
	ConfigDirEnvironment     ConfigDirSource = "environment"
	ConfigDirNativeDefault   ConfigDirSource = "native-default"

	// ConfigDirOverride is a generic alias used when the explicit path came
	// from a one-request CLI override rather than remembered machine state.
	ConfigDirOverride = ConfigDirMachineOverride
)

// ResolvedConfigDir is the effective display/process configuration for one
// harness. Only an Override path should be written into the child environment;
// other sources preserve the provider's inherited native behavior.
type ResolvedConfigDir struct {
	Path   string
	Source ConfigDirSource
}

// ResolveConfigDir applies tt's directory precedence: an explicit machine or
// request override, the provider's inherited environment variable, then its
// native default. Explicit overrides are normalized and checked at use time;
// inherited and native values are intentionally left to the provider.
func ResolveConfigDir(name Name, override string) (ResolvedConfigDir, error) {
	info, err := Info(name)
	if err != nil {
		return ResolvedConfigDir{}, err
	}
	if override != "" {
		path, normalizeErr := NormalizeExplicitConfigDir(override)
		if normalizeErr != nil {
			return ResolvedConfigDir{}, normalizeErr
		}
		return ResolvedConfigDir{Path: path, Source: ConfigDirMachineOverride}, nil
	}
	if inherited := os.Getenv(info.ConfigEnvVar); inherited != "" {
		return ResolvedConfigDir{Path: inherited, Source: ConfigDirEnvironment}, nil
	}
	return ResolvedConfigDir{Path: info.NativeConfigDir, Source: ConfigDirNativeDefault}, nil
}

// NormalizeExplicitConfigDir expands only a leading home shorthand, converts
// the result to an absolute cleaned path, and verifies that it currently names
// a directory. It deliberately does not perform shell or environment-variable
// expansion.
func NormalizeExplicitConfigDir(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("AI harness config directory cannot be empty")
	}
	expanded := path
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand AI harness config directory: %w", err)
		}
		if path == "~" {
			expanded = home
		} else {
			expanded = filepath.Join(home, path[2:])
		}
	}
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve AI harness config directory %q: %w", path, err)
	}
	absPath = filepath.Clean(absPath)
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("AI harness config directory does not exist: %q", absPath)
		}
		// os.Stat returns a PathError whose Error method interpolates Path with
		// %s. Wrap its cause instead so the only rendered copy of this
		// user-controlled path is the quoted one above, while errors.Is retains
		// the underlying filesystem category.
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
		}
		return "", fmt.Errorf("access AI harness config directory %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("AI harness config path is not a directory: %q", absPath)
	}
	return absPath, nil
}
