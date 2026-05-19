package d2_test

import (
	"strings"
	"testing"

	"github.com/ppklau/netdraw/internal/graph"
	d2r "github.com/ppklau/netdraw/internal/renderers/d2"
	"github.com/ppklau/netdraw/internal/schema"
	"github.com/ppklau/netdraw/internal/views"
)

func TestScript_deterministic(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "lon-dc1-spine-01", Role: "spine", Vendor: "arista", Site: "lon-dc1", Status: "active"},
			{Name: "lon-dc1-leaf-01", Role: "leaf", Vendor: "arista", Site: "lon-dc1", Status: "active"},
		},
		Sites: []schema.Site{{Name: "lon-dc1"}},
		PhysicalLinks: []schema.PhysicalLink{
			{AEnd: "lon-dc1-spine-01", ZEnd: "lon-dc1-leaf-01", AInterface: "Ethernet1", ZInterface: "Ethernet1", Kind: "physical", Role: "fabric", Status: "active"},
		},
	}
	v := &views.View{Name: "test", Title: views.Title{Text: "Test View"}}

	s1 := d2r.Script(g, v, nil)
	s2 := d2r.Script(g, v, nil)

	if s1 != s2 {
		t.Fatal("Script() is not deterministic")
	}
}

func TestScript_siteContainer(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "lon-dc1-spine-01", Role: "spine", Site: "lon-dc1", Status: "active"},
		},
		Sites: []schema.Site{{Name: "lon-dc1"}},
	}
	v := &views.View{Name: "test"}

	script := d2r.Script(g, v, nil)

	if !strings.Contains(script, "lon-dc1: {") {
		t.Error("expected site container block for lon-dc1")
	}
	if !strings.Contains(script, "lon-dc1-spine-01: {") {
		t.Error("expected device node lon-dc1-spine-01 inside container")
	}
}

func TestScript_abstractSiteNode(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "lon-dc1-spine-01", Role: "spine", Site: "lon-dc1", Status: "active"},
			{Name: "nyc-dc1", Role: views.AbstractSiteRole, Site: "nyc-dc1"}, // collapsed
		},
		Sites:         []schema.Site{{Name: "lon-dc1"}, {Name: "nyc-dc1"}},
		PhysicalLinks: []schema.PhysicalLink{{AEnd: "lon-dc1-spine-01", ZEnd: "nyc-dc1", Kind: "physical", Role: "wan", Status: "active"}},
	}
	v := &views.View{Name: "test"}

	script := d2r.Script(g, v, nil)

	if !strings.Contains(script, "shape: cloud") {
		t.Error("expected cloud shape for abstract site node")
	}
	if !strings.Contains(script, "lon-dc1.lon-dc1-spine-01 -- nyc-dc1") {
		t.Error("expected inter-site link referencing abstract site node directly")
	}
}

func TestScript_linkConventions(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "spine-01", Role: "spine", Site: "lon-dc1", Status: "active"},
			{Name: "leaf-01", Role: "leaf", Site: "lon-dc1", Status: "active"},
			{Name: "spine-02", Role: "spine", Site: "nyc-dc1", Status: "active"},
		},
		Sites: []schema.Site{{Name: "lon-dc1"}, {Name: "nyc-dc1"}},
		PhysicalLinks: []schema.PhysicalLink{
			{AEnd: "spine-01", ZEnd: "leaf-01", AInterface: "Ethernet1", ZInterface: "Ethernet2", Kind: "physical", Role: "fabric", Status: "active"},
		},
		LogicalLinks: []schema.LogicalLink{
			{AEnd: "spine-01", ZEnd: "spine-02", Kind: "logical", Role: "ibgp_peering", Status: "active"},
		},
	}

	t.Run("no directed arrows on any view", func(t *testing.T) {
		script := d2r.Script(g, &views.View{Name: "test"}, nil)
		if strings.Contains(script, " -> ") {
			t.Error("links must use -- (undirected), not ->")
		}
	})

	t.Run("interface labels suppressed at L0", func(t *testing.T) {
		script := d2r.Script(g, &views.View{Name: "test", DetailLevel: views.L0}, nil)
		if strings.Contains(script, "source-arrowhead.label") || strings.Contains(script, "target-arrowhead.label") {
			t.Error("interface labels must not appear at L0")
		}
	})

	t.Run("interface labels shown at L2", func(t *testing.T) {
		script := d2r.Script(g, &views.View{Name: "test", DetailLevel: views.L2}, nil)
		if strings.Contains(script, `"Ethernet1/Ethernet2"`) {
			t.Error("interface IDs must not be combined as a single centre label")
		}
		if !strings.Contains(script, `source-arrowhead.label: "Ethernet1"`) {
			t.Error("expected source-arrowhead.label for a_interface at L2")
		}
		if !strings.Contains(script, `target-arrowhead.label: "Ethernet2"`) {
			t.Error("expected target-arrowhead.label for z_interface at L2")
		}
	})

	t.Run("logical links are dashed", func(t *testing.T) {
		script := d2r.Script(g, &views.View{Name: "test"}, nil)
		if !strings.Contains(script, "stroke-dash: 5") {
			t.Error("expected stroke-dash on logical links")
		}
	})
}

func TestScript_L1_roleLabelOnLinks(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "spine-01", Role: "spine", Site: "lon-dc1", Status: "active"},
			{Name: "fw-01", Role: "firewall", Site: "nyc-dc1", Status: "active"},
		},
		Sites: []schema.Site{{Name: "lon-dc1"}, {Name: "nyc-dc1"}},
		PhysicalLinks: []schema.PhysicalLink{
			{AEnd: "spine-01", ZEnd: "fw-01", AInterface: "Ethernet1", ZInterface: "Ethernet2", Kind: "physical", Role: "wan", Status: "active"},
		},
		LogicalLinks: []schema.LogicalLink{
			{AEnd: "spine-01", ZEnd: "fw-01", Kind: "logical", Role: "ibgp_peering", Status: "active"},
		},
	}
	v := &views.View{Name: "test", DetailLevel: views.L1}
	script := d2r.Script(g, v, nil)

	if !strings.Contains(script, `label: "wan"`) {
		t.Error("expected center role label for physical link at L1")
	}
	if !strings.Contains(script, `label: "ibgp_peering"`) {
		t.Error("expected center role label for logical link at L1")
	}
	if strings.Contains(script, "source-arrowhead.label") || strings.Contains(script, "target-arrowhead.label") {
		t.Error("interface endpoint labels must not appear at L1")
	}
}

func TestScript_L0_noLabels(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "spine-01", Role: "spine", Site: "lon-dc1", Status: "active"},
			{Name: "fw-01", Role: "firewall", Site: "nyc-dc1", Status: "active"},
		},
		Sites: []schema.Site{{Name: "lon-dc1"}, {Name: "nyc-dc1"}},
		PhysicalLinks: []schema.PhysicalLink{
			{AEnd: "spine-01", ZEnd: "fw-01", AInterface: "Ethernet1", ZInterface: "Ethernet2", Kind: "physical", Role: "wan", Status: "active"},
		},
	}
	v := &views.View{Name: "test", DetailLevel: views.L0}
	script := d2r.Script(g, v, nil)

	if strings.Contains(script, "source-arrowhead.label") || strings.Contains(script, "target-arrowhead.label") {
		t.Error("interface endpoint labels must not appear at L0")
	}
	// Role labels are L1-specific; L0 should show no link labels.
	if strings.Contains(script, `label: "wan"`) {
		t.Error("role label must not appear at L0")
	}
}

func TestScript_conceptStatus(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "london-dc-spine", Role: "spine", Site: "london-dc", Status: schema.StatusConcept, Count: 2},
			{Name: "london-dc-leaf", Role: "leaf", Site: "london-dc", Status: schema.StatusConcept, Count: 4},
		},
		Sites: []schema.Site{{Name: "london-dc"}},
		PhysicalLinks: []schema.PhysicalLink{
			{AEnd: "london-dc-spine", ZEnd: "london-dc-leaf", Kind: "physical", Role: "fabric", Status: schema.StatusConcept},
		},
	}
	v := &views.View{Name: "test", DetailLevel: views.L1}
	script := d2r.Script(g, v, nil)

	if strings.Contains(script, "shape: image") {
		t.Error("concept devices must not use icon shape")
	}
	if !strings.Contains(script, "shape: rectangle") {
		t.Error("expected rectangle shape for concept devices")
	}
	if !strings.Contains(script, "stroke-dash: 4") {
		t.Error("expected dashed border on concept device nodes")
	}
	if !strings.Contains(script, "×2") {
		t.Error("expected count annotation ×2 in concept device label")
	}
	if !strings.Contains(script, "×4") {
		t.Error("expected count annotation ×4 in concept device label")
	}
	// Concept links: light grey stroke-dash:3
	if !strings.Contains(script, `stroke: "#B0BEC5"`) {
		t.Error("expected light grey stroke for concept physical link")
	}
	if !strings.Contains(script, "stroke-dash: 3") {
		t.Error("expected stroke-dash:3 for concept physical link")
	}
	// No directed arrows
	if strings.Contains(script, " -> ") {
		t.Error("links must use -- (undirected)")
	}
}

func TestScript_plannedStatus(t *testing.T) {
	g := &graph.Graph{
		Devices: []schema.Device{
			{Name: "sng-dc1-leaf-03", Role: "leaf", Site: "sng-dc1", Status: schema.StatusPlanned},
		},
		Sites: []schema.Site{{Name: "sng-dc1"}},
		PhysicalLinks: []schema.PhysicalLink{
			{AEnd: "sng-dc1-leaf-03", ZEnd: "sng-dc1-leaf-03", Kind: "physical", Role: "fabric", Status: schema.StatusPlanned},
		},
	}
	v := &views.View{Name: "test"}
	script := d2r.Script(g, v, nil)

	if !strings.Contains(script, "shape: image") {
		t.Error("planned devices should still use icon shape")
	}
	if !strings.Contains(script, "opacity: 0.55") {
		t.Error("expected reduced opacity for planned device nodes")
	}
	// Planned physical links: muted grey dash-5
	if !strings.Contains(script, `stroke: "#90A4AE"`) {
		t.Error("expected muted grey stroke for planned physical link")
	}
	if !strings.Contains(script, "stroke-dash: 5") {
		t.Error("expected stroke-dash:5 for planned physical link")
	}
}

func TestScript_titleAndLegend(t *testing.T) {
	g := &graph.Graph{}
	v := &views.View{
		Name:   "test",
		Legend: true,
		Title:  views.Title{Text: "My Network", Classification: "Internal"},
	}

	script := d2r.Script(g, v, nil)

	if !strings.Contains(script, "_title:") {
		t.Error("expected _title block")
	}
	if !strings.Contains(script, "near: top-center") {
		t.Error("expected near: top-center on title")
	}
	if !strings.Contains(script, "_legend:") {
		t.Error("expected _legend block")
	}
	if !strings.Contains(script, "near: bottom-right") {
		t.Error("expected near: bottom-right on legend")
	}
}
