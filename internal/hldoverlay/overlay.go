// Package hldoverlay merges concept entities from an hld.yml file onto the
// base adapter results. It is called by rendercore after any adapter loads its
// SoT, and also used directly by the standalone hld adapter.
package hldoverlay

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ppklau/netdraw/internal/schema"
)

// hldFile mirrors the on-disk hld.yml structure.
type hldFile struct {
	Sites []hldSite `yaml:"sites"`
	Links []hldLink `yaml:"links"`
}

type hldSite struct {
	Name  string         `yaml:"name"`
	Type  string         `yaml:"type"`
	Roles map[string]int `yaml:"roles"`
}

type hldLink struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	Role string `yaml:"role"`
}

// Result holds the merged adapter output after overlay application.
type Result struct {
	Devices      []schema.Device
	Sites        []schema.Site
	PhysicalLinks []schema.PhysicalLink
}

// Merge reads hldPath and merges concept entities from it onto the base slices.
// If hldPath does not exist, the base slices are returned unchanged.
// Concept devices and sites from hld.yml are appended; concept links are
// appended with endpoints resolved using the disambiguation rules:
//   - "site/role" (contains "/") → concept synthetic device (site-role)
//   - bare name (no "/")         → must exist in the base device set
func Merge(
	baseDevices []schema.Device,
	baseSites []schema.Site,
	basePhysLinks []schema.PhysicalLink,
	hldPath string,
) (*Result, error) {
	data, err := os.ReadFile(hldPath)
	if errors.Is(err, os.ErrNotExist) {
		return &Result{
			Devices:       baseDevices,
			Sites:         baseSites,
			PhysicalLinks: basePhysLinks,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", hldPath, err)
	}

	var f hldFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", hldPath, err)
	}

	// Track existing names so we can skip duplicates and validate bare refs.
	knownDevices := make(map[string]bool, len(baseDevices))
	for _, d := range baseDevices {
		knownDevices[d.Name] = true
	}
	knownSites := make(map[string]bool, len(baseSites))
	for _, s := range baseSites {
		knownSites[s.Name] = true
	}

	devices := append([]schema.Device(nil), baseDevices...)
	sites := append([]schema.Site(nil), baseSites...)
	physLinks := append([]schema.PhysicalLink(nil), basePhysLinks...)

	// Add concept sites and their role-group devices.
	for _, s := range f.Sites {
		if !knownSites[s.Name] {
			sites = append(sites, schema.Site{
				Name:   s.Name,
				Type:   s.Type,
				Status: schema.StatusConcept,
			})
			knownSites[s.Name] = true
		}
		for _, role := range sortedKeys(s.Roles) {
			count := s.Roles[role]
			if count < 1 {
				count = 1
			}
			name := conceptDeviceName(s.Name, role)
			devices = append(devices, schema.Device{
				Name:   name,
				Role:   role,
				Site:   s.Name,
				Status: schema.StatusConcept,
				Count:  count,
			})
			knownDevices[name] = true
		}
	}

	// Add concept links with endpoint disambiguation.
	for i, l := range f.Links {
		aEnd, err := resolveRef(l.From, knownDevices)
		if err != nil {
			return nil, fmt.Errorf("hld.yml link #%d 'from': %w", i+1, err)
		}
		zEnd, err := resolveRef(l.To, knownDevices)
		if err != nil {
			return nil, fmt.Errorf("hld.yml link #%d 'to': %w", i+1, err)
		}
		physLinks = append(physLinks, schema.PhysicalLink{
			AEnd:   aEnd,
			ZEnd:   zEnd,
			Kind:   "physical",
			Role:   l.Role,
			Status: schema.StatusConcept,
		})
	}

	return &Result{
		Devices:       devices,
		Sites:         sites,
		PhysicalLinks: physLinks,
	}, nil
}

// resolveRef resolves a link endpoint.
// "site/role" → concept synthetic device name.
// Bare name   → must exist as a known device (base or concept).
func resolveRef(ref string, knownDevices map[string]bool) (string, error) {
	if strings.Contains(ref, "/") {
		site, role, err := parseRef(ref)
		if err != nil {
			return "", err
		}
		name := conceptDeviceName(site, role)
		if !knownDevices[name] {
			return "", fmt.Errorf("concept device %q not declared in hld.yml sites", name)
		}
		return name, nil
	}
	if !knownDevices[ref] {
		return "", fmt.Errorf("device %q not found in SoT — bare names in hld.yml links must be existing devices", ref)
	}
	return ref, nil
}

func parseRef(ref string) (site, role string, err error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid reference %q: expected \"site/role\"", ref)
	}
	return parts[0], parts[1], nil
}

// ConceptDeviceName constructs the synthetic device name for a (site, role) pair.
// Exported so the expand command and tests can use the same convention.
func ConceptDeviceName(site, role string) string {
	return conceptDeviceName(site, role)
}

func conceptDeviceName(site, role string) string {
	return site + "-" + role
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
