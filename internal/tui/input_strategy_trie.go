package tui

import (
	"fmt"
	"slices"
	"strings"
)

// inputChordNode is one node of a strategy's key trie. bindings is empty on
// prefix nodes; childOrder keeps which-key hints in declaration order. A
// terminal normally holds exactly one binding, and holds several only when every
// one of them opted into SharedKey.
type inputChordNode struct {
	children   map[string]*inputChordNode
	childOrder []string
	bindings   []int
	owners     []OperationID
	shared     bool
}

func newInputChordNode() *inputChordNode {
	return &inputChordNode{children: make(map[string]*inputChordNode)}
}

func (node *inputChordNode) child(key string) *inputChordNode {
	if node == nil {
		return nil
	}
	return node.children[key]
}

func (node *inputChordNode) terminal() bool {
	return node != nil && len(node.bindings) > 0
}

func (node *inputChordNode) ownerList() string {
	names := make([]string, 0, len(node.owners))
	for _, owner := range node.owners {
		names = append(names, string(owner))
	}
	return strings.Join(names, ", ")
}

// insert adds one prefix-free path. A path that passes through or stops on an
// existing terminal, or that ends on a node another chord continues past, is a
// grammar defect rather than a runtime ambiguity to resolve later.
func (node *inputChordNode) insert(
	strategyID InputStrategyID,
	operationID OperationID,
	path []string,
	bindingIndex int,
	shared bool,
) error {
	current := node
	for _, key := range path {
		if current.terminal() {
			return fmt.Errorf(
				"input strategy %s chord prefix collision: %s already ends a binding inside %s (%s vs %s)",
				strategyID,
				chordPathString(path[:len(path)-1]),
				chordPathString(path),
				current.ownerList(),
				operationID,
			)
		}
		next := current.children[key]
		if next == nil {
			next = newInputChordNode()
			current.children[key] = next
			current.childOrder = append(current.childOrder, key)
		}
		current = next
	}
	if current.terminal() {
		if !shared || !current.shared || slices.Contains(current.owners, operationID) {
			return fmt.Errorf(
				"input strategy %s key collision: %s (%s vs %s)",
				strategyID,
				chordPathString(path),
				current.ownerList(),
				operationID,
			)
		}
	}
	if len(current.children) > 0 {
		return fmt.Errorf(
			"input strategy %s chord prefix collision: %s is a prefix of a longer chord (%s vs %s)",
			strategyID,
			chordPathString(path),
			current.ownerList(),
			operationID,
		)
	}
	current.bindings = append(current.bindings, bindingIndex)
	current.owners = append(current.owners, operationID)
	current.shared = shared
	return nil
}

// prefixPaths returns every non-root prefix path in declaration order.
func (node *inputChordNode) prefixPaths(prefix []string) [][]string {
	if node == nil {
		return nil
	}
	paths := make([][]string, 0)
	for _, key := range node.childOrder {
		child := node.children[key]
		if child.terminal() {
			continue
		}
		path := append(append([]string(nil), prefix...), key)
		paths = append(paths, path)
		paths = append(paths, child.prefixPaths(path)...)
	}
	return paths
}

func chordPathString(path []string) string {
	if len(path) == 0 {
		return "(root)"
	}
	return strings.Join(path, " ")
}
