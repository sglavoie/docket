package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestLensTemplateGalleryColorsEveryTemplateFromItsDefinition(t *testing.T) {
	model := lensOperationApp(t)
	// An empty machine leaves every row available, so this case is about color
	// alone; the unavailable rows get their own test below.
	setMachineLenses(t, model, config.NamedLensList{})
	picker := openLensTemplateGallery(t, model)

	for _, template := range config.BuiltinNamedLensTemplates() {
		row := galleryRow(t, picker, template.ID)
		assertLensRowStyle(t, row, template)
		// Every shipped template resolves a role, so a nil style here would mean
		// the assertion above passed vacuously.
		if row.PrimaryStyle == nil {
			t.Errorf("built-in %q resolves no accent, so this test proves nothing", template.Name)
		}
		// The accent rides PrimaryStyle alone: search, sort, and truncation still
		// see the plain name and the plain summary.
		if row.PrimaryText != ansi.Strip(row.PrimaryText) {
			t.Errorf("row %q embeds styling in its primary text: %q", template.Name, row.PrimaryText)
		}
		if row.SecondaryText != ansi.Strip(row.SecondaryText) {
			t.Errorf("row %q embeds styling in its summary: %q", template.Name, row.SecondaryText)
		}
		// Styling must not weaken the payload contract the handler reloads from.
		if payload, ok := row.Data.(string); !ok || payload != template.ID {
			t.Errorf("%s payload = %#v, want the stable ID %s", template.Name, row.Data, template.ID)
		}
	}
}

// A row that cannot be added still carries its metadata, because the reason it
// is unavailable is the machine's state rather than anything about the
// definition. Whether that metadata is painted is the renderer's call, tested
// separately below.
func TestLensTemplateGalleryKeepsStyleOnUnavailableRows(t *testing.T) {
	waiting := lensTemplateByName(t, "Waiting")
	renamed := waiting.Clone()
	renamed.Name = "Blocked On Someone"

	overdue := lensTemplateByName(t, "Overdue")
	stranger := overdue.Clone()
	stranger.ID = lensTemplateForeignID

	stale := lensTemplateByName(t, "Stale")
	replaced := stale.Clone()
	replaced.Query = "invoice"

	model := lensOperationApp(t)
	setMachineLenses(t, model, config.NamedLensList{renamed, stranger, replaced})
	picker := openLensTemplateGallery(t, model)

	for _, template := range []config.NamedLens{waiting, overdue, stale} {
		row := galleryRow(t, picker, template.ID)
		if row.Enabled() {
			t.Fatalf("%q became available, so this test no longer covers a disabled row", template.Name)
		}
		// The accent still answers to the catalog definition, not to the record
		// that blocked it: a renamed or replaced twin never repaints the gallery.
		assertLensRowStyle(t, row, template)
		if row.PrimaryStyle == nil {
			t.Errorf("unavailable row %q dropped its style metadata", template.Name)
		}
		if payload, ok := row.Data.(string); !ok || payload != template.ID {
			t.Errorf("%s payload = %#v, want the stable ID %s", template.Name, row.Data, template.ID)
		}
	}
}

// The renderer is what keeps an unavailable row honest: it must look exactly as
// it did before any style metadata existed, while an available row shows the
// accent.
func TestLensTemplateGalleryRendersUnavailableRowsMutedRatherThanColored(t *testing.T) {
	waiting := lensTemplateByName(t, "Waiting")
	accent, ok := namedLensPrimaryStyle(waiting)
	if !ok {
		t.Fatal("the Waiting template resolves no accent")
	}

	rendered := func(t *testing.T, model *AppModel) (string, string) {
		t.Helper()
		row := galleryRow(t, openLensTemplateGallery(t, model), waiting.ID)
		plain := row
		plain.PrimaryStyle = nil
		surfaceRow := ListSurfaceRow{Match: ListMatch{Item: row}}
		plainRow := ListSurfaceRow{Match: ListMatch{Item: plain}}
		// The width is generous so the disabled reason survives truncation and the
		// assertions below read the row rather than its ellipsis.
		return renderListSurfaceRow(surfaceRow, ListSelectionSingle, 100),
			renderListSurfaceRow(plainRow, ListSelectionSingle, 100)
	}

	t.Run("unavailable", func(t *testing.T) {
		model := lensOperationApp(t)
		// The starters are seeded, so Waiting is already added.
		got, want := rendered(t, model)
		if strings.Contains(got, accent.Render(waiting.Name)) {
			t.Errorf("a disabled row advertised its accent:\n%q", got)
		}
		if got != want {
			t.Errorf("style metadata changed a disabled row:\n got %q\nwant %q", got, want)
		}
		if !strings.Contains(ansi.Strip(got), `Already added as "Waiting"`) {
			t.Errorf("the disabled row stopped saying why:\n%q", ansi.Strip(got))
		}
	})

	t.Run("available", func(t *testing.T) {
		model := lensOperationApp(t)
		setMachineLenses(t, model, config.NamedLensList{})
		got, want := rendered(t, model)
		if !strings.Contains(got, accent.Render(waiting.Name)) {
			t.Errorf("an available row lost its accent:\n%q", got)
		}
		if ansi.Strip(got) != ansi.Strip(want) {
			t.Errorf("the accent moved text: %q, want %q", ansi.Strip(got), ansi.Strip(want))
		}
	})
}

// Cross-surface parity, asserted through both real models rather than through
// the resolver twice: identical definitions must read identically wherever they
// are listed.
func TestLensTemplateGalleryAndSavedPickerAgreeOnIdenticalDefinitions(t *testing.T) {
	model := lensOperationApp(t)
	// Saving every template under its own identity puts the same nine
	// definitions on both surfaces at once.
	setMachineLenses(t, model, config.BuiltinNamedLensTemplates())
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	if model.lensPicker == nil {
		t.Fatalf("openNamedLensPicker left no picker mounted: %v", model.err)
	}
	picker := openLensTemplateGallery(t, model)

	for _, template := range config.BuiltinNamedLensTemplates() {
		saved := lensPickerItemNamed(t, model, template.Name)
		gallery := galleryRow(t, picker, template.ID)
		switch {
		case saved.PrimaryStyle == nil && gallery.PrimaryStyle == nil:
			t.Errorf("%q is neutral on both surfaces, so this pair proves nothing", template.Name)
		case saved.PrimaryStyle == nil || gallery.PrimaryStyle == nil:
			t.Errorf("%q is colored on one surface only: saved=%v gallery=%v",
				template.Name, saved.PrimaryStyle, gallery.PrimaryStyle)
		case saved.PrimaryStyle.GetForeground() != gallery.PrimaryStyle.GetForeground():
			t.Errorf("%q reads %v saved and %v in the gallery", template.Name,
				saved.PrimaryStyle.GetForeground(), gallery.PrimaryStyle.GetForeground())
		}
	}
}

// Rows are built when a surface opens, so a gallery opened after a theme change
// must carry that theme's accents rather than the ones the package started on.
func TestLensTemplateGalleryAccentsFollowTheAppliedTheme(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })
	overdue := lensTemplateByName(t, "Overdue")

	foregrounds := make(map[string]color.Color, 2)
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
			model := lensOperationApp(t)
			setMachineLenses(t, model, config.NamedLensList{})
			picker := openLensTemplateGallery(t, model)

			for _, template := range config.BuiltinNamedLensTemplates() {
				row := galleryRow(t, picker, template.ID)
				// The row is held to the resolver's value under the theme in force,
				// which is what catches a style captured at package init.
				assertLensRowStyle(t, row, template)
			}
			row := galleryRow(t, picker, overdue.ID)
			if row.PrimaryStyle == nil {
				t.Fatalf("%s theme: Overdue carries no style", themeName)
			}
			foregrounds[themeName] = row.PrimaryStyle.GetForeground()
		})
	}
	if foregrounds["dark"] == foregrounds["light"] {
		t.Errorf("the gallery's Overdue accent is %v in both themes", foregrounds["dark"])
	}
}

func TestLensTemplateGalleryCancelsOnEscape(t *testing.T) {
	model := lensOperationApp(t)
	before := len(machineLenses(t, model))
	picker := openLensTemplateGallery(t, model)

	_, dismissed, msg := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !dismissed {
		t.Fatal("Escape left the gallery open")
	}
	if _, ok := msg.(operation.OperationValueCanceledMsg); !ok {
		t.Fatalf("cancel message = %#v, want OperationValueCanceledMsg", msg)
	}
	if got := len(machineLenses(t, model)); got != before {
		t.Fatalf("cancellation wrote %d records, want %d", got, before)
	}
}
