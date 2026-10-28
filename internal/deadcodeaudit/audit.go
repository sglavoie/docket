// Package deadcodeaudit finds production declarations and struct fields that
// ordinary call-graph dead-code analysis cannot see.
package deadcodeaudit

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Finding is one declaration that has no production consumer.
type Finding struct {
	Position token.Position
	Symbol   string
	Kind     string
}

func (f Finding) String() string {
	if f.Kind == "message" {
		return fmt.Sprintf("%s: production-consumed message %s has no production emitter", f.Position, f.Symbol)
	}
	return fmt.Sprintf("%s: production-unused %s %s", f.Position, f.Kind, f.Symbol)
}

type declaration struct {
	position token.Position
	symbol   string
	kind     string
	readKey  string
	exempt   bool
}

type messageUsage struct {
	position token.Position
	symbol   string
	consumed bool
	produced bool
	exempt   bool
}

// Run loads patterns without test variants and returns production-unused
// package declarations and concrete struct fields, plus TUI messages that are
// consumed in production without a production emitter. Tagged and migration
// DTO fields are indirect serialization inputs and are intentionally exempt.
func Run(dir string, patterns ...string) ([]Finding, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	cfg := &packages.Config{
		Dir:   dir,
		Tests: false,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedModule,
	}
	loaded, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if count := packages.PrintErrors(loaded); count > 0 {
		return nil, fmt.Errorf("loading production packages: %d errors", count)
	}

	modulePackages := collectMainModulePackages(loaded)
	declarations := make(map[types.Object]*declaration)
	reads := make(map[types.Object]bool)
	readKeys := make(map[string]bool)
	messages := make(map[string]*messageUsage)
	for _, pkg := range modulePackages {
		collectDeclarations(pkg, declarations)
		collectReads(pkg, reads, readKeys)
		collectMessageDeclarations(pkg, messages)
	}
	for _, pkg := range modulePackages {
		collectMessageUsage(pkg, messages)
	}

	findings := make([]Finding, 0)
	for object, decl := range declarations {
		if decl.exempt || reads[object] || readKeys[decl.readKey] {
			continue
		}
		findings = append(findings, Finding{
			Position: decl.position,
			Symbol:   decl.symbol,
			Kind:     decl.kind,
		})
	}
	for _, message := range messages {
		if message.exempt || !message.consumed || message.produced {
			continue
		}
		findings = append(findings, Finding{
			Position: message.position,
			Symbol:   message.symbol,
			Kind:     "message",
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.Position.Filename != right.Position.Filename {
			return left.Position.Filename < right.Position.Filename
		}
		if left.Position.Line != right.Position.Line {
			return left.Position.Line < right.Position.Line
		}
		return left.Symbol < right.Symbol
	})
	return findings, nil
}

func collectMainModulePackages(roots []*packages.Package) []*packages.Package {
	seen := make(map[*packages.Package]bool)
	var result []*packages.Package
	var visit func(*packages.Package)
	visit = func(pkg *packages.Package) {
		if pkg == nil || seen[pkg] {
			return
		}
		seen[pkg] = true
		if pkg.Module != nil && pkg.Module.Main {
			result = append(result, pkg)
		}
		for _, imported := range pkg.Imports {
			visit(imported)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return result
}

func collectDeclarations(pkg *packages.Package, out map[types.Object]*declaration) {
	if isTestSupportPackage(pkg) {
		return
	}
	parents := parentMap(pkg.Syntax)
	for ident, object := range pkg.TypesInfo.Defs {
		if ident == nil || object == nil || ident.Name == "_" {
			continue
		}
		position := pkg.Fset.Position(ident.Pos())
		switch typed := object.(type) {
		case *types.TypeName:
			if object.Parent() != pkg.Types.Scope() {
				continue
			}
			exempt := typeSpecHasDirective(parents[ident], parents)
			out[object] = newDeclaration(object, typed.Name(), position, "type", exempt)
		case *types.Const:
			if object.Parent() != pkg.Types.Scope() {
				continue
			}
			out[object] = newDeclaration(object, typed.Name(), position, "constant", false)
		case *types.Var:
			switch {
			case typed.IsField():
				owner := namedFieldOwner(ident, parents)
				if owner == "" {
					continue
				}
				exempt := fieldExempt(pkg, ident, typed, parents)
				out[object] = newDeclaration(object, owner+"."+typed.Name(), position, "field", exempt)
			case object.Parent() == pkg.Types.Scope():
				exempt := valueSpecHasDirective(ident, parents, "go:embed")
				out[object] = newDeclaration(object, typed.Name(), position, "variable", exempt)
			}
		}
	}
}

func newDeclaration(object types.Object, symbol string, position token.Position, kind string, exempt bool) *declaration {
	return &declaration{
		position: position,
		symbol:   symbol,
		kind:     kind,
		readKey:  objectReadKey(object),
		exempt:   exempt,
	}
}

func objectReadKey(object types.Object) string {
	if object == nil || object.Pkg() == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s", object.Pkg().Path(), object.Pos(), object.Name())
}

func namedFieldOwner(ident *ast.Ident, parents map[ast.Node]ast.Node) string {
	for node := ast.Node(ident); node != nil; node = parents[node] {
		if spec, ok := node.(*ast.TypeSpec); ok {
			return spec.Name.Name
		}
	}
	return ""
}

func fieldExempt(pkg *packages.Package, ident *ast.Ident, field *types.Var, parents map[ast.Node]ast.Node) bool {
	if field.Embedded() || isMigrationPath(pkg.Fset.Position(ident.Pos()).Filename) {
		return true
	}
	fieldNode, _ := parents[ident].(*ast.Field)
	if fieldNode != nil && fieldNode.Tag != nil {
		return true
	}
	for node := ast.Node(ident); node != nil; node = parents[node] {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if typeSpecHasDirective(spec, parents) {
			return true
		}
		break
	}
	return false
}

func isMigrationPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for _, marker := range []string{"migration", "legacy", "snapshot", "conversion"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func typeSpecHasDirective(node ast.Node, parents map[ast.Node]ast.Node) bool {
	spec, _ := node.(*ast.TypeSpec)
	if spec == nil {
		return false
	}
	if spec.Doc != nil && strings.Contains(spec.Doc.Text(), "deadcodeaudit:reflect") {
		return true
	}
	declaration, _ := parents[spec].(*ast.GenDecl)
	return declaration != nil && declaration.Doc != nil && strings.Contains(declaration.Doc.Text(), "deadcodeaudit:reflect")
}

func valueSpecHasDirective(ident *ast.Ident, parents map[ast.Node]ast.Node, directive string) bool {
	spec, _ := parents[ident].(*ast.ValueSpec)
	if spec == nil {
		return false
	}
	for _, group := range []*ast.CommentGroup{spec.Doc, spec.Comment} {
		if commentGroupContains(group, directive) {
			return true
		}
	}
	if declaration, ok := parents[spec].(*ast.GenDecl); ok && declaration.Doc != nil {
		return commentGroupContains(declaration.Doc, directive)
	}
	return false
}

// CommentGroup.Text intentionally removes compiler directives such as
// //go:embed. Inspect the raw comments when a declaration's liveness is
// provided by a directive rather than by an ordinary Go identifier use.
func commentGroupContains(group *ast.CommentGroup, directive string) bool {
	if group == nil {
		return false
	}
	for _, comment := range group.List {
		if strings.Contains(comment.Text, directive) {
			return true
		}
	}
	return false
}

func collectReads(pkg *packages.Package, reads map[types.Object]bool, readKeys map[string]bool) {
	if isTestSupportPackage(pkg) {
		return
	}
	parents := parentMap(pkg.Syntax)
	for ident, object := range pkg.TypesInfo.Uses {
		field, isField := object.(*types.Var)
		if !isField || !field.IsField() || !writeOnlyFieldUse(ident, parents) {
			reads[object] = true
			readKeys[objectReadKey(object)] = true
		}
	}
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(node ast.Node) bool {
			if binary, ok := node.(*ast.BinaryExpr); ok && (binary.Op == token.EQL || binary.Op == token.NEQ) {
				markComparedFields(pkg.TypesInfo.TypeOf(binary.X), reads, readKeys, map[types.Type]bool{})
			}
			return true
		})
	}
}

// markComparedFields records every field an == or != between two values of t
// reads: Go compares structs field by field, so a field only ever consulted
// through a whole-value comparison is still read. Nested structs and arrays
// are compared the same way.
func markComparedFields(t types.Type, reads map[types.Object]bool, readKeys map[string]bool, seen map[types.Type]bool) {
	if t == nil || seen[t] {
		return
	}
	seen[t] = true
	switch underlying := t.Underlying().(type) {
	case *types.Struct:
		for index := range underlying.NumFields() {
			field := underlying.Field(index)
			reads[field] = true
			readKeys[objectReadKey(field)] = true
			markComparedFields(field.Type(), reads, readKeys, seen)
		}
	case *types.Array:
		markComparedFields(underlying.Elem(), reads, readKeys, seen)
	}
}

func isTestSupportPackage(pkg *packages.Package) bool {
	return pkg != nil && strings.HasSuffix(pkg.PkgPath, "/internal/testutil")
}

// isTUIPackage reports whether pkg is the TUI shell package or any package
// beneath it. Containment rather than an exact suffix is what keeps a *Msg type
// audited once it moves from the shell into a TUI sub-package.
func isTUIPackage(pkg *packages.Package) bool {
	const root = "/internal/tui"
	return strings.HasSuffix(pkg.PkgPath, root) || strings.Contains(pkg.PkgPath, root+"/")
}

func collectMessageDeclarations(pkg *packages.Package, out map[string]*messageUsage) {
	if pkg == nil || isTestSupportPackage(pkg) || !isTUIPackage(pkg) {
		return
	}
	parents := parentMap(pkg.Syntax)
	for ident, object := range pkg.TypesInfo.Defs {
		typeName, ok := object.(*types.TypeName)
		if !ok || ident == nil || object.Parent() != pkg.Types.Scope() || !strings.HasSuffix(typeName.Name(), "Msg") {
			continue
		}
		if _, isInterface := typeName.Type().Underlying().(*types.Interface); isInterface {
			continue
		}
		key := objectReadKey(object)
		out[key] = &messageUsage{
			position: pkg.Fset.Position(ident.Pos()),
			symbol:   typeName.Name(),
			exempt:   typeSpecHasMessageDirective(parents[ident], parents),
		}
	}
}

func collectMessageUsage(pkg *packages.Package, messages map[string]*messageUsage) {
	if pkg == nil || isTestSupportPackage(pkg) || !isTUIPackage(pkg) {
		return
	}
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.TypeSwitchStmt:
				for _, statement := range typed.Body.List {
					clause, ok := statement.(*ast.CaseClause)
					if !ok {
						continue
					}
					for _, expression := range clause.List {
						markMessageType(pkg, messages, expression, func(message *messageUsage) {
							message.consumed = true
						})
					}
				}
			case *ast.CompositeLit:
				markMessageType(pkg, messages, typed.Type, func(message *messageUsage) {
					message.produced = true
				})
			case *ast.CallExpr:
				markMessageType(pkg, messages, typed.Fun, func(message *messageUsage) {
					message.produced = true
				})
			}
			return true
		})
	}
}

func markMessageType(
	pkg *packages.Package,
	messages map[string]*messageUsage,
	expression ast.Expr,
	mark func(*messageUsage),
) {
	var object types.Object
	switch typed := expression.(type) {
	case *ast.Ident:
		object = pkg.TypesInfo.Uses[typed]
	case *ast.SelectorExpr:
		object = pkg.TypesInfo.Uses[typed.Sel]
	case *ast.StarExpr:
		markMessageType(pkg, messages, typed.X, mark)
		return
	default:
		return
	}
	typeName, ok := object.(*types.TypeName)
	if !ok {
		return
	}
	if message := messages[objectReadKey(typeName)]; message != nil {
		mark(message)
	}
}

func typeSpecHasMessageDirective(node ast.Node, parents map[ast.Node]ast.Node) bool {
	spec, _ := node.(*ast.TypeSpec)
	if spec == nil {
		return false
	}
	const directive = "deadcodeaudit:external-message"
	if commentGroupContains(spec.Doc, directive) {
		return true
	}
	declaration, _ := parents[spec].(*ast.GenDecl)
	return declaration != nil && commentGroupContains(declaration.Doc, directive)
}

func writeOnlyFieldUse(ident *ast.Ident, parents map[ast.Node]ast.Node) bool {
	node := ast.Node(ident)
	if keyed, ok := parents[node].(*ast.KeyValueExpr); ok && keyed.Key == node {
		return true
	}
	if selector, ok := parents[node].(*ast.SelectorExpr); ok && selector.Sel == ident {
		node = selector
	}
	parent := parents[node]
	switch typed := parent.(type) {
	case *ast.AssignStmt:
		for _, lhs := range typed.Lhs {
			if lhs == node {
				return true
			}
		}
	case *ast.IncDecStmt:
		return typed.X == node
	case *ast.RangeStmt:
		return typed.Key == node || typed.Value == node
	}
	return false
}

func parentMap(files []*ast.File) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	for _, file := range files {
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			if len(stack) > 0 {
				parents[node] = stack[len(stack)-1]
			}
			stack = append(stack, node)
			return true
		})
	}
	return parents
}
