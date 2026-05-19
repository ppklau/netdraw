package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Scaffold a new netdraw project with example files",
		Args:  cobra.NoArgs,
		RunE:  runInit,
	})
}

func runInit(_ *cobra.Command, _ []string) error {
	if _, err := os.Stat(".netdraw.yml"); err == nil {
		return fmt.Errorf(".netdraw.yml already exists — refusing to overwrite an existing project")
	}

	files := map[string]string{
		".netdraw.yml":        scaffoldNetdrawYML,
		"views.yml":           scaffoldViewsYML,
		"devices.yml":         scaffoldDevicesYML,
		"sites.yml":           scaffoldSitesYML,
		"regions.yml":         scaffoldRegionsYML,
		"physical_links.yml":  scaffoldPhysicalLinksYML,
		"logical_links.yml":   scaffoldLogicalLinksYML,
	}

	order := []string{
		".netdraw.yml",
		"devices.yml",
		"sites.yml",
		"regions.yml",
		"physical_links.yml",
		"logical_links.yml",
		"views.yml",
	}

	for _, name := range order {
		if err := os.WriteFile(name, []byte(files[name]), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		fmt.Printf("  created  %s\n", name)
	}

	fmt.Println()
	fmt.Println("Project initialised. Run `netdraw validate` to check the example files.")
	return nil
}

const scaffoldNetdrawYML = `adapter: flat
sot: .
views: views.yml
output: diagrams/
`

const scaffoldDevicesYML = `devices:
  - name: lon-dc1-spine-01
    role: spine
    vendor: arista
    model: 7050CX3-32S
    site: lon-dc1
    status: active

  - name: lon-dc1-spine-02
    role: spine
    vendor: arista
    model: 7050CX3-32S
    site: lon-dc1
    status: active

  - name: lon-dc1-leaf-01
    role: leaf
    vendor: arista
    model: 7050CX3-32S
    site: lon-dc1
    status: active

  - name: lon-dc1-leaf-02
    role: leaf
    vendor: arista
    model: 7050CX3-32S
    site: lon-dc1
    status: active

  - name: lon-dc1-border-01
    role: border_router
    vendor: arista
    model: 7280R3-48FGP
    site: lon-dc1
    status: active

  - name: nyc-dc1-spine-01
    role: spine
    vendor: arista
    model: 7050CX3-32S
    site: nyc-dc1
    status: active

  - name: nyc-dc1-spine-02
    role: spine
    vendor: arista
    model: 7050CX3-32S
    site: nyc-dc1
    status: active

  - name: nyc-dc1-leaf-01
    role: leaf
    vendor: arista
    model: 7050CX3-32S
    site: nyc-dc1
    status: active

  - name: nyc-dc1-fw-01
    role: firewall
    vendor: fortinet
    model: FortiGate-600E
    site: nyc-dc1
    status: active
`

const scaffoldSitesYML = `sites:
  - name: lon-dc1
    type: datacenter
    region: emea
    status: active

  - name: nyc-dc1
    type: datacenter
    region: amer
    status: active
`

const scaffoldRegionsYML = `regions:
  - name: emea
    status: active

  - name: amer
    status: active
`

const scaffoldPhysicalLinksYML = `physical_links:
  # lon-dc1 fabric — spine-leaf
  - a_end: lon-dc1-spine-01
    z_end: lon-dc1-leaf-01
    a_interface: Ethernet1
    z_interface: Ethernet1
    kind: physical
    role: fabric
    speed: 100G
    source: manual
    status: active

  - a_end: lon-dc1-spine-01
    z_end: lon-dc1-leaf-02
    a_interface: Ethernet2
    z_interface: Ethernet1
    kind: physical
    role: fabric
    speed: 100G
    source: manual
    status: active

  - a_end: lon-dc1-spine-02
    z_end: lon-dc1-leaf-01
    a_interface: Ethernet1
    z_interface: Ethernet2
    kind: physical
    role: fabric
    speed: 100G
    source: manual
    status: active

  - a_end: lon-dc1-spine-02
    z_end: lon-dc1-leaf-02
    a_interface: Ethernet2
    z_interface: Ethernet2
    kind: physical
    role: fabric
    speed: 100G
    source: manual
    status: active

  # nyc-dc1 fabric — spine-leaf
  - a_end: nyc-dc1-spine-01
    z_end: nyc-dc1-leaf-01
    a_interface: Ethernet1
    z_interface: Ethernet1
    kind: physical
    role: fabric
    speed: 100G
    source: manual
    status: active

  - a_end: nyc-dc1-spine-02
    z_end: nyc-dc1-leaf-01
    a_interface: Ethernet1
    z_interface: Ethernet2
    kind: physical
    role: fabric
    speed: 100G
    source: manual
    status: active

  # nyc-dc1 firewall uplinks
  - a_end: nyc-dc1-spine-01
    z_end: nyc-dc1-fw-01
    a_interface: Ethernet3
    z_interface: port1
    kind: physical
    role: uplink
    speed: 10G
    source: manual
    status: active

  - a_end: nyc-dc1-spine-02
    z_end: nyc-dc1-fw-01
    a_interface: Ethernet3
    z_interface: port2
    kind: physical
    role: uplink
    speed: 10G
    source: manual
    status: active

  # WAN — inter-site
  - a_end: lon-dc1-border-01
    z_end: nyc-dc1-fw-01
    a_interface: Ethernet1
    z_interface: wan1
    kind: physical
    role: wan
    speed: 10G
    source: manual
    status: active

  # OOB management link
  - a_end: lon-dc1-spine-01
    z_end: lon-dc1-spine-02
    a_interface: Management0
    z_interface: Management0
    kind: physical
    role: oob
    speed: 1G
    source: manual
    status: active
`

const scaffoldLogicalLinksYML = `logical_links:
  - a_end: lon-dc1-spine-01
    z_end: nyc-dc1-spine-01
    kind: logical
    role: ibgp_peering
    protocol: bgp
    source: config_pipeline
    status: active

  - a_end: lon-dc1-spine-02
    z_end: nyc-dc1-spine-02
    kind: logical
    role: ibgp_peering
    protocol: bgp
    source: config_pipeline
    status: active
`

const scaffoldViewsYML = `views:
  - name: wan-overview
    detail_level: L0
    scope:
      sites: [lon-dc1, nyc-dc1]
      exclude_links: [role=oob]
    validation_mode: strict
    title:
      text: WAN Overview
      classification: Internal
    legend: true

  - name: lon-dc1-fabric
    detail_level: L2
    connection_kinds: [physical]
    connection_roles: [fabric]
    scope:
      focus_site: lon-dc1
    validation_mode: strict
`
