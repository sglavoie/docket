package cmd

import (
	"fmt"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

// Version reporting is the acceptance surface for `go install`, which passes no
// ldflags. These tests pin both halves of that contract: ldflags still win when
// the justfile sets them, and the module build info fills the gap when it does
// not.

func stubBuildInfo(t *testing.T, info *debug.BuildInfo, ok bool) {
	t.Helper()

	original := readBuildInfo
	t.Cleanup(func() { readBuildInfo = original })
	readBuildInfo = func() (*debug.BuildInfo, bool) { return info, ok }
}

func stubLdflags(t *testing.T, version, commit, buildTime string) {
	t.Helper()

	originalVersion, originalCommit, originalBuildTime := Version, GitCommit, BuildTime
	t.Cleanup(func() {
		Version, GitCommit, BuildTime = originalVersion, originalCommit, originalBuildTime
	})
	Version, GitCommit, BuildTime = version, commit, buildTime
}

func TestResolveBuildInfoPrefersLdflags(t *testing.T) {
	stubLdflags(t, "1.2.3", "abc1234", "2026-08-28T10:00:00Z")
	stubBuildInfo(t, &debug.BuildInfo{
		Main:     debug.Module{Version: "v9.9.9"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "ignored"}},
	}, true)

	version, commit, buildTime := resolveBuildInfo()
	if version != "1.2.3" || commit != "abc1234" || buildTime != "2026-08-28T10:00:00Z" {
		t.Fatalf("resolveBuildInfo() = %q, %q, %q; want the ldflags values untouched", version, commit, buildTime)
	}
}

func TestResolveBuildInfoUsesModuleVersionUnderGoInstall(t *testing.T) {
	stubLdflags(t, devVersion, "", "")
	// `go install pkg@version` records the module version and no VCS stamps.
	stubBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}}, true)

	version, commit, buildTime := resolveBuildInfo()
	if version != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", version)
	}
	if commit != "" || buildTime != "" {
		t.Errorf("commit/buildTime = %q/%q, want both empty", commit, buildTime)
	}
}

func TestResolveBuildInfoUsesVCSStampsFromSourceBuild(t *testing.T) {
	stubLdflags(t, devVersion, "", "")
	// A plain `go build` in a checkout stamps VCS settings but leaves the module
	// version at the devel placeholder, which is not worth reporting.
	stubBuildInfo(t, &debug.BuildInfo{
		Main: debug.Module{Version: moduleDevVersion},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeefcafe"},
			{Key: "vcs.time", Value: "2026-08-28T09:30:00Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}, true)

	version, commit, buildTime := resolveBuildInfo()
	if version != devVersion {
		t.Errorf("version = %q, want %q", version, devVersion)
	}
	if commit != "deadbee" || buildTime != "2026-08-28T09:30:00Z" {
		t.Errorf("commit/buildTime = %q/%q, want the VCS stamps, the revision short like the justfile's", commit, buildTime)
	}
}

func TestResolveBuildInfoFallsBackWhenBuildInfoIsUnavailable(t *testing.T) {
	stubLdflags(t, devVersion, "", "")
	stubBuildInfo(t, nil, false)

	version, commit, buildTime := resolveBuildInfo()
	if version != devVersion || commit != "" || buildTime != "" {
		t.Fatalf("resolveBuildInfo() = %q, %q, %q; want the compiled-in defaults", version, commit, buildTime)
	}
}

func TestPrintVersionRendersResolvedValues(t *testing.T) {
	stubLdflags(t, devVersion, "", "")
	stubBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}}, true)

	bare, _, _ := captureDoctorOutput(t, func() error {
		printVersion()
		return nil
	})
	if strings.TrimSpace(bare) != "docket version v1.0.0" {
		t.Errorf("printVersion() = %q, want the bare module version", strings.TrimSpace(bare))
	}

	stubBuildInfo(t, &debug.BuildInfo{
		Main: debug.Module{Version: "v1.0.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeefcafe"},
			{Key: "vcs.time", Value: "2026-08-28T09:30:00Z"},
		},
	}, true)

	stamped, _, _ := captureDoctorOutput(t, func() error {
		printVersion()
		return nil
	})
	want := "docket version v1.0.0 (commit: deadbee, built: 2026-08-28T09:30:00Z)"
	if strings.TrimSpace(stamped) != want {
		t.Errorf("printVersion() = %q, want %q", strings.TrimSpace(stamped), want)
	}
}

// TestRootLongDescriptionNamesTheCurrentSchema keeps the first text a new user
// reads from drifting behind the migration chain again.
func TestRootLongDescriptionNamesTheCurrentSchema(t *testing.T) {
	long := rootCmd.Long
	for _, current := range []string{
		fmt.Sprintf("schema-%d catalogs", config.CurrentConfigVersion),
		fmt.Sprintf("schema-%d task Markdown", storage.CurrentTaskSchemaVersion),
	} {
		if !strings.Contains(long, current) {
			t.Errorf("root Long description does not name %q:\n%s", current, long)
		}
	}
	for _, retired := range []string{"schema-6", "schema-5", "schema 6", "schema 5"} {
		if strings.Contains(long, retired) {
			t.Errorf("root Long description still names retired schema vocabulary %q", retired)
		}
	}
}
