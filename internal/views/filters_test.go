package views

import (
	"testing"

	"github.com/ppklau/netdraw/internal/graph"
	"github.com/ppklau/netdraw/internal/schema"
)

func acmeGraph() *graph.Graph {
	devices := []schema.Device{
		{Name: "lon-dc1-spine-01", Role: "spine", Site: "lon-dc1", Status: schema.StatusActive},
		{Name: "lon-dc1-spine-02", Role: "spine", Site: "lon-dc1", Status: schema.StatusActive},
		{Name: "lon-dc1-leaf-01", Role: "leaf", Site: "lon-dc1", Status: schema.StatusActive},
		{Name: "lon-dc1-leaf-02", Role: "leaf", Site: "lon-dc1", Status: schema.StatusActive},
		{Name: "lon-dc1-border-01", Role: "border_router", Site: "lon-dc1", Status: schema.StatusActive},
		{Name: "nyc-dc1-spine-01", Role: "spine", Site: "nyc-dc1", Status: schema.StatusActive},
		{Name: "nyc-dc1-spine-02", Role: "spine", Site: "nyc-dc1", Status: schema.StatusActive},
		{Name: "nyc-dc1-leaf-01", Role: "leaf", Site: "nyc-dc1", Status: schema.StatusActive},
		{Name: "nyc-dc1-fw-01", Role: "firewall", Site: "nyc-dc1", Status: schema.StatusActive},
	}
	sites := []schema.Site{
		{Name: "lon-dc1", Region: "emea"},
		{Name: "nyc-dc1", Region: "amer"},
	}
	regions := []schema.Region{
		{Name: "emea"},
		{Name: "amer"},
	}
	physicalLinks := []schema.PhysicalLink{
		{AEnd: "lon-dc1-spine-01", ZEnd: "lon-dc1-leaf-01", Kind: "physical", Role: "fabric", Speed: "100G"},
		{AEnd: "lon-dc1-spine-01", ZEnd: "lon-dc1-leaf-02", Kind: "physical", Role: "fabric", Speed: "100G"},
		{AEnd: "lon-dc1-spine-02", ZEnd: "lon-dc1-leaf-01", Kind: "physical", Role: "fabric", Speed: "100G"},
		{AEnd: "lon-dc1-spine-02", ZEnd: "lon-dc1-leaf-02", Kind: "physical", Role: "fabric", Speed: "100G"},
		{AEnd: "nyc-dc1-spine-01", ZEnd: "nyc-dc1-leaf-01", Kind: "physical", Role: "fabric", Speed: "100G"},
		{AEnd: "nyc-dc1-spine-02", ZEnd: "nyc-dc1-leaf-01", Kind: "physical", Role: "fabric", Speed: "100G"},
		{AEnd: "nyc-dc1-spine-01", ZEnd: "nyc-dc1-fw-01", Kind: "physical", Role: "uplink", Speed: "10G"},
		{AEnd: "nyc-dc1-spine-02", ZEnd: "nyc-dc1-fw-01", Kind: "physical", Role: "uplink", Speed: "10G"},
		{AEnd: "lon-dc1-spine-01", ZEnd: "lon-dc1-border-01", Kind: "physical", Role: "uplink", Speed: "10G"},
		{AEnd: "lon-dc1-spine-02", ZEnd: "lon-dc1-border-01", Kind: "physical", Role: "uplink", Speed: "10G"},
		{AEnd: "lon-dc1-border-01", ZEnd: "nyc-dc1-fw-01", Kind: "physical", Role: "wan", Speed: "10G"},
		{AEnd: "lon-dc1-spine-01", ZEnd: "lon-dc1-spine-02", Kind: "physical", Role: "oob", Speed: "1G"},
	}
	logicalLinks := []schema.LogicalLink{
		{AEnd: "lon-dc1-spine-01", ZEnd: "nyc-dc1-spine-01", Kind: "logical", Role: "ibgp_peering", Protocol: "bgp"},
		{AEnd: "lon-dc1-spine-02", ZEnd: "nyc-dc1-spine-02", Kind: "logical", Role: "ibgp_peering", Protocol: "bgp"},
	}
	return graph.Build(devices, sites, regions, physicalLinks, logicalLinks)
}

func TestFilter(t *testing.T) {
	g := acmeGraph()

	tests := []struct {
		name         string
		view         View
		wantDevices  int
		wantPhys     int
		wantLogical  int
	}{
		{
			name: "wan-overview: sites filter + exclude oob",
			view: View{
				Scope: Scope{
					Sites:        []string{"lon-dc1", "nyc-dc1"},
					ExcludeLinks: []string{"role=oob"},
				},
			},
			wantDevices: 9,
			wantPhys:    11,
			wantLogical: 2,
		},
		{
			name: "lon-dc1-fabric: focus_site + kinds=[physical] + roles=[fabric]",
			view: View{
				ConnectionKinds: []string{"physical"},
				ConnectionRoles: []string{"fabric"},
				Scope: Scope{
					FocusSite: "lon-dc1",
				},
			},
			wantDevices: 4,
			wantPhys:    4,
			wantLogical: 0,
		},
		{
			name: "focus_site=lon-dc1, kinds=[physical], no role filter",
			view: View{
				ConnectionKinds: []string{"physical"},
				Scope: Scope{
					FocusSite: "lon-dc1",
				},
			},
			wantDevices: 6,
			wantPhys:    8,
			wantLogical: 0,
		},
		{
			name: "collapse_sites=[nyc-*], no other filters",
			view: View{
				Scope: Scope{
					CollapseSites: []string{"nyc-*"},
				},
			},
			wantDevices: 6,
			wantPhys:    8,
			wantLogical: 2,
		},
		{
			name: "roles=[spine] + sites=[lon-dc1, nyc-dc1]",
			view: View{
				Scope: Scope{
					Sites: []string{"lon-dc1", "nyc-dc1"},
					Roles: []string{"spine"},
				},
			},
			wantDevices: 4,
			wantPhys:    1,
			wantLogical: 2,
		},
		{
			name: "exclude_devices=[role=firewall] + sites=[lon-dc1, nyc-dc1]",
			view: View{
				Scope: Scope{
					Sites:          []string{"lon-dc1", "nyc-dc1"},
					ExcludeDevices: []string{"role=firewall"},
				},
			},
			wantDevices: 8,
			wantPhys:    9,
			wantLogical: 2,
		},
		{
			name: "L0 auto-collapses all sites regardless of scope.sites",
			view: View{
				DetailLevel: L0,
				Scope: Scope{
					Sites: []string{"lon-dc1", "nyc-dc1"},
				},
			},
			// Both sites become abstract nodes; intra-site links drop, only the
			// wan link survives inter-site; two ibgp links deduplicate to one.
			wantDevices: 2,
			wantPhys:    1,
			wantLogical: 1,
		},
		{
			name: "L0 auto-collapses focus_site too",
			view: View{
				DetailLevel: L0,
				Scope: Scope{
					FocusSite: "lon-dc1",
				},
			},
			wantDevices: 2,
			wantPhys:    1,
			wantLogical: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filter(g, &tt.view)
			if got := len(result.Devices); got != tt.wantDevices {
				t.Errorf("devices: got %d, want %d", got, tt.wantDevices)
			}
			if got := len(result.PhysicalLinks); got != tt.wantPhys {
				t.Errorf("physical links: got %d, want %d", got, tt.wantPhys)
			}
			if got := len(result.LogicalLinks); got != tt.wantLogical {
				t.Errorf("logical links: got %d, want %d", got, tt.wantLogical)
			}
		})
	}
}
