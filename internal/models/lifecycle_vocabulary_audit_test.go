package models

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestRuntimeLifecycleVocabularyIsCanonical prevents retired task lifecycle
// symbols and serialized fields from returning outside the narrow legacy decode
// boundaries. Project status, blocker status, and UI message status are unrelated
// concepts and are intentionally not banned.
func TestRuntimeLifecycleVocabularyIsCanonical(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve audit test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))

	retiredIdentifiers := map[string]bool{
		"TaskStatus": true, "SectionForState": true, "LegacyStatusForState": true,
		"DeriveWorkflowState": true, "TargetStateForStatus": true, "StateForSection": true,
		"SetSection": true, "GetSections": true, "EnsureSection": true, "HasSection": true,
		"SectionColors": true, "FieldStatus": true, "FieldSection": true,
		"ActionSetStatus": true, "ActionSetSection": true,
		"StatusTodo": true, "StatusInProgress": true, "StatusReview": true,
		"StatusStandby": true, "StatusDone": true, "StatusBlocked": true,
		"StatusCanceled": true, "StatusMaybe": true,
	}
	legacyTags := map[string]bool{
		"status": true, "section": true, "stage": true,
		"sections": true, "section_colors": true,
	}
	allowedLegacyTagFiles := map[string]bool{
		"internal/config/settings_section_colors.go": true,
		"internal/template/template.go":              true,
		"internal/tui/uitest_types.go":               true,
		"internal/workflow/rule.go":                  true,
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		fileSet := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			t.Errorf("%s: parse: %v", relative, parseErr)
			return nil
		}

		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.Ident:
				if retiredIdentifiers[value.Name] {
					t.Errorf("%s:%d: retired runtime lifecycle identifier %q", relative, fileSet.Position(value.Pos()).Line, value.Name)
				}
			case *ast.Field:
				if value.Tag == nil {
					break
				}
				raw, unquoteErr := strconv.Unquote(value.Tag.Value)
				if unquoteErr != nil {
					break
				}
				for _, namespace := range []string{"json", "yaml"} {
					name := strings.Split(reflect.StructTag(raw).Get(namespace), ",")[0]
					if legacyTags[name] && !allowedLegacyTagFiles[relative] {
						t.Errorf("%s:%d: legacy lifecycle %s tag %q outside a decoder boundary",
							relative, fileSet.Position(value.Pos()).Line, namespace, name)
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
