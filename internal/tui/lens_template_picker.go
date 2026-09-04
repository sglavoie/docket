package tui

import (
	"errors"
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// lensTemplatePickerPrefix namespaces the gallery's row IDs. The row identity is
// derived from the template's stable UUID rather than its name, so a catalog
// rename never silently moves the cursor onto a different template.
const lensTemplatePickerPrefix = "lens-template:"

const (
	lensTemplatePickerTitle    = "Add Built-in View"
	lensTemplatePickerSubtitle = "Choose a lens template"
)

// newLensTemplatePicker opens the built-in view gallery. It is a view-level
// provider: the catalog it renders is global, so it resolves no task target and
// needs no entity under the cursor.
//
// Availability is decided here against the latest machine snapshot, and decided
// again inside executeLensAddTemplateOperation against the snapshot the commit
// actually sees. This copy is what greys the row; that copy is what refuses.
func (m *AppModel) newLensTemplatePicker(
	request operation.OperationExecutionRequest,
) (*OperationValuePickerModel, error) {
	if m == nil || m.runtime == nil {
		return nil, errors.New("workspace runtime is unavailable")
	}
	return newOperationValuePickerModel(
		request,
		operation.ValueProviderLensTemplate,
		lensTemplatePickerTitle,
		lensTemplatePickerSubtitle,
		lensTemplateListSource(m.runtime.Machine()),
		false,
		nil,
	)
}

// lensTemplateListSource adapts the frozen catalog into gallery rows, sorted by
// the name the user reads rather than the catalog order the definitions are
// declared in. Unavailable templates keep their row and carry a reason, because
// a template that vanished when it was added would leave the gallery unable to
// say why.
func lensTemplateListSource(machine *config.MachineState) ListSource {
	templates := config.BuiltinNamedLensTemplates()
	sort.SliceStable(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})
	items := make([]ListItem, 0, len(templates))
	for _, template := range templates {
		summary := namedLensSummary(template)
		item := ListItem{
			ID:            lensTemplateItemID(template.ID),
			PrimaryText:   template.Name,
			SecondaryText: summary,
			// The summary is searchable so a template can be found by the facets it
			// sets, exactly as the saved-lens picker lets one be found by its own.
			Aliases:        []string{template.Name, summary},
			DisabledReason: lensTemplateDisabledReason(machine, template),
			// The payload is the stable ID alone. The handler reloads the definition
			// from the catalog, so a row can never commit a projection the catalog
			// has since changed.
			Data: template.ID,
		}
		// The accent is asked of the catalog definition here, exactly as the
		// saved-lens picker asks it of its own record, so a template previews the
		// color it will wear once added and no identity check is needed to make the
		// two surfaces agree. It rides PrimaryStyle beside the payload rather than
		// touching it, and an unavailable row still renders muted: the renderer
		// already drops a disabled row's color, because a template that cannot be
		// added must not advertise one it cannot deliver.
		if style, ok := namedLensPrimaryStyle(template); ok {
			item.PrimaryStyle = &style
		}
		items = append(items, item)
	}
	return ListSource{Items: items}
}

// lensTemplateDisabledReason names the one rule that blocks adding a template,
// or "" when nothing does. The stable ID answers first: a template already added
// is already added whatever it has since been renamed to, which is why a renamed
// or replaced template stays unavailable while a same-name stranger under a
// foreign ID reports the weaker name collision.
func lensTemplateDisabledReason(machine *config.MachineState, template config.NamedLens) string {
	if existing := machine.NamedLensByID(template.ID); existing != nil {
		return fmt.Sprintf("Already added as %q", existing.Name)
	}
	if machine != nil && namedLensNameTaken(machine.NamedLenses, template.Name, template.ID) {
		return fmt.Sprintf("A lens named %q already exists", template.Name)
	}
	return ""
}

func lensTemplateItemID(id string) ListItemID {
	return ListItemID(lensTemplatePickerPrefix + id)
}
