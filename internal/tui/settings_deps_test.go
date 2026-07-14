package tui

import "github.com/sglavoie/tt/internal/config"

// newSettingsDeps builds the settings editor's dependency bundle for tests from a
// Config and an optional manager, decomposing the Config into the domain-native
// runtime the editor now reads and commits through. The runtime is rooted at the
// manager's workspace directory when a manager is present (so preset file I/O and
// domain commits land in the same temp workspace), otherwise at the Config's own
// data dir. Passing a nil manager exercises the isolated render path.
func newSettingsDeps(cfg *config.RuntimeState, mgr *config.Manager) *shellDeps {
	dataDir := cfg.DataDir()
	if mgr != nil {
		dataDir = config.WorkspaceRootFromConfigPath(mgr.GetConfigPath())
	}
	return &shellDeps{Runtime: testRuntimeAt(dataDir, cfg), Manager: mgr}
}
