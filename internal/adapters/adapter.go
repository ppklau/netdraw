// Package adapters defines the SoT adapter interface and its implementations.
package adapters

import "github.com/ppklau/netdraw/internal/schema"

// Adapter is the contract every SoT adapter must satisfy.
// Each method loads the corresponding entity collection from the source of truth.
type Adapter interface {
	Devices() ([]schema.Device, error)
	Sites() ([]schema.Site, error)
	Regions() ([]schema.Region, error)
	PhysicalLinks() ([]schema.PhysicalLink, error)
	LogicalLinks() ([]schema.LogicalLink, error)
}
