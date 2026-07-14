package tui

import "github.com/sglavoie/tt/internal/models"

// This file populates the RouteFactory seam that session 29 defined but left nil
// for every route. A RouteFactory constructs a feature view from shared
// ViewContext plus the route's concrete parameters, so navigation can build a
// primary view without the shell reaching into that view's package internals.
//
// Only the primary feature views cleanly constructible from ViewContext + a
// resolved project are registered here (Dashboard, project tree, outliner, quick
// capture, and the two inbox processors). Modal editors that need extra runtime
// bindings (a selected task, a source project) — the task forms and the project
// selector — keep their bespoke construction until RouteParams grows to carry
// those bindings in a later session. Registering the tractable primary factories
// now is exactly the incremental "populate factories as you migrate each feature
// view's construction" step the router was built for.

// resolveRouteProject resolves the project a route is bound to: the route's
// ProjectID param when present and known, otherwise the workspace's current
// project. It returns nil only when neither is available, which the caller's
// constructor tolerates (an empty project view).
func resolveRouteProject(ctx ViewContext, params RouteParams) *models.Project {
	if ctx.Runtime == nil {
		return nil
	}
	if params.ProjectID != "" {
		if p := ctx.Runtime.GetProject(params.ProjectID); p != nil {
			return p
		}
	}
	return ctx.Runtime.CurrentProject()
}

// primaryFeatureFactories returns the RouteFactory for each primary feature view
// that can be built from ViewContext and a resolved project. Views absent from
// this map keep a nil Factory and are still constructed by their legacy call
// sites, so populating a subset is safe and drift-free.
func primaryFeatureFactories() map[ViewType]RouteFactory {
	return map[ViewType]RouteFactory{
		ViewDashboard: func(ctx ViewContext, _ RouteParams) View {
			return NewDashboardModel(ctx.Runtime)
		},
		ViewProjectTree: func(ctx ViewContext, params RouteParams) View {
			return NewProjectTreeModel(resolveRouteProject(ctx, params), ctx.ConfigManager, ctx.Runtime)
		},
		ViewOutliner: func(ctx ViewContext, params RouteParams) View {
			return NewOutlinerModel(resolveRouteProject(ctx, params), ctx.ConfigManager, ctx.Runtime)
		},
		ViewQuickCapture: func(ctx ViewContext, params RouteParams) View {
			var allProjects []*models.Project
			if ctx.Runtime != nil {
				allProjects = ctx.Runtime.ActiveProjects()
			}
			return NewQuickCaptureModel(resolveRouteProject(ctx, params), allProjects, false)
		},
		ViewInboxProcessor: func(ctx ViewContext, params RouteParams) View {
			return NewInboxProcessorModel(resolveRouteProject(ctx, params), ctx.ConfigManager, ctx.Runtime)
		},
		ViewGlobalInboxProcessor: func(ctx ViewContext, _ RouteParams) View {
			return NewGlobalInboxProcessorModel(ctx.ConfigManager, ctx.Runtime)
		},
	}
}
