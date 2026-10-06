package compat

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

const contractDocsDir = "../../docs/mobile"

// contractExamples names every tagged JSON example the contract documents must
// carry and the top-level fields each example must spell. The examples are the
// wire shapes later sessions implement against, so a document losing one, or
// an example losing a required field, is a contract change rather than an edit.
var contractExamples = map[string]struct {
	document string
	fields   []string
}{
	"bridge-request":      {"bridge-contract.md", []string{"bridge_version", "request_id", "op", "workspace", "expect", "payload"}},
	"bridge-response":     {"bridge-contract.md", []string{"bridge_version", "request_id", "ok", "error"}},
	"sync-manifest":       {"sync-envelope.md", []string{"format", "format_version", "workspace_id", "txn_id", "device_id", "device_seq", "parents", "kind", "created_at", "schemas", "requires", "effects"}},
	"device-capabilities": {"sync-envelope.md", []string{"format", "format_version", "device_id", "platform", "bridge_versions", "sync_format_versions", "schemas", "capabilities"}},
	"fixture-manifest":    {"fixture-contract.md", []string{"format", "format_version", "name", "workspace", "description", "schemas", "expect", "coverage"}},
}

var taggedJSONFence = regexp.MustCompile("(?s)```json tt:([a-z-]+)\n(.*?)\n```")

func readContractDoc(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(contractDocsDir, name))
	if err != nil {
		t.Fatalf("contract document %s: %v", name, err)
	}
	return data
}

// TestCompatContractDocumentsAreWellFormed parses every tagged example in the
// mobile contract documents and holds it to its required fields, the fixture
// example to the strict manifest decoder, and the effect example to the
// complete-effect shape.
func TestCompatContractDocumentsAreWellFormed(t *testing.T) {
	found := map[string]map[string]any{}
	for _, name := range []string{"README.md", "bridge-contract.md", "sync-envelope.md", "fixture-contract.md", "write-path-inventory.md"} {
		data := readContractDoc(t, name)
		for _, match := range taggedJSONFence.FindAllSubmatch(data, -1) {
			tag := string(match[1])
			spec, ok := contractExamples[tag]
			if !ok {
				t.Errorf("%s: unknown example tag %q", name, tag)
				continue
			}
			if spec.document != name {
				t.Errorf("example %q belongs in %s, found in %s", tag, spec.document, name)
			}
			if _, dup := found[tag]; dup {
				t.Errorf("example %q appears twice", tag)
			}
			var value map[string]any
			if err := json.Unmarshal(match[2], &value); err != nil {
				t.Errorf("%s: example %q is not valid JSON: %v", name, tag, err)
				continue
			}
			found[tag] = value
			if tag == "fixture-manifest" {
				decoder := json.NewDecoder(bytes.NewReader(match[2]))
				decoder.DisallowUnknownFields()
				var manifest fixtureManifest
				if err := decoder.Decode(&manifest); err != nil {
					t.Errorf("fixture-manifest example does not decode strictly: %v", err)
				}
			}
		}
	}
	for tag, spec := range contractExamples {
		value, ok := found[tag]
		if !ok {
			t.Errorf("%s has no ```json tt:%s example", spec.document, tag)
			continue
		}
		for _, field := range spec.fields {
			if _, ok := value[field]; !ok {
				t.Errorf("example %q lacks required field %q", tag, field)
			}
		}
	}
	if manifest, ok := found["sync-manifest"]; ok {
		effects, _ := manifest["effects"].([]any)
		if len(effects) == 0 {
			t.Error("sync-manifest example carries no effects")
		}
		for index, raw := range effects {
			effect, _ := raw.(map[string]any)
			for _, field := range []string{"kind", "path", "op", "base_blob", "blob"} {
				if _, ok := effect[field]; !ok {
					t.Errorf("sync-manifest effect %d lacks %q", index, field)
				}
			}
		}
	}
}

// TestCompatFixtureVocabularyMatchesProbes keeps the documented coverage
// vocabulary and the probes one set.
func TestCompatFixtureVocabularyMatchesProbes(t *testing.T) {
	data := string(readContractDoc(t, "fixture-contract.md"))
	section := data[strings.Index(data, "## Coverage vocabulary"):]
	rows := regexp.MustCompile("(?m)^\\| `([a-z_0-9.]+)` \\|").FindAllStringSubmatch(section, -1)
	var documented []string
	for _, row := range rows {
		documented = append(documented, row[1])
	}
	var probed []string
	for key := range coverageProbes {
		probed = append(probed, key)
	}
	sort.Strings(documented)
	sort.Strings(probed)
	if !slices.Equal(documented, probed) {
		t.Fatalf("documented vocabulary %v\ndiffers from probes %v", documented, probed)
	}
}
