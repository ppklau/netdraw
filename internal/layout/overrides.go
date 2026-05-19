package layout

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Position is a 2D coordinate for a graph node in a specific view.
type Position struct {
	X float64 `yaml:"x"`
	Y float64 `yaml:"y"`
}

// Overrides holds per-view manual node positions loaded from layout_overrides.yml.
type Overrides struct {
	data map[string]map[string]Position
}

type overridesFile struct {
	Overrides map[string]map[string]Position `yaml:"overrides"`
}

// LoadFile reads layout_overrides.yml from path.
// If the file does not exist, an empty Overrides is returned without error.
func LoadFile(path string) (*Overrides, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Overrides{data: make(map[string]map[string]Position)}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var f overridesFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if f.Overrides == nil {
		f.Overrides = make(map[string]map[string]Position)
	}
	return &Overrides{data: f.Overrides}, nil
}

// SaveFile writes o to path in layout_overrides.yml format.
func SaveFile(path string, o *Overrides) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	f := overridesFile{Overrides: o.data}
	raw, err := yaml.Marshal(&f)
	if err != nil {
		return fmt.Errorf("marshaling overrides: %w", err)
	}
	return os.WriteFile(path, raw, 0o644)
}

// ForView returns the node→position map for a specific view (nil if none recorded).
func (o *Overrides) ForView(viewName string) map[string]Position {
	if o == nil {
		return nil
	}
	return o.data[viewName]
}

// SetForView stores a complete position map for viewName, replacing any existing entry.
func (o *Overrides) SetForView(viewName string, positions map[string]Position) {
	if o.data == nil {
		o.data = make(map[string]map[string]Position)
	}
	o.data[viewName] = positions
}
