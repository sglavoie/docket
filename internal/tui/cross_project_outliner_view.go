package tui

// scopeChainSeparator joins nested project-scope names inside the breadcrumb
// anchor (e.g. "[Alpha › Beta]").
const scopeChainSeparator = " › "

// scopeChromeSeparator joins the qualifiers appended to a scope or project name
// (e.g. "[Client Migration · 12 tasks]").
const scopeChromeSeparator = " · "

// activeProjectsScopeLabel names the unscoped global Outliner population in
// every piece of scope chrome.
const activeProjectsScopeLabel = "Active Projects"

// crossProjectFocusContext returns the full project-qualified Focus path.
// Width fitting belongs to the shared title renderer.
func (m *OutlinerModel) crossProjectFocusContext() string {
	if len(m.crossFocusStack) == 0 {
		return ""
	}
	top := m.crossFocusStack[len(m.crossFocusStack)-1]
	project := m.crossResolveProject(top.ProjectID)
	if project == nil {
		return ""
	}
	return taskFocusContext(project, project.GetTaskByID(top.TaskID))
}
