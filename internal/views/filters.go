package views

import (
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/ppklau/netdraw/internal/graph"
	"github.com/ppklau/netdraw/internal/schema"
)

const AbstractSiteRole = "abstract_site"

// Filter returns a new graph.Graph scoped and filtered according to v.
func Filter(g *graph.Graph, v *View) *graph.Graph {
	full, collapsed := resolveSiteSets(g, v)

	// Build device lookup: name → device (concrete only, from full-detail sites).
	deviceBySite := make(map[string][]schema.Device)
	for _, d := range g.Devices {
		deviceBySite[d.Site] = append(deviceBySite[d.Site], d)
	}

	// Concrete devices: in full-detail sites, passing roles and exclude_devices filters.
	nameFor := make(map[string]string) // original device name → resolved name in filtered graph
	concreteDevices := []schema.Device{}

	for _, d := range g.Devices {
		if !full[d.Site] {
			continue
		}
		if len(v.Scope.Roles) > 0 && !slices.Contains(v.Scope.Roles, d.Role) {
			continue
		}
		if matchesAnyExpr(v.Scope.ExcludeDevices, d) {
			continue
		}
		concreteDevices = append(concreteDevices, d)
		nameFor[d.Name] = d.Name
	}

	// For collapsed sites, each device in that site maps to the site name (abstract node).
	for _, d := range g.Devices {
		if collapsed[d.Site] {
			nameFor[d.Name] = d.Site
		}
	}

	// Filter and rewrite links.
	type dedupKey struct {
		a, z, kind, role string
	}
	seen := make(map[dedupKey]bool)

	filteredPhys := []schema.PhysicalLink{}
	for _, l := range g.PhysicalLinks {
		if len(v.ConnectionKinds) > 0 && !slices.Contains(v.ConnectionKinds, l.Kind) {
			continue
		}
		if len(v.ConnectionRoles) > 0 && !slices.Contains(v.ConnectionRoles, l.Role) {
			continue
		}
		if matchesAnyExprLink(v.Scope.ExcludeLinks, l.Kind, l.Role) {
			continue
		}
		aName := nameFor[l.AEnd]
		zName := nameFor[l.ZEnd]
		if aName == "" || zName == "" {
			continue
		}
		if aName == zName {
			continue
		}
		// Dedup: sorted endpoints + kind + role.
		pair := [2]string{aName, zName}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		k := dedupKey{pair[0], pair[1], l.Kind, l.Role}
		if seen[k] {
			continue
		}
		seen[k] = true
		rewritten := l
		rewritten.AEnd = aName
		rewritten.ZEnd = zName
		filteredPhys = append(filteredPhys, rewritten)
	}

	seen = make(map[dedupKey]bool)
	filteredLogical := []schema.LogicalLink{}
	for _, l := range g.LogicalLinks {
		if len(v.ConnectionKinds) > 0 && !slices.Contains(v.ConnectionKinds, l.Kind) {
			continue
		}
		if len(v.ConnectionRoles) > 0 && !slices.Contains(v.ConnectionRoles, l.Role) {
			continue
		}
		if matchesAnyExprLink(v.Scope.ExcludeLinks, l.Kind, l.Role) {
			continue
		}
		aName := nameFor[l.AEnd]
		zName := nameFor[l.ZEnd]
		if aName == "" || zName == "" {
			continue
		}
		if aName == zName {
			continue
		}
		pair := [2]string{aName, zName}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		k := dedupKey{pair[0], pair[1], l.Kind, l.Role}
		if seen[k] {
			continue
		}
		seen[k] = true
		rewritten := l
		rewritten.AEnd = aName
		rewritten.ZEnd = zName
		filteredLogical = append(filteredLogical, rewritten)
	}

	// Determine which nodes actually appear in filtered links.
	linkedNodes := make(map[string]bool)
	usedAbstractSites := make(map[string]bool)
	for _, l := range filteredPhys {
		linkedNodes[l.AEnd] = true
		linkedNodes[l.ZEnd] = true
		if collapsed[l.AEnd] {
			usedAbstractSites[l.AEnd] = true
		}
		if collapsed[l.ZEnd] {
			usedAbstractSites[l.ZEnd] = true
		}
	}
	for _, l := range filteredLogical {
		linkedNodes[l.AEnd] = true
		linkedNodes[l.ZEnd] = true
		if collapsed[l.AEnd] {
			usedAbstractSites[l.AEnd] = true
		}
		if collapsed[l.ZEnd] {
			usedAbstractSites[l.ZEnd] = true
		}
	}

	// Prune concrete devices that have no visible links after filtering.
	// A device scoped into the view but connected only via excluded link roles
	// would otherwise appear as an isolated node.
	pruned := concreteDevices[:0]
	for _, d := range concreteDevices {
		if linkedNodes[d.Name] {
			pruned = append(pruned, d)
		}
	}
	concreteDevices = pruned

	// Build final device list: concrete + abstract site nodes that appear in links.
	allDevices := make([]schema.Device, len(concreteDevices))
	copy(allDevices, concreteDevices)
	abstractSiteNames := make([]string, 0, len(usedAbstractSites))
	for s := range usedAbstractSites {
		abstractSiteNames = append(abstractSiteNames, s)
	}
	sort.Strings(abstractSiteNames)
	for _, s := range abstractSiteNames {
		allDevices = append(allDevices, schema.Device{
			Name: s,
			Role: AbstractSiteRole,
			Site: s,
		})
	}

	return &graph.Graph{
		Devices:       allDevices,
		Sites:         g.Sites,
		Regions:       g.Regions,
		PhysicalLinks: filteredPhys,
		LogicalLinks:  filteredLogical,
	}
}

// resolveSiteSets returns the full-detail and collapsed site sets based on scope.
func resolveSiteSets(g *graph.Graph, v *View) (full map[string]bool, collapsed map[string]bool) {
	full = make(map[string]bool)
	collapsed = make(map[string]bool)

	allSites := make([]string, 0, len(g.Sites))
	for _, s := range g.Sites {
		allSites = append(allSites, s.Name)
	}

	if v.Scope.FocusSite != "" {
		full[v.Scope.FocusSite] = true
		for _, s := range allSites {
			if s != v.Scope.FocusSite {
				collapsed[s] = true
			}
		}
	} else {
		if len(v.Scope.Sites) > 0 {
			for _, s := range v.Scope.Sites {
				full[s] = true
			}
		} else {
			for _, s := range allSites {
				full[s] = true
			}
		}

		// Apply collapse_sites globs: move matching sites from full → collapsed.
		for _, pattern := range v.Scope.CollapseSites {
			for s := range full {
				matched, err := path.Match(pattern, s)
				if err == nil && matched {
					delete(full, s)
					collapsed[s] = true
				}
			}
		}
	}

	// L0: force all sites to abstract nodes regardless of scope config.
	if v.DetailLevel == L0 {
		for s := range full {
			collapsed[s] = true
		}
		clear(full)
	}

	return
}

// matchesAnyExpr checks if any key=value expression matches the device.
func matchesAnyExpr(exprs []string, d schema.Device) bool {
	for _, expr := range exprs {
		k, v, ok := strings.Cut(expr, "=")
		if !ok {
			continue
		}
		switch k {
		case "role":
			if d.Role == v {
				return true
			}
		case "site":
			if d.Site == v {
				return true
			}
		case "vendor":
			if d.Vendor == v {
				return true
			}
		case "model":
			if d.Model == v {
				return true
			}
		}
	}
	return false
}

// matchesAnyExprLink checks if any key=value expression matches a link by kind or role.
func matchesAnyExprLink(exprs []string, kind, role string) bool {
	for _, expr := range exprs {
		k, v, ok := strings.Cut(expr, "=")
		if !ok {
			continue
		}
		switch k {
		case "role":
			if role == v {
				return true
			}
		case "kind":
			if kind == v {
				return true
			}
		}
	}
	return false
}
