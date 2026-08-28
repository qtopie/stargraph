package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// NodeLinkGraph 标准前端图可视化结构 (D3.js / Cytoscape / ECharts / React Flow)
type NodeLinkGraph struct {
	Nodes []*NodeLinkNode `json:"nodes"`
	Links []*NodeLinkLink `json:"links"`
}

type NodeLinkNode struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
}

type NodeLinkLink struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Relation    string  `json:"relation"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
}

// ToNodeLinkJSON 将实体与关系转换为标准 Web 前端友好的 Node-Link JSON 格式
func ToNodeLinkJSON(nodes []*Node, edges []*Edge) ([]byte, error) {
	g := NodeLinkGraph{
		Nodes: make([]*NodeLinkNode, 0, len(nodes)),
		Links: make([]*NodeLinkLink, 0, len(edges)),
	}

	for _, n := range nodes {
		g.Nodes = append(g.Nodes, &NodeLinkNode{
			ID:          n.ID,
			Name:        n.Name,
			Type:        n.Type,
			Description: n.Description,
			Weight:      n.Weight,
		})
	}

	for _, e := range edges {
		g.Links = append(g.Links, &NodeLinkLink{
			Source:      e.SourceID,
			Target:      e.TargetID,
			Relation:    e.Relation,
			Description: e.Description,
			Weight:      e.Weight,
		})
	}

	return json.MarshalIndent(g, "", "  ")
}

// ToDOT 将图转换为 Graphviz DOT 纯文本格式
func ToDOT(nodes []*Node, edges []*Edge, graphName string) string {
	if graphName == "" {
		graphName = "StarGraph"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "digraph %s {\n", graphName)
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=rounded, fontname=\"Arial\"];\n")
	sb.WriteString("  edge [fontname=\"Arial\", fontsize=10];\n\n")

	for _, n := range nodes {
		label := fmt.Sprintf("%s\\n(%s)", n.Name, n.Type)
		fmt.Fprintf(&sb, "  \"%s\" [label=\"%s\"];\n", n.ID, label)
	}

	sb.WriteString("\n")
	for _, e := range edges {
		fmt.Fprintf(&sb, "  \"%s\" -> \"%s\" [label=\"%s\", weight=%.1f];\n", e.SourceID, e.TargetID, e.Relation, e.Weight)
	}

	sb.WriteString("}\n")
	return sb.String()
}

// ToGraphML 导出为标准 XML GraphML 格式 (供 Gephi 桌面软件分析)
func ToGraphML(w io.Writer, nodes []*Node, edges []*Edge) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<graphml xmlns="http://graphml.graphdrawing.org/xmlns">
  <key id="name" for="node" attr.name="name" attr.type="string"/>
  <key id="type" for="node" attr.name="type" attr.type="string"/>
  <key id="desc" for="node" attr.name="description" attr.type="string"/>
  <key id="rel" for="edge" attr.name="relation" attr.type="string"/>
  <key id="weight" for="edge" attr.name="weight" attr.type="double"/>
  <graph id="G" edgedefault="directed">
`)

	for _, n := range nodes {
		fmt.Fprintf(&sb, "    <node id=\"%s\">\n", n.ID)
		fmt.Fprintf(&sb, "      <data key=\"name\">%s</data>\n", escapeXML(n.Name))
		fmt.Fprintf(&sb, "      <data key=\"type\">%s</data>\n", escapeXML(n.Type))
		fmt.Fprintf(&sb, "      <data key=\"desc\">%s</data>\n", escapeXML(n.Description))
		sb.WriteString("    </node>\n")
	}

	for _, e := range edges {
		fmt.Fprintf(&sb, "    <edge id=\"%s\" source=\"%s\" target=\"%s\">\n", e.ID, e.SourceID, e.TargetID)
		fmt.Fprintf(&sb, "      <data key=\"rel\">%s</data>\n", escapeXML(e.Relation))
		fmt.Fprintf(&sb, "      <data key=\"weight\">%.2f</data>\n", e.Weight)
		sb.WriteString("    </edge>\n")
	}

	sb.WriteString("  </graph>\n</graphml>\n")
	_, err := io.WriteString(w, sb.String())
	return err
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
