package graph_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/qtopie/stargraph/pkg/graph"
)

func TestGraphExport(t *testing.T) {
	nodes := []*graph.Node{
		{ID: "entity:A", Name: "Node A", Type: "concept"},
		{ID: "entity:B", Name: "Node B", Type: "technology"},
	}
	edges := []*graph.Edge{
		{ID: "rel:1", SourceID: "entity:A", TargetID: "entity:B", Relation: "CONNECTS_TO", Weight: 1.0},
	}

	// 1. Test NodeLink JSON
	jsonData, err := graph.ToNodeLinkJSON(nodes, edges)
	if err != nil {
		t.Fatalf("ToNodeLinkJSON failed: %v", err)
	}
	if !strings.Contains(string(jsonData), "Node A") {
		t.Errorf("JSON output missing Node A")
	}

	// 2. Test DOT
	dotStr := graph.ToDOT(nodes, edges, "TestGraph")
	if !strings.Contains(dotStr, "digraph TestGraph") {
		t.Errorf("DOT output missing header")
	}

	// 3. Test GraphML
	var buf bytes.Buffer
	if err := graph.ToGraphML(&buf, nodes, edges); err != nil {
		t.Fatalf("ToGraphML failed: %v", err)
	}
	if !strings.Contains(buf.String(), "<graphml") {
		t.Errorf("GraphML output missing header")
	}
}
