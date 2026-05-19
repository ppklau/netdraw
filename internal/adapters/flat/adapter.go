// Package flat implements the flat YAML SoT adapter.
// It reads the file layout produced by netdraw init:
//
//	<sot-root>/
//	├── devices.yml
//	├── sites.yml
//	├── regions.yml          (optional)
//	├── physical_links.yml
//	└── logical_links.yml    (optional)
package flat

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/ppklau/netdraw/internal/schema"
)

// Adapter reads the flat file layout scaffolded by netdraw init.
type Adapter struct {
	root string
}

// New returns a flat Adapter rooted at the given directory.
func New(root string) *Adapter {
	return &Adapter{root: root}
}

func (a *Adapter) Devices() ([]schema.Device, error) {
	var f struct {
		Devices []schema.Device `yaml:"devices"`
	}
	if err := a.load("devices.yml", &f); err != nil {
		return nil, err
	}
	return f.Devices, nil
}

func (a *Adapter) Sites() ([]schema.Site, error) {
	var f struct {
		Sites []schema.Site `yaml:"sites"`
	}
	if err := a.load("sites.yml", &f); err != nil {
		return nil, err
	}
	return f.Sites, nil
}

func (a *Adapter) Regions() ([]schema.Region, error) {
	path := filepath.Join(a.root, "regions.yml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	var f struct {
		Regions []schema.Region `yaml:"regions"`
	}
	if err := a.load("regions.yml", &f); err != nil {
		return nil, err
	}
	return f.Regions, nil
}

func (a *Adapter) PhysicalLinks() ([]schema.PhysicalLink, error) {
	var f struct {
		PhysicalLinks []schema.PhysicalLink `yaml:"physical_links"`
	}
	if err := a.load("physical_links.yml", &f); err != nil {
		return nil, err
	}
	return f.PhysicalLinks, nil
}

func (a *Adapter) LogicalLinks() ([]schema.LogicalLink, error) {
	path := filepath.Join(a.root, "logical_links.yml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	var f struct {
		LogicalLinks []schema.LogicalLink `yaml:"logical_links"`
	}
	if err := a.load("logical_links.yml", &f); err != nil {
		return nil, err
	}
	return f.LogicalLinks, nil
}

func (a *Adapter) load(filename string, out interface{}) error {
	path := filepath.Join(a.root, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	return nil
}
