package ttbridge

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/bridge"
)

// TestBindingExportsExactlyTheContractSurface keeps the gomobile surface to the
// two functions the contract names: anything else exported here would cross
// into Swift as a generated type.
func TestBindingExportsExactlyTheContractSurface(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "ttbridge.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var exported []string
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Name.IsExported() {
				exported = append(exported, decl.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if spec.Name.IsExported() {
						exported = append(exported, spec.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.IsExported() {
							exported = append(exported, name.Name)
						}
					}
				}
			}
		}
	}
	slices.Sort(exported)
	if !slices.Equal(exported, []string{"BridgeVersion", "Call"}) {
		t.Fatalf("the binding exports %v, want exactly BridgeVersion and Call", exported)
	}
}

func TestBindingAnswersThroughTheSharedBridge(t *testing.T) {
	if BridgeVersion() != bridge.Version {
		t.Fatalf("BridgeVersion() = %d, want %d", BridgeVersion(), bridge.Version)
	}
	var response struct {
		RequestID string `json:"request_id"`
		Error     struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	reply := Call(`{"bridge_version":1,"request_id":"r","op":"query.projects","workspace":"w"}`)
	if err := json.Unmarshal([]byte(reply), &response); err != nil {
		t.Fatalf("Call returned non-JSON %q: %v", reply, err)
	}
	if response.RequestID != "r" || response.Error.Code != bridge.CodeNotOpen {
		t.Fatalf("Call answered %s", reply)
	}
}
