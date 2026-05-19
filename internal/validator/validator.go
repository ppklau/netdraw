// Package validator checks SoT entities for required fields and referential integrity.
package validator

import (
	"fmt"

	"github.com/ppklau/netdraw/internal/schema"
)

// Issue is a single validation problem.
type Issue struct {
	Message string
}

// Result holds all issues found during validation.
type Result struct {
	Issues []Issue
}

func (r *Result) add(format string, args ...any) {
	r.Issues = append(r.Issues, Issue{Message: fmt.Sprintf(format, args...)})
}

// Validate checks the provided entity slices for required fields and cross-references.
// In strict mode all issues are errors; in warn mode they are advisory only.
// The returned Result always contains the full issue list — the caller decides the exit behaviour.
func Validate(
	devices []schema.Device,
	sites []schema.Site,
	regions []schema.Region,
	physLinks []schema.PhysicalLink,
	logLinks []schema.LogicalLink,
) *Result {
	r := &Result{}

	deviceSet := make(map[string]bool, len(devices))
	for _, d := range devices {
		if d.Name != "" {
			deviceSet[d.Name] = true
		}
	}
	siteSet := make(map[string]bool, len(sites))
	for _, s := range sites {
		if s.Name != "" {
			siteSet[s.Name] = true
		}
	}
	regionSet := make(map[string]bool, len(regions))
	for _, rg := range regions {
		if rg.Name != "" {
			regionSet[rg.Name] = true
		}
	}
	hasRegions := len(regions) > 0

	for _, d := range devices {
		checkDevice(r, d, siteSet)
	}
	for _, s := range sites {
		checkSite(r, s, regionSet, hasRegions)
	}
	for _, rg := range regions {
		if rg.Name == "" {
			r.add("regions.yml: region with empty name")
		}
	}
	for i, pl := range physLinks {
		checkPhysicalLink(r, pl, i, deviceSet)
	}
	for i, ll := range logLinks {
		checkLogicalLink(r, ll, i, deviceSet)
	}

	return r
}

func checkDevice(r *Result, d schema.Device, siteSet map[string]bool) {
	if d.Name == "" {
		r.add("devices.yml: device with empty name")
		return
	}
	switch d.Status {
	case schema.StatusConcept, "":
		// only name required
	case schema.StatusPlanned:
		requireField(r, "devices.yml", "device", d.Name, "role", d.Role, d.Status)
		requireField(r, "devices.yml", "device", d.Name, "site", d.Site, d.Status)
	case schema.StatusConfirmed:
		requireField(r, "devices.yml", "device", d.Name, "role", d.Role, d.Status)
		requireField(r, "devices.yml", "device", d.Name, "vendor", d.Vendor, d.Status)
		requireField(r, "devices.yml", "device", d.Name, "site", d.Site, d.Status)
	case schema.StatusActive:
		requireField(r, "devices.yml", "device", d.Name, "role", d.Role, d.Status)
		requireField(r, "devices.yml", "device", d.Name, "vendor", d.Vendor, d.Status)
		requireField(r, "devices.yml", "device", d.Name, "model", d.Model, d.Status)
		requireField(r, "devices.yml", "device", d.Name, "site", d.Site, d.Status)
	default:
		r.add("devices.yml: device %q — unknown status %q", d.Name, d.Status)
	}
	if d.Site != "" && !siteSet[d.Site] {
		r.add("devices.yml: device %q — unknown site %q", d.Name, d.Site)
	}
}

func checkSite(r *Result, s schema.Site, regionSet map[string]bool, hasRegions bool) {
	if s.Name == "" {
		r.add("sites.yml: site with empty name")
		return
	}
	switch s.Status {
	case schema.StatusConcept, "":
		// only name required
	case schema.StatusPlanned:
		requireField(r, "sites.yml", "site", s.Name, "type", s.Type, s.Status)
	case schema.StatusConfirmed, schema.StatusActive:
		requireField(r, "sites.yml", "site", s.Name, "type", s.Type, s.Status)
		requireField(r, "sites.yml", "site", s.Name, "region", s.Region, s.Status)
	default:
		r.add("sites.yml: site %q — unknown status %q", s.Name, s.Status)
	}
	if hasRegions && s.Region != "" && !regionSet[s.Region] {
		r.add("sites.yml: site %q — unknown region %q", s.Name, s.Region)
	}
}

func checkPhysicalLink(r *Result, pl schema.PhysicalLink, idx int, deviceSet map[string]bool) {
	if pl.AEnd == "" || pl.ZEnd == "" {
		r.add("physical_links.yml: link #%d — missing a_end or z_end", idx+1)
		return
	}
	ref := fmt.Sprintf("%s → %s", pl.AEnd, pl.ZEnd)
	if !deviceSet[pl.AEnd] {
		r.add("physical_links.yml: link %s — unknown device %q (a_end)", ref, pl.AEnd)
	}
	if !deviceSet[pl.ZEnd] {
		r.add("physical_links.yml: link %s — unknown device %q (z_end)", ref, pl.ZEnd)
	}
	switch pl.Status {
	case schema.StatusConcept, "":
		// only endpoints required
	case schema.StatusPlanned:
		requireField(r, "physical_links.yml", "link", ref, "kind", pl.Kind, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "role", pl.Role, pl.Status)
	case schema.StatusConfirmed:
		requireField(r, "physical_links.yml", "link", ref, "a_interface", pl.AInterface, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "z_interface", pl.ZInterface, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "kind", pl.Kind, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "role", pl.Role, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "speed", pl.Speed, pl.Status)
	case schema.StatusActive:
		requireField(r, "physical_links.yml", "link", ref, "a_interface", pl.AInterface, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "z_interface", pl.ZInterface, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "kind", pl.Kind, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "role", pl.Role, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "speed", pl.Speed, pl.Status)
		requireField(r, "physical_links.yml", "link", ref, "source", pl.Source, pl.Status)
	default:
		r.add("physical_links.yml: link %s — unknown status %q", ref, pl.Status)
	}
}

func checkLogicalLink(r *Result, ll schema.LogicalLink, idx int, deviceSet map[string]bool) {
	if ll.AEnd == "" || ll.ZEnd == "" {
		r.add("logical_links.yml: link #%d — missing a_end or z_end", idx+1)
		return
	}
	ref := fmt.Sprintf("%s → %s", ll.AEnd, ll.ZEnd)
	if !deviceSet[ll.AEnd] {
		r.add("logical_links.yml: link %s — unknown device %q (a_end)", ref, ll.AEnd)
	}
	if !deviceSet[ll.ZEnd] {
		r.add("logical_links.yml: link %s — unknown device %q (z_end)", ref, ll.ZEnd)
	}
	switch ll.Status {
	case schema.StatusConcept, "":
		// only endpoints required
	case schema.StatusPlanned:
		requireField(r, "logical_links.yml", "link", ref, "kind", ll.Kind, ll.Status)
		requireField(r, "logical_links.yml", "link", ref, "role", ll.Role, ll.Status)
	case schema.StatusConfirmed:
		requireField(r, "logical_links.yml", "link", ref, "kind", ll.Kind, ll.Status)
		requireField(r, "logical_links.yml", "link", ref, "role", ll.Role, ll.Status)
		requireField(r, "logical_links.yml", "link", ref, "protocol", ll.Protocol, ll.Status)
	case schema.StatusActive:
		requireField(r, "logical_links.yml", "link", ref, "kind", ll.Kind, ll.Status)
		requireField(r, "logical_links.yml", "link", ref, "role", ll.Role, ll.Status)
		requireField(r, "logical_links.yml", "link", ref, "protocol", ll.Protocol, ll.Status)
		requireField(r, "logical_links.yml", "link", ref, "source", ll.Source, ll.Status)
	default:
		r.add("logical_links.yml: link %s — unknown status %q", ref, ll.Status)
	}
}

func requireField(r *Result, file, kind, name, field, value string, status schema.Status) {
	if value == "" {
		r.add("%s: %s %q — missing %s (status: %s)", file, kind, name, field, status)
	}
}
