package tui

// syncTestRuntime reconciles the shell project cache after a test mutates the
// in-memory runtime domains.
func syncTestRuntime(app *AppModel) {
	app.syncProjectCache()
}
