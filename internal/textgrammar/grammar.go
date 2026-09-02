// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"slices"
)

// Surface identifies a consumer of the shared inline grammar.
func Definitions(surface Surface) []Definition {
	result := make([]Definition, 0, len(registry))
	for _, spec := range registry {
		if spec.Legacy || spec.Surfaces&surface == 0 {
			continue
		}
		forms := formsForSurface(spec.forms, surface)
		if len(forms) == 0 {
			continue
		}
		def := spec.Definition
		// A search-only entry states its meaning once, in searchDescription: it has
		// no mutation reading to describe, so it leaves Description empty in the
		// registry rather than carrying the same sentence twice. Every other entry
		// swaps in the predicate wording on search alone.
		if surface == SurfaceSearch || def.Description == "" {
			def.Description = spec.searchDescription
		}
		def.Forms = slices.Clone(forms)
		def.Values = slices.Clone(def.Values)
		def.RetiredValues = slices.Clone(def.RetiredValues)
		def.Examples = slices.Clone(def.Examples)
		def.Clears = clearsForSurface(def.Clears, surface, spec.searchClearText)
		result = append(result, def)
	}
	return result
}

// GuideDefinitions returns the same definitions Definitions returns for a
// surface, in teach order. Every surface that presents the grammar to a
// human — the capture syntax guide today — reads this projection, so parse
// order stays free to move a field for disambiguation alone.
func GuideDefinitions(surface Surface) []Definition {
	available := Definitions(surface)
	byField := make(map[Field]Definition, len(available))
	for _, def := range available {
		byField[def.Field] = def
	}
	result := make([]Definition, 0, len(available))
	for _, field := range guideOrder {
		if def, ok := byField[field]; ok {
			result = append(result, def)
		}
	}
	return result
}

// DefinitionFor returns the definition for a field on a surface.
func DefinitionFor(surface Surface, field Field) (Definition, bool) {
	for _, def := range Definitions(surface) {
		if def.Field == field {
			return def, true
		}
	}
	return Definition{}, false
}

func formsForSurface(forms []surfaceForm, surface Surface) []string {
	result := make([]string, 0, len(forms))
	for _, form := range forms {
		if form.Surfaces&surface != 0 {
			result = append(result, form.Text)
		}
	}
	return result
}

func clearsForSurface(clears []ClearOperation, surface Surface, searchText map[string]string) []ClearOperation {
	result := make([]ClearOperation, 0, len(clears))
	for _, clear := range clears {
		if clear.Surfaces&surface == 0 {
			continue
		}
		if surface == SurfaceSearch {
			clear.Description = searchText[clear.Value]
		}
		result = append(result, clear)
	}
	return result
}
