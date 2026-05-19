// Package drawio generates draw.io XML from a normalised graph and parses
// draw.io files to extract node positions for layout_overrides.yml.
package drawio

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/ppklau/netdraw/internal/graph"
	"github.com/ppklau/netdraw/internal/icons"
	"github.com/ppklau/netdraw/internal/layout"
	"github.com/ppklau/netdraw/internal/schema"
	"github.com/ppklau/netdraw/internal/views"
)

const (
	deviceW      = 64.0
	deviceH      = 64.0
	labelH       = 28.0 // vertical space below icon for the label
	rowPad       = 12.0 // gap between role-tier rows within a container
	devicePadX   = 20.0
	headerSize   = 30.0
	containerPad = 16.0
	sitePad      = 40.0
	abstractW    = 120.0
	abstractH    = 60.0
	PageWidth    = 1169.0
	PageHeight   = 827.0
)

// roleTier assigns each device role to a row tier within a site container.
// Lower numbers appear nearer the top (edge-facing devices first).
var roleTier = map[string]int{
	"border_router":        0,
	"router":               0,
	"edge_router":          0,
	"firewall":             1,
	"load_balancer":        1,
	"spine":                2,
	"core_switch":          2,
	"distribution_switch":  2,
	"leaf":                 3,
	"access_switch":        3,
	"server":               4,
	"hypervisor":           4,
	"storage":              4,
	"wireless_ap":          5,
	"laptop":               5,
}

func devTier(role string) int {
	if t, ok := roleTier[role]; ok {
		return t
	}
	return 99
}

// XML generates draw.io XML from a filtered graph and view config.
// positions maps node names to absolute page coordinates.
// Pass nil or an empty map to auto-layout all nodes.
func XML(g *graph.Graph, v *views.View, positions map[string]layout.Position) ([]byte, error) {
	if positions == nil {
		positions = make(map[string]layout.Position)
	}

	abstract := make(map[string]bool)
	bySite := make(map[string][]schema.Device)
	for _, d := range g.Devices {
		if d.Role == views.AbstractSiteRole {
			abstract[d.Name] = true
		} else {
			bySite[d.Site] = append(bySite[d.Site], d)
		}
	}

	sortedSites := sortedMapKeys(bySite)
	for _, site := range sortedSites {
		sort.Slice(bySite[site], func(i, j int) bool {
			return bySite[site][i].Name < bySite[site][j].Name
		})
	}
	abstractNames := sortedBoolMapKeys(abstract)

	// Group devices within each site by role tier for multi-row layout.
	type tierMap = map[int][]schema.Device
	siteTierGroups := make(map[string]tierMap)
	for _, site := range sortedSites {
		groups := make(tierMap)
		for _, d := range bySite[site] {
			t := devTier(d.Role)
			groups[t] = append(groups[t], d)
		}
		for _, devs := range groups {
			sort.Slice(devs, func(i, j int) bool { return devs[i].Name < devs[j].Name })
		}
		siteTierGroups[site] = groups
	}

	// Per-site container dimensions (width by widest tier, height by tier count).
	type dims struct{ w, h float64 }
	siteDims := make(map[string]dims)
	for _, site := range sortedSites {
		groups := siteTierGroups[site]
		tiers := sortedIntMapKeys(groups)
		maxInRow := 1
		for _, t := range tiers {
			if len(groups[t]) > maxInRow {
				maxInRow = len(groups[t])
			}
		}
		numRows := len(tiers)
		if numRows == 0 {
			numRows = 1
		}
		rowH := deviceH + labelH
		siteDims[site] = dims{
			w: float64(maxInRow)*(deviceW+devicePadX) + containerPad,
			h: headerSize + float64(numRows)*rowH + float64(numRows-1)*rowPad + containerPad*2,
		}
	}

	// Absolute site positions (left-to-right rows, wrapping at page edge)
	siteAbsPos := make(map[string]layout.Position)
	{
		x, y, maxRowH := 40.0, 60.0, 0.0
		for _, site := range sortedSites {
			if p, ok := positions[site]; ok {
				siteAbsPos[site] = p
				continue
			}
			d := siteDims[site]
			if x+d.w > PageWidth-40 && len(siteAbsPos) > 0 {
				x = 40.0
				y += maxRowH + sitePad
				maxRowH = 0
			}
			siteAbsPos[site] = layout.Position{X: x, Y: y}
			x += d.w + sitePad
			if d.h > maxRowH {
				maxRowH = d.h
			}
		}
	}

	// Device positions relative to site container body (below the header row).
	// Devices are arranged in role-tier rows; within each tier, left-to-right by name.
	devRelPos := make(map[string]layout.Position)
	for _, site := range sortedSites {
		sp := siteAbsPos[site]
		groups := siteTierGroups[site]
		tiers := sortedIntMapKeys(groups)
		rowY := containerPad
		for _, t := range tiers {
			for i, d := range groups[t] {
				if p, ok := positions[d.Name]; ok {
					devRelPos[d.Name] = layout.Position{
						X: p.X - sp.X,
						Y: p.Y - sp.Y - headerSize,
					}
				} else {
					devRelPos[d.Name] = layout.Position{
						X: containerPad/2 + float64(i)*(deviceW+devicePadX),
						Y: rowY,
					}
				}
			}
			rowY += deviceH + labelH + rowPad
		}
	}

	// Normalise device positions and recompute container dimensions for sites
	// whose devices were placed by dagre rather than the tier-row formula.
	// D2's internal container header is shorter than drawio's headerSize, so
	// raw dagre Y values can be zero or negative; shift everything so the
	// topmost device starts at least containerPad below the container body.
	for _, site := range sortedSites {
		hasOverride := false
		for _, dev := range bySite[site] {
			if _, ok := positions[dev.Name]; ok {
				hasOverride = true
				break
			}
		}
		if !hasOverride {
			continue
		}

		// Find the minimum X and Y across all devices in this container.
		minX, minY := devRelPos[bySite[site][0].Name].X, devRelPos[bySite[site][0].Name].Y
		for _, dev := range bySite[site][1:] {
			dp := devRelPos[dev.Name]
			if dp.X < minX {
				minX = dp.X
			}
			if dp.Y < minY {
				minY = dp.Y
			}
		}

		// Shift so that the closest device is inset by at least containerPad from
		// the left edge and 2×containerPad from the top (extra clearance below the
		// drawio swimlane header, which is taller than D2's container header).
		shiftX := containerPad/2 - minX
		shiftY := containerPad*2 - minY
		if shiftX < 0 {
			shiftX = 0
		}
		if shiftY < 0 {
			shiftY = 0
		}
		for _, dev := range bySite[site] {
			dp := devRelPos[dev.Name]
			devRelPos[dev.Name] = layout.Position{X: dp.X + shiftX, Y: dp.Y + shiftY}
		}

		// Recompute container size to fit the normalised device positions.
		var maxRight, maxBottom float64
		for _, dev := range bySite[site] {
			dp := devRelPos[dev.Name]
			if r := dp.X + deviceW; r > maxRight {
				maxRight = r
			}
			if b := dp.Y + deviceH + labelH; b > maxBottom {
				maxBottom = b
			}
		}
		siteDims[site] = dims{
			w: maxRight + containerPad,
			h: headerSize + maxBottom + containerPad,
		}
	}

	// Abstract node positions (placed below the last row of site containers)
	absNodePos := make(map[string]layout.Position)
	{
		bottom := 60.0
		for _, site := range sortedSites {
			d := siteDims[site]
			sp := siteAbsPos[site]
			if b := sp.Y + d.h; b > bottom {
				bottom = b
			}
		}
		ax, ay := 40.0, bottom+sitePad
		for _, name := range abstractNames {
			if p, ok := positions[name]; ok {
				absNodePos[name] = p
			} else {
				absNodePos[name] = layout.Position{X: ax, Y: ay}
				ax += abstractW + sitePad
			}
		}
	}

	// Emit XML
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<mxGraphModel dx="1422" dy="762" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="%d" pageHeight="%d" math="0" shadow="0">`+"\n",
		int(PageWidth), int(PageHeight))
	b.WriteString("  <root>\n")
	b.WriteString("    <mxCell id=\"0\"/>\n")
	b.WriteString("    <mxCell id=\"1\" parent=\"0\"/>\n")

	// Site containers and their device children
	for _, site := range sortedSites {
		sp := siteAbsPos[site]
		d := siteDims[site]
		siteID := "s_" + site
		emitVertex(&b, siteID, site,
			`swimlane;startSize=30;fillColor=#FAFAFA;strokeColor=#BDBDBD;strokeWidth=1;fontStyle=1;fontSize=12;`,
			"1", sp.X, sp.Y, d.w, d.h)

		for _, dev := range bySite[site] {
			dp := devRelPos[dev.Name]
			emitVertex(&b, "d_"+dev.Name, deviceLabel(dev), deviceCellStyle(dev), siteID, dp.X, dp.Y, deviceW, deviceH)
		}
	}

	// Abstract (collapsed) site nodes
	for _, name := range abstractNames {
		ap := absNodePos[name]
		emitVertex(&b, "a_"+name, name,
			`ellipse;shape=cloud;fillColor=#ECEFF1;strokeColor=#90A4AE;strokeWidth=1;`,
			"1", ap.X, ap.Y, abstractW, abstractH)
	}

	// Physical link edges (sorted for determinism)
	physLinks := append([]schema.PhysicalLink(nil), g.PhysicalLinks...)
	sort.Slice(physLinks, func(i, j int) bool {
		if physLinks[i].AEnd != physLinks[j].AEnd {
			return physLinks[i].AEnd < physLinks[j].AEnd
		}
		return physLinks[i].ZEnd < physLinks[j].ZEnd
	})
	for _, l := range physLinks {
		emitEdge(&b, "e_"+l.AEnd+"_"+l.ZEnd, physEdgeStyle(l.Role),
			nodeID(l.AEnd, abstract), nodeID(l.ZEnd, abstract))
	}

	// Logical link edges (sorted for determinism)
	logLinks := append([]schema.LogicalLink(nil), g.LogicalLinks...)
	sort.Slice(logLinks, func(i, j int) bool {
		if logLinks[i].AEnd != logLinks[j].AEnd {
			return logLinks[i].AEnd < logLinks[j].AEnd
		}
		return logLinks[i].ZEnd < logLinks[j].ZEnd
	})
	for _, l := range logLinks {
		emitEdge(&b, "le_"+l.AEnd+"_"+l.ZEnd, logEdgeStyle(l.Role),
			nodeID(l.AEnd, abstract), nodeID(l.ZEnd, abstract))
	}

	b.WriteString("  </root>\n")
	b.WriteString("</mxGraphModel>\n")
	return []byte(b.String()), nil
}

// XML parsing types — used by ExtractPositions only.

type xmlModel struct {
	XMLName xml.Name `xml:"mxGraphModel"`
	Root    xmlRoot  `xml:"root"`
}

type xmlRoot struct {
	Cells []xmlCell `xml:"mxCell"`
}

type xmlCell struct {
	ID       string      `xml:"id,attr"`
	Parent   string      `xml:"parent,attr"`
	Vertex   string      `xml:"vertex,attr"`
	Geometry xmlGeometry `xml:"mxGeometry"`
}

type xmlGeometry struct {
	X float64 `xml:"x,attr"`
	Y float64 `xml:"y,attr"`
}

// ExtractPositions parses a draw.io XML file and returns a map of node name →
// absolute page position. Only nodes generated by netdraw are recognised
// (cell IDs prefixed with "s_", "d_", "a_").
func ExtractPositions(data []byte) (map[string]layout.Position, error) {
	var model xmlModel
	if err := xml.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("parsing draw.io XML: %w", err)
	}

	siteAbsPos := make(map[string]layout.Position) // cell_id → absolute site position
	result := make(map[string]layout.Position)

	// First pass: top-level site containers and abstract nodes (parent="1")
	for _, cell := range model.Root.Cells {
		if cell.Vertex != "1" || cell.Parent != "1" {
			continue
		}
		switch {
		case strings.HasPrefix(cell.ID, "s_"):
			pos := layout.Position{X: cell.Geometry.X, Y: cell.Geometry.Y}
			siteAbsPos[cell.ID] = pos
			result[cell.ID[2:]] = pos
		case strings.HasPrefix(cell.ID, "a_"):
			result[cell.ID[2:]] = layout.Position{X: cell.Geometry.X, Y: cell.Geometry.Y}
		}
	}

	// Second pass: device cells nested inside site containers
	for _, cell := range model.Root.Cells {
		if cell.Vertex != "1" || !strings.HasPrefix(cell.ID, "d_") {
			continue
		}
		if sp, ok := siteAbsPos[cell.Parent]; ok {
			// Device geometry is relative to the site container body (below the header)
			result[cell.ID[2:]] = layout.Position{
				X: sp.X + cell.Geometry.X,
				Y: sp.Y + headerSize + cell.Geometry.Y,
			}
		}
	}

	return result, nil
}

// ── XML generation helpers ────────────────────────────────────────────────────

func emitVertex(b *strings.Builder, id, label, style, parent string, x, y, w, h float64) {
	fmt.Fprintf(b, "    <mxCell id=%q value=%q style=%q vertex=%q parent=%q>\n",
		id, attr(label), style, "1", parent)
	fmt.Fprintf(b, "      <mxGeometry x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" as=\"geometry\"/>\n",
		x, y, w, h)
	b.WriteString("    </mxCell>\n")
}

func emitEdge(b *strings.Builder, id, style, source, target string) {
	fmt.Fprintf(b, "    <mxCell id=%q style=%q edge=%q source=%q target=%q parent=%q>\n",
		id, style, "1", source, target, "1")
	b.WriteString("      <mxGeometry relative=\"1\" as=\"geometry\"/>\n")
	b.WriteString("    </mxCell>\n")
}

// attr escapes s for use as an XML attribute value.
// Handles &, <, >, " and newlines (→ &#xa; as draw.io expects).
func attr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "\n", "&#xa;")
	return s
}

// nodeID returns the draw.io cell ID for a given device or abstract site node.
func nodeID(name string, abstract map[string]bool) string {
	if abstract[name] {
		return "a_" + name
	}
	return "d_" + name
}

// ── Style helpers ─────────────────────────────────────────────────────────────

func deviceLabel(d schema.Device) string {
	if d.Status == schema.StatusConcept {
		label := d.Role
		if d.Count > 1 {
			label += fmt.Sprintf(" ×%d", d.Count)
		}
		return label
	}
	label := d.Name + "\n" + d.Role
	if d.Vendor != "" {
		label += " · " + d.Vendor
	}
	return label
}

func deviceCellStyle(d schema.Device) string {
	switch d.Status {
	case schema.StatusConcept:
		return "rounded=1;whiteSpace=wrap;fillColor=#ECEFF1;strokeColor=#90A4AE;strokeWidth=1;dashed=1;fontSize=10;fontColor=#78909C;verticalAlign=middle;"
	case schema.StatusPlanned:
		return fmt.Sprintf("shape=image;image=%s;aspect=fixed;verticalLabelPosition=bottom;align=center;verticalAlign=top;fontSize=10;fillColor=none;strokeColor=none;opacity=55;",
			icons.DrawioSVGURI(d.Role))
	default:
		return fmt.Sprintf("shape=image;image=%s;aspect=fixed;verticalLabelPosition=bottom;align=center;verticalAlign=top;fontSize=10;fillColor=none;strokeColor=none;",
			icons.DrawioSVGURI(d.Role))
	}
}

func physEdgeStyle(role string) string {
	stroke, width, dash := physLinkAttrs(role)
	s := fmt.Sprintf("edgeStyle=none;endArrow=none;strokeColor=%s;strokeWidth=%d;", stroke, width)
	if dash > 0 {
		s += fmt.Sprintf("dashed=1;dashPattern=%d %d;", dash, dash)
	}
	return s
}

func logEdgeStyle(role string) string {
	return fmt.Sprintf("edgeStyle=none;endArrow=none;strokeColor=%s;strokeWidth=1;dashed=1;dashPattern=5 5;", logLinkColor(role))
}

// physLinkAttrs mirrors the link styling from the D2 renderer for visual consistency.
func physLinkAttrs(role string) (stroke string, width, dash int) {
	switch role {
	case "fabric":
		return "#546E7A", 2, 0
	case "uplink":
		return "#455A64", 2, 0
	case "wan":
		return "#E65100", 3, 0
	case "oob":
		return "#BDBDBD", 1, 3
	case "mlag_peer":
		return "#1565C0", 2, 0
	default:
		return "#546E7A", 1, 0
	}
}

// logLinkColor mirrors the D2 renderer link colours.
func logLinkColor(role string) string {
	switch role {
	case "ibgp_peering":
		return "#5C6BC0"
	case "ebgp_peering":
		return "#2E7D32"
	case "ospf":
		return "#FF8F00"
	default:
		return "#757575"
	}
}

// ── Sort helpers ──────────────────────────────────────────────────────────────

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedBoolMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntMapKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
