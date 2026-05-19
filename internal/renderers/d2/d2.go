// Package d2 generates D2 text from a normalised graph and renders it to SVG
// via the D2 Go library (oss.terrastruct.com/d2). No shell-out required.
package d2

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sort"
	"strings"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2target"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/geo"
	"oss.terrastruct.com/d2/lib/textmeasure"

	"github.com/ppklau/netdraw/internal/graph"
	"github.com/ppklau/netdraw/internal/icons"
	"github.com/ppklau/netdraw/internal/layout"
	"github.com/ppklau/netdraw/internal/schema"
	"github.com/ppklau/netdraw/internal/views"
)

// iconSize controls the rendered width and height of device icon nodes (pixels).
const iconSize = 64

// ifaceFontSize controls the font size of interface endpoint labels (points).
const ifaceFontSize = 8

// Script generates deterministic D2 text from a filtered graph and view config.
// positions is the per-view layout override map (may be nil). Position injection
// into the D2 compiled graph is deferred to Phase 6; the parameter is accepted
// now so the call-site API is stable.
func Script(g *graph.Graph, v *views.View, _ map[string]layout.Position) string {
	var b strings.Builder

	// Classify devices: abstract site nodes vs concrete devices in site containers.
	abstract := make(map[string]bool)          // site names that are collapsed nodes
	bySite := make(map[string][]schema.Device) // site → concrete devices
	owner := make(map[string]string)           // device name → site ID

	for _, d := range g.Devices {
		if d.Role == views.AbstractSiteRole {
			abstract[d.Name] = true
		} else {
			bySite[d.Site] = append(bySite[d.Site], d)
			owner[d.Name] = d.Site
		}
	}

	// nodeRef returns the full D2 path for a device name.
	// Concrete: "site-id.device-id"; abstract: "site-id".
	nodeRef := func(name string) string {
		if abstract[name] {
			return safeID(name)
		}
		return safeID(owner[name]) + "." + safeID(name)
	}

	// ── Title block ──────────────────────────────────────────────────────────
	if v.Title.Text != "" {
		titleLabel := v.Title.Text
		if v.Title.Classification != "" {
			titleLabel += "\nClassification: " + v.Title.Classification
		}
		b.WriteString("_title: {\n")
		b.WriteString("  near: top-center\n")
		b.WriteString("  shape: text\n")
		fmt.Fprintf(&b, "  label: %q\n", titleLabel)
		b.WriteString("  style: {\n")
		b.WriteString("    font-size: 20\n")
		b.WriteString("    bold: true\n")
		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}

	// ── Site containers (sorted for determinism) ─────────────────────────────
	sortedSites := make([]string, 0, len(bySite))
	for site := range bySite {
		sortedSites = append(sortedSites, site)
	}
	sort.Strings(sortedSites)

	for _, siteName := range sortedSites {
		devs := bySite[siteName]
		sort.Slice(devs, func(i, j int) bool { return devs[i].Name < devs[j].Name })

		fmt.Fprintf(&b, "%s: {\n", safeID(siteName))
		fmt.Fprintf(&b, "  label: %q\n", siteName)
		b.WriteString("  style: {\n")
		b.WriteString("    fill: \"#FAFAFA\"\n")
		b.WriteString("    stroke: \"#BDBDBD\"\n")
		b.WriteString("    stroke-width: 1\n")
		b.WriteString("  }\n")

		for _, d := range devs {
			fmt.Fprintf(&b, "\n  %s: {\n", safeID(d.Name))
			writeDeviceNode(&b, d, "  ")
			b.WriteString("  }\n")
		}

		b.WriteString("}\n\n")
	}

	// ── Abstract site nodes (collapsed sites) ────────────────────────────────
	abstractNames := make([]string, 0, len(abstract))
	for name := range abstract {
		abstractNames = append(abstractNames, name)
	}
	sort.Strings(abstractNames)

	for _, name := range abstractNames {
		fmt.Fprintf(&b, "%s: {\n", safeID(name))
		fmt.Fprintf(&b, "  label: %q\n", name)
		b.WriteString("  shape: cloud\n")
		b.WriteString("  style: {\n")
		b.WriteString("    fill: \"#ECEFF1\"\n")
		b.WriteString("    stroke: \"#90A4AE\"\n")
		b.WriteString("    stroke-width: 1\n")
		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}

	// ── Physical links (sorted for determinism) ──────────────────────────────
	physLinks := make([]schema.PhysicalLink, len(g.PhysicalLinks))
	copy(physLinks, g.PhysicalLinks)
	sort.Slice(physLinks, func(i, j int) bool {
		if physLinks[i].AEnd != physLinks[j].AEnd {
			return physLinks[i].AEnd < physLinks[j].AEnd
		}
		return physLinks[i].ZEnd < physLinks[j].ZEnd
	})

	// showIfaces: endpoint labels (interface IDs) at L2/L3.
	// showLinkRole: center role label for logical links at L1 (physical link role is conveyed by color/style).
	showIfaces := v.DetailLevel == views.L2 || v.DetailLevel == views.L3
	showLinkRole := v.DetailLevel == views.L1

	for _, l := range physLinks {
		aRef := nodeRef(l.AEnd)
		zRef := nodeRef(l.ZEnd)
		stroke, width, dash := physLinkStyle(l.Role)
		// Status overrides: concept → dotted grey; planned → dashed muted grey
		if l.Status == schema.StatusConcept {
			stroke, width, dash = "#B0BEC5", 1, 3
		} else if l.Status == schema.StatusPlanned {
			stroke, width, dash = "#90A4AE", 1, 5
		}

		fmt.Fprintf(&b, "%s -- %s {\n", aRef, zRef)
		if showIfaces {
			if l.AInterface != "" {
				fmt.Fprintf(&b, "  source-arrowhead.label: %q\n", l.AInterface)
			}
			if l.ZInterface != "" {
				fmt.Fprintf(&b, "  target-arrowhead.label: %q\n", l.ZInterface)
			}
		}
		b.WriteString("  style: {\n")
		if showIfaces {
			fmt.Fprintf(&b, "    font-size: %d\n", ifaceFontSize)
		}
		fmt.Fprintf(&b, "    stroke: %q\n", stroke)
		fmt.Fprintf(&b, "    stroke-width: %d\n", width)
		if dash > 0 {
			fmt.Fprintf(&b, "    stroke-dash: %d\n", dash)
		}
		b.WriteString("  }\n")
		b.WriteString("}\n")
	}

	if len(physLinks) > 0 {
		b.WriteString("\n")
	}

	// ── Logical links (sorted for determinism) ───────────────────────────────
	logLinks := make([]schema.LogicalLink, len(g.LogicalLinks))
	copy(logLinks, g.LogicalLinks)
	sort.Slice(logLinks, func(i, j int) bool {
		if logLinks[i].AEnd != logLinks[j].AEnd {
			return logLinks[i].AEnd < logLinks[j].AEnd
		}
		return logLinks[i].ZEnd < logLinks[j].ZEnd
	})

	for _, l := range logLinks {
		aRef := nodeRef(l.AEnd)
		zRef := nodeRef(l.ZEnd)
		stroke := logLinkStroke(l.Role)
		linkDash := 5
		if l.Status == schema.StatusConcept {
			stroke, linkDash = "#B0BEC5", 3
		} else if l.Status == schema.StatusPlanned {
			stroke = "#90A4AE"
		}

		fmt.Fprintf(&b, "%s -- %s {\n", aRef, zRef)
		if showLinkRole && l.Role != "" {
			fmt.Fprintf(&b, "  label: %q\n", l.Role)
		}
		b.WriteString("  style: {\n")
		fmt.Fprintf(&b, "    stroke: %q\n", stroke)
		b.WriteString("    stroke-width: 1\n")
		fmt.Fprintf(&b, "    stroke-dash: %d\n", linkDash)
		b.WriteString("  }\n")
		b.WriteString("}\n")
	}

	// ── Legend (near: bottom-right) ──────────────────────────────────────────
	if v.Legend {
		legendLabel := "Legend" +
			"\n─────  physical" +
			"\n- - -  logical (dashed)" +
			"\n·····  oob (dotted)"
		b.WriteString("\n_legend: {\n")
		b.WriteString("  near: bottom-right\n")
		b.WriteString("  shape: rectangle\n")
		fmt.Fprintf(&b, "  label: %q\n", legendLabel)
		b.WriteString("  style: {\n")
		b.WriteString("    fill: \"#FAFAFA\"\n")
		b.WriteString("    stroke: \"#BDBDBD\"\n")
		b.WriteString("    font-size: 12\n")
		b.WriteString("  }\n")
		b.WriteString("}\n")
	}

	return b.String()
}

// straightenEdges recomputes every connection's route as a direct two-point line
// between the facing borders of the source and destination shapes, then clears
// IsCurve so the SVG renderer emits straight SVG paths instead of bezier curves.
//
// Dagre's bezier routing exits nodes from whichever side suits the curve, which
// can be the "wrong" side for a straight line (e.g. a node exits rightward even
// though the target is to the left). Recomputing from shape centers fixes this.
func straightenEdges(diagram *d2target.Diagram) {
	shapeMap := make(map[string]d2target.Shape, len(diagram.Shapes))
	for _, s := range diagram.Shapes {
		shapeMap[s.ID] = s
	}

	for i := range diagram.Connections {
		c := &diagram.Connections[i]
		c.IsCurve = false

		src, srcOK := shapeMap[c.Src]
		dst, dstOK := shapeMap[c.Dst]
		if !srcOK || !dstOK {
			if len(c.Route) > 2 {
				c.Route = []*geo.Point{c.Route[0], c.Route[len(c.Route)-1]}
			}
			continue
		}

		srcCX := float64(src.Pos.X) + float64(src.Width)/2
		srcCY := float64(src.Pos.Y) + float64(src.Height)/2
		dstCX := float64(dst.Pos.X) + float64(dst.Width)/2
		dstCY := float64(dst.Pos.Y) + float64(dst.Height)/2

		p0 := shapeEdgePoint(srcCX, srcCY, dstCX, dstCY, float64(src.Width), float64(src.Height))
		p1 := shapeEdgePoint(dstCX, dstCY, srcCX, srcCY, float64(dst.Width), float64(dst.Height))
		c.Route = []*geo.Point{p0, p1}
	}
}

// shapeEdgePoint returns the point on the axis-aligned bounding box of a shape
// (centered at cx,cy with dimensions w×h) where the ray toward (tx,ty) exits.
func shapeEdgePoint(cx, cy, tx, ty, w, h float64) *geo.Point {
	dx, dy := tx-cx, ty-cy
	if dx == 0 && dy == 0 {
		return &geo.Point{X: cx, Y: cy}
	}
	hw, hh := w/2, h/2
	var t float64 = math.MaxFloat64
	if dx > 0 {
		t = math.Min(t, hw/dx)
	} else if dx < 0 {
		t = math.Min(t, -hw/dx)
	}
	if dy > 0 {
		t = math.Min(t, hh/dy)
	} else if dy < 0 {
		t = math.Min(t, -hh/dy)
	}
	return &geo.Point{X: cx + dx*t, Y: cy + dy*t}
}

// dagreLayout returns a dagre LayoutGraph with increased node separation so that
// long device labels don't overlap between adjacent horizontally-placed nodes.
func dagreLayout() d2graph.LayoutGraph {
	return func(ctx context.Context, g *d2graph.Graph) error {
		opts := d2dagrelayout.DefaultOpts
		opts.NodeSep = 120
		return d2dagrelayout.Layout(ctx, g, &opts)
	}
}

// RenderSVG compiles a D2 script and produces SVG bytes via the D2 Go library.
// Uses the dagre layout engine — no external binary required.
func RenderSVG(ctx context.Context, script string) ([]byte, error) {
	// Suppress D2's internal debug logging; netdraw controls its own output.
	ctx = d2log.With(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("initializing text ruler: %w", err)
	}

	pad := int64(d2svg.DEFAULT_PADDING)
	themeID := int64(0) // NeutralDefault

	renderOpts := &d2svg.RenderOpts{
		Pad:     &pad,
		ThemeID: &themeID,
	}

	layoutEngine := "dagre"
	diagram, _, err := d2lib.Compile(ctx, script, &d2lib.CompileOptions{
		Ruler:  ruler,
		Layout: &layoutEngine,
		LayoutResolver: func(engine string) (d2graph.LayoutGraph, error) {
			if engine == "dagre" {
				return dagreLayout(), nil
			}
			return nil, fmt.Errorf("unsupported layout engine %q", engine)
		},
	}, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("compiling D2 script: %w", err)
	}

	straightenEdges(diagram)

	svg, err := d2svg.Render(diagram, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("rendering SVG: %w", err)
	}

	return svg, nil
}

// writeDeviceNode writes the D2 properties for a single device node.
// indent is the whitespace prefix for each line (e.g. "  " when inside a container).
// Visual treatment varies by status:
//   - concept: dashed-border rectangle, role label only, count annotation if >1
//   - planned: icon with reduced opacity, full label
//   - confirmed/active: full icon, full label
func writeDeviceNode(b *strings.Builder, d schema.Device, indent string) {
	switch d.Status {
	case schema.StatusConcept:
		label := d.Role
		if d.Count > 1 {
			label += fmt.Sprintf(" ×%d", d.Count)
		}
		fmt.Fprintf(b, "%slabel: %q\n", indent, label)
		b.WriteString(indent + "shape: rectangle\n")
		b.WriteString(indent + "style: {\n")
		b.WriteString(indent + "  fill: \"#ECEFF1\"\n")
		b.WriteString(indent + "  stroke: \"#90A4AE\"\n")
		b.WriteString(indent + "  stroke-width: 1\n")
		b.WriteString(indent + "  stroke-dash: 4\n")
		b.WriteString(indent + "  font-color: \"#78909C\"\n")
		b.WriteString(indent + "}\n")

	case schema.StatusPlanned:
		label := d.Name + "\n" + d.Role
		if d.Vendor != "" {
			label += " · " + d.Vendor
		}
		fmt.Fprintf(b, "%slabel: %q\n", indent, label)
		b.WriteString(indent + "shape: image\n")
		fmt.Fprintf(b, "%sicon: %q\n", indent, icons.DataURI(d.Role))
		fmt.Fprintf(b, "%swidth: %d\n", indent, iconSize)
		fmt.Fprintf(b, "%sheight: %d\n", indent, iconSize)
		b.WriteString(indent + "style: {\n")
		b.WriteString(indent + "  opacity: 0.55\n")
		b.WriteString(indent + "}\n")

	default: // confirmed, active, or unset
		label := d.Name + "\n" + d.Role
		if d.Vendor != "" {
			label += " · " + d.Vendor
		}
		fmt.Fprintf(b, "%slabel: %q\n", indent, label)
		b.WriteString(indent + "shape: image\n")
		fmt.Fprintf(b, "%sicon: %q\n", indent, icons.DataURI(d.Role))
		fmt.Fprintf(b, "%swidth: %d\n", indent, iconSize)
		fmt.Fprintf(b, "%sheight: %d\n", indent, iconSize)
	}
}

// PositionsFromLayout runs D2's dagre layout engine on the graph and returns
// top-level container positions (site containers and abstract nodes) scaled to
// draw.io page coordinates. Returns nil, nil if no positionable nodes are found.
// Only top-level objects are returned; per-device positions within site containers
// are handled by the draw.io renderer itself.
func PositionsFromLayout(ctx context.Context, g *graph.Graph, v *views.View, pageW, pageH float64) (map[string]layout.Position, error) {
	ctx = d2log.With(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))

	script := Script(g, v, nil)

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("initializing text ruler: %w", err)
	}

	pad := int64(d2svg.DEFAULT_PADDING)
	themeID := int64(0)
	renderOpts := &d2svg.RenderOpts{Pad: &pad, ThemeID: &themeID}
	engine := "dagre"

	_, dg, err := d2lib.Compile(ctx, script, &d2lib.CompileOptions{
		Ruler:  ruler,
		Layout: &engine,
		LayoutResolver: func(e string) (d2graph.LayoutGraph, error) {
			if e == "dagre" {
				return dagreLayout(), nil
			}
			return nil, fmt.Errorf("unsupported layout engine %q", e)
		},
	}, renderOpts)
	if err != nil {
		return nil, fmt.Errorf("D2 layout: %w", err)
	}

	// Build reverse lookup: D2 safe ID → original device name.
	// safeID only replaces '.' with '_', so names without dots are unchanged.
	safeToDevice := make(map[string]string)
	for _, d := range g.Devices {
		if d.Role != views.AbstractSiteRole {
			safeToDevice[safeID(d.Name)] = d.Name
		}
	}

	type rawPos struct{ x, y, w, h float64 }
	raw := make(map[string]rawPos)

	for _, obj := range dg.Root.ChildrenArray {
		if strings.HasPrefix(obj.ID, "_") || obj.Box == nil || obj.TopLeft == nil {
			continue
		}
		raw[obj.ID] = rawPos{x: obj.TopLeft.X, y: obj.TopLeft.Y, w: obj.Width, h: obj.Height}
	}

	if len(raw) == 0 {
		return nil, nil
	}

	var minX, minY, maxX, maxY float64
	first := true
	for _, p := range raw {
		if first {
			minX, minY = p.x, p.y
			maxX, maxY = p.x+p.w, p.y+p.h
			first = false
		} else {
			if p.x < minX {
				minX = p.x
			}
			if p.y < minY {
				minY = p.y
			}
			if p.x+p.w > maxX {
				maxX = p.x + p.w
			}
			if p.y+p.h > maxY {
				maxY = p.y + p.h
			}
		}
	}

	spanX, spanY := maxX-minX, maxY-minY
	if spanX < 1 {
		spanX = 1
	}
	if spanY < 1 {
		spanY = 1
	}

	margin := 40.0
	scale := math.Min((pageW-2*margin)/spanX, (pageH-2*margin)/spanY)
	if scale > 1.0 {
		scale = 1.0
	}

	result := make(map[string]layout.Position, len(raw))
	for name, p := range raw {
		result[name] = layout.Position{
			X: margin + (p.x-minX)*scale,
			Y: margin + (p.y-minY)*scale,
		}
	}

	// Extract per-device positions from within each site container.
	// D2 TopLeft coordinates are absolute in SVG space; apply the same scale
	// so the drawio renderer's existing position-override path places devices
	// at dagre's computed positions rather than the fallback tier-row grid.
	for _, obj := range dg.Root.ChildrenArray {
		if strings.HasPrefix(obj.ID, "_") || obj.Box == nil || obj.TopLeft == nil {
			continue
		}
		for _, child := range obj.ChildrenArray {
			if child.Box == nil || child.TopLeft == nil {
				continue
			}
			origName, ok := safeToDevice[child.ID]
			if !ok {
				continue
			}
			result[origName] = layout.Position{
				X: margin + (child.TopLeft.X-minX)*scale,
				Y: margin + (child.TopLeft.Y-minY)*scale,
			}
		}
	}

	return result, nil
}

// safeID sanitises a device or site name to a valid D2 identifier.
// Dots are replaced since D2 uses '.' as the hierarchy separator.
func safeID(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

// physLinkStyle returns D2 style values for a physical link role.
// Returns: stroke colour, stroke-width, stroke-dash (0 = solid).
func physLinkStyle(role string) (stroke string, width, dash int) {
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

// logLinkStroke returns the D2 stroke colour for a logical link role.
func logLinkStroke(role string) string {
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
