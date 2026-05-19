// Package views handles views.yml parsing, scope filtering, and detail level logic.
package views

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type View struct {
	Name            string   `yaml:"name"`
	DetailLevel     string   `yaml:"detail_level"`
	ConnectionKinds []string `yaml:"connection_kinds"`
	ConnectionRoles []string `yaml:"connection_roles"`
	ValidationMode  string   `yaml:"validation_mode"`
	Legend          bool     `yaml:"legend"`
	Scope           Scope    `yaml:"scope"`
	Title           Title    `yaml:"title"`
}

type Scope struct {
	Sites          []string `yaml:"sites"`
	Roles          []string `yaml:"roles"`
	FocusSite      string   `yaml:"focus_site"`
	CollapseSites  []string `yaml:"collapse_sites"`
	ExcludeLinks   []string `yaml:"exclude_links"`
	ExcludeDevices []string `yaml:"exclude_devices"`
}

type Title struct {
	Text           string `yaml:"text"`
	Classification string `yaml:"classification"`
}

// ParseFile reads and parses a views.yml file.
func ParseFile(path string) ([]View, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var f struct {
		Views []View `yaml:"views"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	for i := range f.Views {
		v := &f.Views[i]
		if v.DetailLevel == "" {
			v.DetailLevel = L2
		} else if !IsValidLevel(v.DetailLevel) {
			return nil, fmt.Errorf("view %q: unknown detail_level %q (must be L0, L1, L2, or L3)", v.Name, v.DetailLevel)
		}
	}
	return f.Views, nil
}

// Find returns the first view with the given name.
func Find(views []View, name string) (*View, bool) {
	for i := range views {
		if views[i].Name == name {
			return &views[i], true
		}
	}
	return nil, false
}
