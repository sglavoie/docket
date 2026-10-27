package operationhost

import (
	"fmt"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// validateMenuBarSpecs resolves the bar once against the catalog the app just
// registered, so a menu naming an operation that was renamed or retired fails
// at startup rather than opening a menu with a hole in it. The launch view is
// All with no cursor and a hidden bar, which is the state this resolution
// describes; a machine whose file says otherwise changes what the rows show,
// never which rows exist.
func (c *Controller) validateMenuBarSpecs() error {
	strategy, ok := c.Registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		return fmt.Errorf("simplified input strategy is required")
	}
	ctx := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Capabilities.Add(operation.CapabilitySimplifiedInput)
	menus, err := outliner.ResolveMenuEntries(
		c.Registry,
		strategy,
		ctx,
		outliner.OutlinerMenuSpecs(chrome.ModeAll, false, true, true),
	)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(menus))
	for _, menu := range menus {
		if menu.ID == "" || menu.Title == "" {
			return fmt.Errorf("menu %q declares no identity or title", menu.Title)
		}
		if seen[menu.ID] {
			return fmt.Errorf("menu ID %s is declared twice", menu.ID)
		}
		seen[menu.ID] = true
		for _, entry := range menu.Entries {
			if entry.OperationID == "" {
				// Rules and group headings are not actions: a heading may name its
				// group, but neither may advertise a shortcut or checked state.
				if entry.Key != "" || entry.Checked != nil {
					return fmt.Errorf("menu %s separator carries entry content", menu.ID)
				}
				continue
			}
			if entry.Label == "" {
				return fmt.Errorf("menu %s entry %s resolved without a label", menu.ID, entry.OperationID)
			}
		}
	}
	return nil
}
