package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestSoleTUIFoundations(t *testing.T) {
	t.Parallel()

	forbiddenIdentifiers := map[string]bool{
		"BasePicker":                              true,
		"SelectorModel":                           true,
		"PaletteAction":                           true,
		"CommandRegistry":                         true,
		"KeybindingRegistry":                      true,
		"KeySyncView":                             true,
		"PrefixNode":                              true,
		"repeatableAction":                        true,
		"RecordRepeatableActionMsg":               true,
		"prefixableView":                          true,
		"BulkOperationComponent":                  true,
		"InputStrategyLegacy":                     true,
		"InboxProcessorModel":                     true,
		"GlobalInboxProcessorModel":               true,
		"PerspectiveMixin":                        true,
		"ModeCategorySmart":                       true,
		"OrderPolicyLens":                         true,
		"IsLensHeader":                            true,
		"ReviewLanding":                           true,
		"ManageLanding":                           true,
		"routeForPresentation":                    true,
		"ViewOutliner":                            true,
		"ViewFocusBoard":                          true,
		"ViewCommandPalette":                      true,
		"CockpitCommandOpenCommandPalette":        true,
		"OverlayPalette":                          true,
		"GlobalProjectPickerModel":                true,
		"GlobalTaskPickerModel":                   true,
		"TaskQuickSwitchModel":                    true,
		"ShowQuickCaptureMsg":                     true,
		"CloseQuickCaptureMsg":                    true,
		"ShowBulkCaptureMsg":                      true,
		"CloseBulkCaptureMsg":                     true,
		"ViewQuickCapture":                        true,
		"ViewBulkCapture":                         true,
		"QuickCaptureModel":                       true,
		"BulkCaptureModel":                        true,
		"PinnedTasksModel":                        true,
		"RecentTasksModel":                        true,
		"ViewPinnedTasks":                         true,
		"ViewRecentTasks":                         true,
		"PinnedPreviewModel":                      true,
		"ShowAttrFilterPickerMsg":                 true,
		"AttrFilterPickerModel":                   true,
		"ViewAttrFilterPicker":                    true,
		"projectTasksGroup":                       true,
		"buildCrossProjectRowsWithSectionHeaders": true,
		"buildProjectGroupedRows":                 true,
		"legacyAllPresentationGroup":              true,
		"legacyAllPresentationGroupOrder":         true,
		"legacySectionForStateGroup":              true,
		"legacyStatusColumnForState":              true,
		"renderedHeaderStateGroupForTask":         true,
	}
	requiredTypes := map[string]int{
		"OperationRegistry": 0,
		"ListSurfaceModel":  0,
		"Route":             0,
		"Overlay":           0,
		"OutlinerModel":     0,
	}
	forbiddenFiles := map[string]bool{
		"base_picker.go":                             true,
		"base_picker_render.go":                      true,
		"base_picker_always_search.go":               true,
		"selector.go":                                true,
		"command.go":                                 true,
		"command_registry.go":                        true,
		"command_palette.go":                         true,
		"keybindings.go":                             true,
		"handled_keys.go":                            true,
		"repeat_action.go":                           true,
		"shared_primary_types.go":                    true,
		"bulk_operation_component.go":                true,
		"presentation_routes.go":                     true,
		"perspective_mixin.go":                       true,
		"outliner_perspectives.go":                   true,
		"pinned_preview.go":                          true,
		"global_project_picker.go":                   true,
		"global_task_picker.go":                      true,
		"task_quick_switch.go":                       true,
		"task_quick_switch_filter.go":                true,
		"task_quick_switch_render.go":                true,
		"task_quick_switch_update.go":                true,
		"outliner_scope_picker.go":                   true,
		"attribute_filter_picker.go":                 true,
		"handler_attr_filter.go":                     true,
		"cross_project_outliner_rows.go":             true,
		"cross_project_outliner_mutations_status.go": true,
	}

	fset := token.NewFileSet()
	appOutlinerFields := 0
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		name := entry.Name()
		if forbiddenFiles[name] ||
			strings.Contains(name, "quick_picker") ||
			strings.Contains(name, "prefix") ||
			strings.HasPrefix(name, "palette_") ||
			strings.HasPrefix(name, "inbox_") ||
			strings.HasPrefix(name, "global_inbox_") {
			t.Errorf("retired TUI foundation file returned: %s", path)
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.Ident:
				if forbiddenIdentifiers[node.Name] {
					t.Errorf("retired TUI foundation identifier %s returned in %s", node.Name, path)
				}
			case *ast.TypeSpec:
				if _, ok := requiredTypes[node.Name.Name]; ok {
					requiredTypes[node.Name.Name]++
				}
				if node.Name.Name == "AppModel" {
					if model, ok := node.Type.(*ast.StructType); ok {
						for _, field := range model.Fields.List {
							if expressionContainsIdentifier(field.Type, "OutlinerModel") {
								appOutlinerFields += max(len(field.Names), 1)
							}
						}
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production TUI: %v", err)
	}

	for typeName, count := range requiredTypes {
		if count != 1 {
			t.Errorf("foundation type %s definitions = %d, want exactly 1", typeName, count)
		}
	}
	if appOutlinerFields != 1 {
		t.Errorf("AppModel OutlinerModel fields = %d, want exactly 1", appOutlinerFields)
	}
}

func expressionContainsIdentifier(expression ast.Expr, name string) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Ident); ok && identifier.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}
