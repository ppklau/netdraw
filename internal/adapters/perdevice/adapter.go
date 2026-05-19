// Package perdevice implements the per-device YAML SoT adapter.
// It reads a directory structure where each device has its own YAML file,
// organised under a site subdirectory. Links are derived from the device
// files — there are no separate physical_links.yml or logical_links.yml.
//
// Expected directory layout:
//
//	sot/
//	├── regions/<code>.yml        → schema.Region
//	├── sites/<code>.yml          → schema.Site
//	└── devices/<site>/<host>.yml → schema.Device + link derivation
//
// Field mappings:
//   - platform "arista_eos" → vendor "arista"
//   - platform "frr"        → vendor "frr"
//   - lab_state "active"    → status active
//   - lab_state anything else → status planned
//
// Physical links are derived by walking each device's interfaces[] list and
// collecting (hostname, iface, peer_device, peer_interface) tuples, then
// deduplicating by normalising each pair so each link appears once.
//
// Logical links are derived from each device's bgp.neighbors[] list, resolved
// to hostnames via loopback and interface IP addresses.
package perdevice

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ppklau/netdraw/internal/schema"
)

// Adapter reads a per-device YAML SoT rooted at a directory.
type Adapter struct {
	root    string
	devs    []pdDevice
	devLoad bool
}

// New returns a perdevice Adapter rooted at the given SoT directory.
func New(root string) *Adapter {
	return &Adapter{root: root}
}

// ─── YAML structs ─────────────────────────────────────────────────────────────

type pdRegionFile struct {
	Region struct {
		Code string `yaml:"code"`
		Name string `yaml:"name"`
	} `yaml:"region"`
}

type pdSiteFile struct {
	Site struct {
		Code            string `yaml:"code"`
		Name            string `yaml:"name"`
		Region          string `yaml:"region"`
		LabInstantiated bool   `yaml:"lab_instantiated"`
	} `yaml:"site"`
}

type pdDevice struct {
	Hostname      string        `yaml:"hostname"`
	Role          string        `yaml:"role"`
	Platform      string        `yaml:"platform"`
	HardwareModel string        `yaml:"hardware_model"`
	Site          string        `yaml:"site"`
	LabState      string        `yaml:"lab_state"`
	Loopback      pdLoopback    `yaml:"loopback"`
	Interfaces    []pdInterface `yaml:"interfaces"`
	BGP           *pdBGP        `yaml:"bgp"`
}

type pdLoopback struct {
	IP string `yaml:"ip"`
}

type pdInterface struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	IP            string `yaml:"ip"`
	PeerDevice    string `yaml:"peer_device"`
	PeerInterface string `yaml:"peer_interface"`
	MTU           int    `yaml:"mtu"`
	Speed         string `yaml:"speed"`
}

type pdBGP struct {
	LocalAS   int           `yaml:"local_as"`
	Neighbors []pdNeighbor  `yaml:"neighbors"`
}

type pdNeighbor struct {
	PeerIP   string `yaml:"peer_ip"`
	RemoteAS int    `yaml:"remote_as"`
}

// ─── Adapter interface ────────────────────────────────────────────────────────

func (a *Adapter) Regions() ([]schema.Region, error) {
	dir := filepath.Join(a.root, "regions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("perdevice: reading regions: %w", err)
	}

	var out []schema.Region
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		var f pdRegionFile
		if err := readYAML(filepath.Join(dir, e.Name()), &f); err != nil {
			return nil, err
		}
		out = append(out, schema.Region{
			Name:   f.Region.Code,
			Status: schema.StatusActive,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (a *Adapter) Sites() ([]schema.Site, error) {
	dir := filepath.Join(a.root, "sites")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("perdevice: reading sites: %w", err)
	}

	var out []schema.Site
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		var f pdSiteFile
		if err := readYAML(filepath.Join(dir, e.Name()), &f); err != nil {
			return nil, err
		}
		status := schema.StatusPlanned
		if f.Site.LabInstantiated {
			status = schema.StatusActive
		}
		out = append(out, schema.Site{
			Name:   f.Site.Code,
			Type:   "datacenter",
			Region: f.Site.Region,
			Status: status,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (a *Adapter) Devices() ([]schema.Device, error) {
	devs, err := a.allDevices()
	if err != nil {
		return nil, err
	}

	out := make([]schema.Device, 0, len(devs))
	for _, d := range devs {
		out = append(out, schema.Device{
			Name:   d.Hostname,
			Role:   d.Role,
			Vendor: vendorFromPlatform(d.Platform),
			Model:  d.HardwareModel,
			Site:   d.Site,
			Status: statusFromLabState(d.LabState),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (a *Adapter) PhysicalLinks() ([]schema.PhysicalLink, error) {
	devs, err := a.allDevices()
	if err != nil {
		return nil, err
	}

	knownDevs := make(map[string]bool, len(devs))
	for _, d := range devs {
		knownDevs[d.Hostname] = true
	}

	seen := map[string]bool{}
	var out []schema.PhysicalLink

	for _, d := range devs {
		for _, iface := range d.Interfaces {
			if iface.PeerDevice == "" || iface.PeerInterface == "" {
				continue
			}
			if !knownDevs[iface.PeerDevice] {
				continue
			}
			key := endpointKey(d.Hostname, iface.Name, iface.PeerDevice, iface.PeerInterface)
			if seen[key] {
				continue
			}
			seen[key] = true

			out = append(out, schema.PhysicalLink{
				AEnd:       d.Hostname,
				ZEnd:       iface.PeerDevice,
				AInterface: iface.Name,
				ZInterface: iface.PeerInterface,
				Kind:       "physical",
				Role:       physRole(iface.Description, iface.MTU),
				Speed:      inferSpeed(iface.Speed, iface.MTU),
				Source:     "lldp_discovery",
				Status:     statusFromLabState(d.LabState),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].AEnd != out[j].AEnd {
			return out[i].AEnd < out[j].AEnd
		}
		return out[i].ZEnd < out[j].ZEnd
	})
	return out, nil
}

func (a *Adapter) LogicalLinks() ([]schema.LogicalLink, error) {
	devs, err := a.allDevices()
	if err != nil {
		return nil, err
	}

	ipToHost := buildIPMap(devs)

	seen := map[string]bool{}
	var out []schema.LogicalLink

	for _, d := range devs {
		if d.BGP == nil {
			continue
		}
		for _, n := range d.BGP.Neighbors {
			peerHost, ok := ipToHost[stripCIDR(n.PeerIP)]
			if !ok {
				continue
			}

			role := "ibgp_peering"
			if n.RemoteAS != d.BGP.LocalAS {
				role = "ebgp_peering"
			}

			key := logicalKey(d.Hostname, peerHost, role)
			if seen[key] {
				continue
			}
			seen[key] = true

			out = append(out, schema.LogicalLink{
				AEnd:     d.Hostname,
				ZEnd:     peerHost,
				Kind:     "logical",
				Role:     role,
				Protocol: "bgp",
				Source:   "config_pipeline",
				Status:   statusFromLabState(d.LabState),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].AEnd != out[j].AEnd {
			return out[i].AEnd < out[j].AEnd
		}
		return out[i].ZEnd < out[j].ZEnd
	})
	return out, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (a *Adapter) allDevices() ([]pdDevice, error) {
	if a.devLoad {
		return a.devs, nil
	}

	devDir := filepath.Join(a.root, "devices")
	siteDirs, err := os.ReadDir(devDir)
	if err != nil {
		return nil, fmt.Errorf("perdevice: reading devices: %w", err)
	}

	for _, siteEntry := range siteDirs {
		if !siteEntry.IsDir() || siteEntry.Name() == "branches" {
			continue
		}
		siteDir := filepath.Join(devDir, siteEntry.Name())
		files, err := os.ReadDir(siteDir)
		if err != nil {
			return nil, fmt.Errorf("perdevice: reading %s: %w", siteDir, err)
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".yml") {
				continue
			}
			var dev pdDevice
			if err := readYAML(filepath.Join(siteDir, f.Name()), &dev); err != nil {
				return nil, err
			}
			if dev.Hostname == "" {
				continue
			}
			a.devs = append(a.devs, dev)
		}
	}

	a.devLoad = true
	return a.devs, nil
}

func buildIPMap(devs []pdDevice) map[string]string {
	m := make(map[string]string, len(devs)*8)
	for _, d := range devs {
		if d.Loopback.IP != "" {
			m[stripCIDR(d.Loopback.IP)] = d.Hostname
		}
		for _, iface := range d.Interfaces {
			if iface.IP != "" {
				m[stripCIDR(iface.IP)] = d.Hostname
			}
		}
	}
	return m
}

func stripCIDR(cidr string) string {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return strings.TrimSpace(cidr)
	}
	return ip.String()
}

func vendorFromPlatform(platform string) string {
	switch platform {
	case "arista_eos":
		return "arista"
	case "frr":
		return "frr"
	default:
		return platform
	}
}

func statusFromLabState(state string) schema.Status {
	if state == "active" {
		return schema.StatusActive
	}
	return schema.StatusPlanned
}

func inferSpeed(speed string, mtu int) string {
	if speed != "" {
		return speed
	}
	switch mtu {
	case 9214:
		return "100G"
	case 9000:
		return "10G"
	default:
		return ""
	}
}

func physRole(desc string, mtu int) string {
	lower := strings.ToLower(desc)
	switch {
	case strings.Contains(lower, "[fabric]") || mtu == 9214:
		return "fabric"
	case strings.HasPrefix(lower, "wan"):
		return "wan"
	case strings.Contains(lower, "oob") || strings.Contains(lower, "management"):
		return "oob"
	default:
		return "internal"
	}
}

func endpointKey(hostA, ifaceA, hostZ, ifaceZ string) string {
	a := hostA + ":" + ifaceA
	z := hostZ + ":" + ifaceZ
	if a > z {
		a, z = z, a
	}
	return a + "|" + z
}

func logicalKey(a, z, role string) string {
	if a > z {
		a, z = z, a
	}
	return a + "|" + z + "|" + role
}

func readYAML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("perdevice: reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("perdevice: parsing %s: %w", path, err)
	}
	return nil
}
