// Package graph defines the normalised topology representation produced by all adapters.
package graph

import "github.com/ppklau/netdraw/internal/schema"

// Graph is the normalised intermediate form that the view layer and renderers operate on.
// All adapters must produce a Graph; the rest of the pipeline never touches raw SoT files.
type Graph struct {
	Devices       []schema.Device
	Sites         []schema.Site
	Regions       []schema.Region
	PhysicalLinks []schema.PhysicalLink
	LogicalLinks  []schema.LogicalLink
}

// Build assembles a Graph from the entity slices returned by an adapter.
func Build(
	devices []schema.Device,
	sites []schema.Site,
	regions []schema.Region,
	physicalLinks []schema.PhysicalLink,
	logicalLinks []schema.LogicalLink,
) *Graph {
	return &Graph{
		Devices:       devices,
		Sites:         sites,
		Regions:       regions,
		PhysicalLinks: physicalLinks,
		LogicalLinks:  logicalLinks,
	}
}
