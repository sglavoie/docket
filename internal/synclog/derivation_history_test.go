package synclog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

// synthHistory mints sealed manifests and the blobs they name straight into
// memory, so a history of any shape reaches buildGraph without a workspace, a
// journal, or a device to exchange with.
type synthHistory struct {
	t         testing.TB
	manifests []*Manifest
	blobs     synthBlobs
	seq       map[string]uint64
}

// synthBlobs is the BlobSource a synthetic history's materializer reads.
type synthBlobs map[string][]byte

func (b synthBlobs) Blob(digest string) ([]byte, error) {
	data, ok := b[digest]
	if !ok {
		return nil, fmt.Errorf("no blob %s", digest)
	}
	return data, nil
}

func newSynthHistory(t testing.TB) *synthHistory {
	return &synthHistory{t: t, blobs: synthBlobs{}, seq: map[string]uint64{}}
}

// blob retains content and returns its digest.
func (s *synthHistory) blob(content string) *string {
	digest := Digest([]byte(content))
	s.blobs[digest] = []byte(content)
	return &digest
}

// commit mints one transaction and returns its id.
func (s *synthHistory) commit(device string, kind Kind, parents []string, effects ...Effect) string {
	s.t.Helper()
	sorted := append([]string{}, parents...)
	sort.Strings(sorted)
	s.seq[device]++
	m := &Manifest{
		Format:        ManifestFormat,
		FormatVersion: ManifestFormatVersion,
		WorkspaceID:   "ws",
		DeviceID:      device,
		DeviceSeq:     s.seq[device],
		Parents:       sorted,
		Kind:          kind,
		CreatedAt:     "2026-10-12T12:00:00Z",
		Schemas:       map[string]int{SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1},
		Requires:      []string{},
		Effects:       effects,
	}
	if err := m.seal(); err != nil {
		s.t.Fatal(err)
	}
	s.manifests = append(s.manifests, m)
	return m.TxnID
}

// materializerOver is a materializer over g whose blobs come from memory and
// whose workspace is an empty directory.
func (s *synthHistory) materializerOver(g *graph) *materializer {
	s.t.Helper()
	h, err := New(Config{
		DataDir:  s.t.TempDir(),
		DeviceID: "observer",
		Schemas:  map[string]int{SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1},
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return newMaterializer(h, g, synthMerger{}, s.blobs)
}

// synthCatalog is the slice of the catalog reconciliation reads, in the
// envelope parseCatalog decodes.
type synthCatalog struct {
	WorkspaceID string           `json:"workspace_id"`
	Data        synthCatalogData `json:"data"`
}

type synthCatalogData struct {
	Projects []synthProject `json:"projects"`
	Revision int            `json:"revision,omitempty"`
}

type synthProject struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
}

func encodeSynthCatalog(revision int, projects []synthProject) string {
	sorted := append([]synthProject{}, projects...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	data, err := json.Marshal(synthCatalog{WorkspaceID: "ws", Data: synthCatalogData{Projects: sorted, Revision: revision}})
	if err != nil {
		panic(err)
	}
	return string(data)
}

// synthMerger is a pure Merger over opaque bytes: a merge is a function of its
// base and sides alone, a fixed share of them report a conflict, and a catalog
// merge stays a catalog.
type synthMerger struct{}

func (synthMerger) Merge(req MergeRequest) (MergeResult, error) {
	if req.Kind == EffectCatalog {
		return mergeSynthCatalogs(req)
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "merge of %s over %s:", req.Key, Digest(req.Base.Data))
	for _, side := range req.Sides {
		fmt.Fprintf(&b, " %s", Digest(side.Data))
	}
	result := MergeResult{Data: b.Bytes()}
	if digest := Digest(result.Data); digest[len(digest)-1] < '8' {
		result.Conflicts = []ConflictDetail{{Subject: req.Key, Field: "body", Detail: "both sides changed it"}}
	}
	return result, nil
}

func mergeSynthCatalogs(req MergeRequest) (MergeResult, error) {
	slugs := map[string]string{}
	revision := 0
	var result MergeResult
	for _, side := range req.Sides {
		var catalog synthCatalog
		if err := json.Unmarshal(side.Data, &catalog); err != nil {
			return MergeResult{}, err
		}
		revision += catalog.Data.Revision
		for _, project := range catalog.Data.Projects {
			if held, ok := slugs[project.ID]; ok && held != project.Slug {
				result.Conflicts = append(result.Conflicts, ConflictDetail{Subject: project.ID, Field: "slug", Detail: "renamed on both sides"})
				continue
			}
			slugs[project.ID] = project.Slug
		}
	}
	projects := make([]synthProject, 0, len(slugs))
	for id, slug := range slugs {
		projects = append(projects, synthProject{ID: id, Slug: slug})
	}
	result.Data = []byte(encodeSynthCatalog(revision, projects))
	return result, nil
}

func (synthMerger) RestoreProject(catalog []byte, projectID string, from []byte) ([]byte, bool, error) {
	var current, earlier synthCatalog
	if err := json.Unmarshal(catalog, &current); err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(from, &earlier); err != nil {
		return nil, false, err
	}
	for _, project := range earlier.Data.Projects {
		if project.ID == projectID {
			restored := append(current.Data.Projects, project)
			return []byte(encodeSynthCatalog(current.Data.Revision, restored)), true, nil
		}
	}
	return nil, false, nil
}
