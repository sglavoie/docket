package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// recoveryGatewayInventory classifies every direct TUI call into an
// application boundary that can append a workspace-ledger entry. A new direct
// boundary fails closed until its transaction-correlation route is named here.
var recoveryGatewayInventory = map[string]string{
	"internal/tui/app_domain_commit.go:AppModel.commitCatalogProjectMutation:CommitProjectArtifactsResult": "returns the transaction ID to project pin and tag feedback",
	"internal/tui/app_domain_commit.go:AppModel.commitRuntimeDomainsWithUndo:CommitProjectArtifactsResult": "returns the transaction ID to project archive feedback",
	"internal/tui/feature_coordinator_secondary.go:TaskItemCoordinator.CommitProject:CommitProject":        "emits BatchTransactionResultMsg with the transaction ID",
	"internal/tui/project_creator.go:ProjectCreatorModel.createProject:CommitProjectMutationResult":        "emits ProjectCreatedMsg with the transaction ID",
	"internal/tui/project_editor.go:projectEditorSave.commit:RenameProjectResult":                          "emits ProjectUpdatedMsg with the transaction ID",
	"internal/tui/project_note_commands.go:noteMutationCmd:ApplyNoteBatch":                                 "emits ProjectNoteMutationResultMsg carrying NoteBatchResult.TransactionID",
}

// These application commits intentionally do not create user-facing ledger
// entries. Keeping them in a separate explicit inventory prevents them from
// being mistaken for silently uncorrelated mutations.
var recoveryGatewayExemptions = map[string]string{
	"internal/tui/app_domain_commit.go:AppModel.commitRuntimeDomainsWithUndo:CommitConfig":      "internal repair/config synchronization with an empty undo action",
	"internal/tui/app_projects.go:AppModel.ensureSystemUnfiledProject:CommitWorkspaceBootstrap": "workspace bootstrap performed before a user-visible project mutation exists",
}

func TestRecoveryFeedbackAuditClassifiesLedgerProducingTUIGateways(t *testing.T) {
	root := findNoteAuditRepoRoot(t)
	actual := collectRecoveryGatewayCalls(t, root)
	var unexpected []string
	seen := make(map[string]bool)
	for _, key := range actual {
		if _, ok := recoveryGatewayInventory[key]; ok {
			seen[key] = true
			continue
		}
		if _, ok := recoveryGatewayExemptions[key]; ok {
			seen[key] = true
			continue
		}
		unexpected = append(unexpected, key)
	}
	var stale []string
	for key := range recoveryGatewayInventory {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	for key := range recoveryGatewayExemptions {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	if len(unexpected) > 0 || len(stale) > 0 {
		sort.Strings(unexpected)
		sort.Strings(stale)
		t.Fatalf("recovery gateway inventory drifted; unexpected=%v stale=%v", unexpected, stale)
	}
}

func TestRecoveryFeedbackAuditReasonsAreExplicit(t *testing.T) {
	for key, reason := range recoveryGatewayInventory {
		assertRecoveryGatewayReason(t, key, reason)
	}
	for key, reason := range recoveryGatewayExemptions {
		assertRecoveryGatewayReason(t, key, reason)
	}
}

func assertRecoveryGatewayReason(t *testing.T, key, reason string) {
	t.Helper()
	lower := strings.ToLower(strings.TrimSpace(reason))
	if lower == "" || strings.Contains(lower, "todo") || strings.Contains(lower, "tbd") {
		t.Fatalf("%s has a placeholder recovery classification: %q", key, reason)
	}
}

func collectRecoveryGatewayCalls(t *testing.T, root string) []string {
	t.Helper()
	tracked := map[string]bool{
		"ApplyNoteBatch":               true,
		"CommitConfig":                 true,
		"CommitProject":                true,
		"CommitProjectArtifactsResult": true,
		"CommitProjectMutationResult":  true,
		"CommitWorkspaceBootstrap":     true,
		"RenameProjectResult":          true,
	}
	var calls []string
	dir := filepath.Join(root, "internal", "tui")
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			owner := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
					if ident, ok := star.X.(*ast.Ident); ok {
						owner = ident.Name + "." + owner
					}
				} else if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
					owner = ident.Name + "." + owner
				}
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if ok && tracked[selector.Sel.Name] {
					calls = append(calls, filepath.ToSlash(rel)+":"+owner+":"+selector.Sel.Name)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collect recovery gateways: %v", err)
	}
	sort.Strings(calls)
	return calls
}
