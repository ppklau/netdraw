package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ppklau/netdraw/internal/schema"
)

func init() {
	cmd := &cobra.Command{
		Use:   "expand",
		Short: "Scaffold devices.yml and physical_links.yml from hld.yml",
		Long: `Reads hld.yml from the SoT root and generates flat YAML files at status: planned.

Named device entries are created with synthetic hostnames derived from role names
and counts (e.g. lon-dc2-spine-01 through lon-dc2-spine-04). Physical links
are generated as a full cross-product of the role pairs defined in hld.yml.

The resulting files are ready for a design engineer to fill in vendor, model,
and interface assignments.

By default expand will not overwrite existing files. Use --force to overwrite.`,
		Args: cobra.NoArgs,
		RunE: runExpand,
	}

	cmd.Flags().StringP("output", "o", "", "output directory (default: SoT root)")
	cmd.Flags().Bool("force", false, "overwrite existing files")

	rootCmd.AddCommand(cmd)
}

// hldExpandFile mirrors the on-disk hld.yml structure used by expand.
type hldExpandFile struct {
	Sites []hldExpandSite `yaml:"sites"`
	Links []hldExpandLink `yaml:"links"`
}

type hldExpandSite struct {
	Name  string         `yaml:"name"`
	Type  string         `yaml:"type"`
	Roles map[string]int `yaml:"roles"`
}

type hldExpandLink struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	Role string `yaml:"role"`
}

// Local types with omitempty on optional fields for clean YAML output.
type expandDevice struct {
	Name   string        `yaml:"name"`
	Role   string        `yaml:"role"`
	Vendor string        `yaml:"vendor,omitempty"`
	Model  string        `yaml:"model,omitempty"`
	Site   string        `yaml:"site"`
	Status schema.Status `yaml:"status"`
}

type expandSite struct {
	Name   string        `yaml:"name"`
	Type   string        `yaml:"type,omitempty"`
	Region string        `yaml:"region,omitempty"`
	Status schema.Status `yaml:"status"`
}

type expandPhysLink struct {
	AEnd       string        `yaml:"a_end"`
	ZEnd       string        `yaml:"z_end"`
	AInterface string        `yaml:"a_interface,omitempty"`
	ZInterface string        `yaml:"z_interface,omitempty"`
	Kind       string        `yaml:"kind"`
	Role       string        `yaml:"role"`
	Speed      string        `yaml:"speed,omitempty"`
	Source     string        `yaml:"source"`
	Status     schema.Status `yaml:"status"`
}

func runExpand(cmd *cobra.Command, _ []string) error {
	outputFlag, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	hldPath := filepath.Join(cfg.SoT, "hld.yml")
	data, err := os.ReadFile(hldPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", hldPath, err)
	}

	var f hldExpandFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parsing %s: %w", hldPath, err)
	}

	outDir := cfg.SoT
	if outputFlag != "" {
		outDir = outputFlag
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Build a map of site → role → list of hostnames, and collect flat slices.
	type siteRoleKey struct{ site, role string }
	deviceNames := make(map[siteRoleKey][]string)

	var sites []expandSite
	var devices []expandDevice

	for _, s := range f.Sites {
		sites = append(sites, expandSite{
			Name:   s.Name,
			Type:   s.Type,
			Status: schema.StatusPlanned,
		})

		for _, role := range sortedStringKeys(s.Roles) {
			count := s.Roles[role]
			if count < 1 {
				count = 1
			}
			key := siteRoleKey{s.Name, role}
			for i := range count {
				hostname := fmt.Sprintf("%s-%s-%02d", s.Name, role, i+1)
				deviceNames[key] = append(deviceNames[key], hostname)
				devices = append(devices, expandDevice{
					Name:   hostname,
					Role:   role,
					Site:   s.Name,
					Status: schema.StatusPlanned,
				})
			}
		}
	}

	// Generate physical links as a full cross-product of each role pair.
	var physLinks []expandPhysLink
	for i, l := range f.Links {
		aSite, aRole, err := expandParseRef(l.From)
		if err != nil {
			return fmt.Errorf("hld.yml link #%d 'from': %w", i+1, err)
		}
		zSite, zRole, err := expandParseRef(l.To)
		if err != nil {
			return fmt.Errorf("hld.yml link #%d 'to': %w", i+1, err)
		}

		aDevs := deviceNames[siteRoleKey{aSite, aRole}]
		zDevs := deviceNames[siteRoleKey{zSite, zRole}]

		if len(aDevs) == 0 {
			return fmt.Errorf("hld.yml link #%d: role %q not defined for site %q", i+1, aRole, aSite)
		}
		if len(zDevs) == 0 {
			return fmt.Errorf("hld.yml link #%d: role %q not defined for site %q", i+1, zRole, zSite)
		}

		for _, a := range aDevs {
			for _, z := range zDevs {
				physLinks = append(physLinks, expandPhysLink{
					AEnd:   a,
					ZEnd:   z,
					Kind:   "physical",
					Role:   l.Role,
					Source: "design_intent",
					Status: schema.StatusPlanned,
				})
			}
		}
	}

	// Write sites.yml, devices.yml, physical_links.yml.
	if err := writeExpandFile(outDir, "sites.yml", map[string]any{"sites": sites}, force); err != nil {
		return err
	}
	if err := writeExpandFile(outDir, "devices.yml", map[string]any{"devices": devices}, force); err != nil {
		return err
	}
	if err := writeExpandFile(outDir, "physical_links.yml", map[string]any{"physical_links": physLinks}, force); err != nil {
		return err
	}

	fmt.Printf("expanded %d devices across %d sites into %s\n", len(devices), len(sites), outDir)
	fmt.Println("  sites.yml, devices.yml, physical_links.yml written at status: planned")
	fmt.Println("  next: set adapter: flat in .netdraw.yml, fill in vendor/model/interfaces")
	return nil
}

func writeExpandFile(dir, name string, payload any, force bool) error {
	path := filepath.Join(dir, name)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists — use --force to overwrite", path)
		}
	}
	out, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", name, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("  wrote %s\n", path)
	return nil
}

func expandParseRef(ref string) (site, role string, err error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid reference %q: expected \"site/role\"", ref)
	}
	return parts[0], parts[1], nil
}

func sortedStringKeys(m map[string]int) []string {
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
