// Package hld implements the standalone High Level Design adapter.
// It wraps the hldoverlay package with an empty base graph, producing
// concept-mode entities for a greenfield topology described in hld.yml.
//
// For overlaying HLD concept entities onto an existing SoT, the overlay
// is applied transparently by rendercore — adapter authors do not need
// to use this package directly.
package hld

import (
	"path/filepath"

	"github.com/ppklau/netdraw/internal/hldoverlay"
	"github.com/ppklau/netdraw/internal/schema"
)

// Adapter reads hld.yml from root and presents it as a standalone SoT.
type Adapter struct {
	root   string
	cached *hldoverlay.Result
	err    error
	loaded bool
}

// New returns an HLD Adapter rooted at the given directory.
func New(root string) *Adapter {
	return &Adapter{root: root}
}

func (a *Adapter) load() (*hldoverlay.Result, error) {
	if !a.loaded {
		a.loaded = true
		a.cached, a.err = hldoverlay.Merge(nil, nil, nil, filepath.Join(a.root, "hld.yml"))
	}
	return a.cached, a.err
}

func (a *Adapter) Devices() ([]schema.Device, error) {
	r, err := a.load()
	if err != nil {
		return nil, err
	}
	return r.Devices, nil
}

func (a *Adapter) Sites() ([]schema.Site, error) {
	r, err := a.load()
	if err != nil {
		return nil, err
	}
	return r.Sites, nil
}

func (a *Adapter) Regions() ([]schema.Region, error) {
	return nil, nil
}

func (a *Adapter) PhysicalLinks() ([]schema.PhysicalLink, error) {
	r, err := a.load()
	if err != nil {
		return nil, err
	}
	return r.PhysicalLinks, nil
}

func (a *Adapter) LogicalLinks() ([]schema.LogicalLink, error) {
	return nil, nil
}
