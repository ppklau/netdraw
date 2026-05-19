# NetDraw YAML Reference

The YAML source files are your network source of truth (SoT). NetDraw reads them, builds an internal graph, applies view filters, and hands the result to a rendering engine (D2 or draw.io). Everything visible in a diagram comes from these files — nothing is hardcoded in the renderer.

There are six file types:

| File | Purpose |
|------|---------|
| `devices.yml` | Network nodes (routers, switches, firewalls, …) |
| `sites.yml` | Physical or logical locations that group devices |
| `physical_links.yml` | Cabled connections between device interfaces |
| `logical_links.yml` | Protocol-level relationships (BGP peers, OSPF adjacencies, …) |
| `views.yml` | Diagram definitions: what to show and at what level of detail |
| `layout_overrides.yml` | Pinned node positions from draw.io edits (auto-managed) |

---

## Lifecycle status

Every entity (device, site, region, physical link, logical link) carries a `status` field that represents its lifecycle stage. Status affects how the entity is drawn and therefore must be set correctly.

| Value | Meaning | Visual treatment |
|-------|---------|-----------------|
| `concept` | Design intent, not yet committed | Devices: dashed-border grey rectangle, muted text. Links: dotted light grey |
| `planned` | In the project plan but not yet built | Devices: icon at 55% opacity. Links: dashed muted grey |
| `confirmed` | Build approved or in progress | Full icon, full colour (same as `active`) |
| `active` | Live in production | Full icon, role-based colour |

`confirmed` and `active` render identically. The distinction is available for your own tracking; diagrams do not differentiate them visually.

---

## devices.yml

```yaml
devices:
  - name: lon-dc1-spine-01
    role: spine
    vendor: arista
    model: 7050CX3-32S
    site: lon-dc1
    status: active
```

### Field reference

| Field | Required | Affects rendering | Notes |
|-------|----------|------------------|-------|
| `name` | yes | yes | Rendered as the primary node label. Also the identifier used in `a_end`/`z_end` of link files. |
| `status` | yes | yes | Controls node visual style. See [Lifecycle status](#lifecycle-status). |
| `role` | no | yes | Selects the node icon (spine, leaf, border_router, firewall, etc.). Appended to the label after the name. Used for `scope.roles` and `exclude_devices: [role=X]` filters. Concept-status devices show role as their entire label. |
| `vendor` | no | yes | Appended to the node label as `· vendor` (e.g. `· arista`). Omitted for `concept`-status devices. Not shown for `model`. |
| `model` | no | no | Stored in the graph and available for `exclude_devices: [model=X]` filtering, but not rendered in any label. |
| `site` | no | yes | Determines which site container the device appears in. Devices without a site are orphaned and will not render unless they appear in a link. |

### Device icons

Icons are resolved by `role`. Unrecognised roles fall back to a generic network-device icon. Vendor is not used for icon selection.

---

## sites.yml

```yaml
sites:
  - name: lon-dc1
    region: emea
    type: datacenter
    status: active
```

### Field reference

| Field | Required | Affects rendering | Notes |
|-------|----------|------------------|-------|
| `name` | yes | yes | Rendered as the site container label. Referenced by `scope.sites`, `scope.focus_site`, and `scope.collapse_sites` in views. |
| `status` | yes | no | Stored in the graph. Site status does not currently affect container appearance — the site container renders the same regardless of status. |
| `type` | no | no | Stored in the graph for external tooling. Not rendered. |
| `region` | no | no | Stored in the graph. Not rendered directly, but regions are used to group sites in the `perdevice` adapter. |

---

## physical_links.yml

Physical links represent cabled connections. Each link connects two device interfaces. The link's role drives its colour and thickness; the status overrides that styling for pre-production links.

```yaml
physical_links:
  - a_end: lon-dc1-spine-01
    z_end: lon-dc1-leaf-01
    a_interface: Ethernet1
    z_interface: Ethernet49
    kind: physical
    role: fabric
    speed: 100G
    source: lldp_discovery
    status: active
```

### Field reference

| Field | Required | Affects rendering | Notes |
|-------|----------|------------------|-------|
| `a_end` | yes | yes | Source device name. Must match a device `name` in `devices.yml`. |
| `z_end` | yes | yes | Destination device name. Must match a device `name` in `devices.yml`. |
| `status` | yes | yes | Overrides role-based styling for pre-production links. See [Link status and colour](#link-status-and-colour). |
| `role` | no | yes | Determines line colour and thickness at `confirmed`/`active` status. Also shown as a centre label at L1 detail. Used by `connection_roles` and `exclude_links: [role=X]` filters. See [Physical link roles](#physical-link-roles). |
| `a_interface` | no | yes | Shown as an endpoint label at the `a_end` of the link at L2 and L3 detail. Not shown at L0 or L1. |
| `z_interface` | no | yes | Shown as an endpoint label at the `z_end` of the link at L2 and L3 detail. |
| `kind` | no | no (filter only) | Must be `physical`. Used by `connection_kinds: [physical]` view filter. Does not affect visual style. |
| `speed` | no | no | Stored in the graph. Not rendered in any diagram output. |
| `source` | no | no | Provenance: `manual`, `lldp_discovery`, or `design_intent`. Not rendered. |
| `lag_id` | no | no | Identifies LAG membership. Not rendered. |

### Physical link roles

The `role` field drives colour and stroke weight on `confirmed`/`active` links. Unrecognised roles fall back to the fabric style.

| Role | Colour | Width | Style |
|------|--------|-------|-------|
| `fabric` | `#546E7A` (blue-grey) | 2 | solid |
| `uplink` | `#455A64` (dark blue-grey) | 2 | solid |
| `wan` | `#E65100` (deep orange) | 3 | solid |
| `oob` | `#BDBDBD` (light grey) | 1 | dotted |
| `mlag_peer` | `#1565C0` (blue) | 2 | solid |
| *(other)* | `#546E7A` (blue-grey) | 1 | solid |

### Link status and colour

`status` takes precedence over `role` for pre-production links:

| Status | Stroke | Width | Style |
|--------|--------|-------|-------|
| `concept` | `#B0BEC5` (light grey) | 1 | dotted (dash 3) |
| `planned` | `#90A4AE` (muted grey) | 1 | dashed (dash 5) |
| `confirmed` / `active` | Role-based (table above) | Role-based | Role-based |

---

## logical_links.yml

Logical links represent protocol-level relationships: BGP peerings, OSPF adjacencies, VXLAN overlays. They render as dashed lines coloured by role. Logical links are typically generated by a config pipeline and should not be hand-edited.

```yaml
logical_links:
  - a_end: lon-dc1-spine-01
    z_end: lon-dc1-spine-02
    kind: logical
    role: ibgp_peering
    protocol: bgp
    source: config_pipeline
    status: active
```

### Field reference

| Field | Required | Affects rendering | Notes |
|-------|----------|------------------|-------|
| `a_end` | yes | yes | Source device name. |
| `z_end` | yes | yes | Destination device name. |
| `status` | yes | yes | Overrides colour for pre-production links (same thresholds as physical links). |
| `role` | no | yes | Determines line colour. Also shown as a centre label at L1 detail. Used by `connection_roles` and `exclude_links: [role=X]` filters. See [Logical link roles](#logical-link-roles). |
| `kind` | no | no (filter only) | `logical` or `derived`. Used by `connection_kinds` view filter. Does not affect visual style. |
| `protocol` | no | no | Stored in the graph. Not rendered. |
| `source` | no | no | Provenance: `config_pipeline` or `manual`. Not rendered. |

### Logical link roles

All logical links render as dashed (dash 5). Status overrides apply as with physical links.

| Role | Colour |
|------|--------|
| `ibgp_peering` | `#5C6BC0` (indigo) |
| `ebgp_peering` | `#2E7D32` (green) |
| `ospf` | `#FF8F00` (amber) |
| *(other)* | `#757575` (grey) |

---

## views.yml

Views are the control surface for what gets rendered. Each entry in `views.yml` produces one output file. A view specifies a detail level, a scope, optional link filters, and layout hints. The SoT files define the network; views define what part of it to show and how.

```yaml
views:
  - name: wan-overview
    detail_level: L0
    scope:
      exclude_links: [role=oob]
    title:
      text: Global WAN Overview
      classification: Internal
    legend: true

  - name: lon-dc1-fabric
    detail_level: L2
    connection_kinds: [physical]
    connection_roles: [fabric, uplink]
    scope:
      focus_site: lon-dc1
      exclude_devices: [role=oob_switch]
    title:
      text: LON-DC1 Fabric Topology
      classification: Internal
```

### Field reference

| Field | Required | Notes |
|-------|----------|-------|
| `name` | yes | Output filename (without extension): `name: lon-dc1-fabric` → `lon-dc1-fabric.svg`. |
| `detail_level` | yes | Controls how much information is shown. See [Detail levels](#detail-levels). |
| `scope` | no | Restricts which sites, devices, and links appear. See [Scope filters](#scope-filters). |
| `connection_kinds` | no | Whitelist of link `kind` values to include (`physical`, `logical`). If omitted, both kinds appear. |
| `connection_roles` | no | Whitelist of link `role` values. If omitted, all roles appear (subject to `exclude_links`). Combine with `connection_kinds` to show only, say, fabric and uplink physical links. |
| `title.text` | no | Title rendered at the top of the diagram. |
| `title.classification` | no | Appended below the title as `Classification: <value>`. |
| `legend` | no | If `true`, adds a legend box (physical / logical / oob line styles). Defaults to `false`. |
| `validation_mode` | no | `strict` (default) or `warn`. Controls whether reference errors in this view fail the render or emit warnings. Not a rendering property — does not affect diagram appearance. |

---

## Detail levels

The detail level is the primary dial for how much information a diagram carries. Choose it based on your audience.

| Level | Node rendering | Link labels | Typical use |
|-------|---------------|-------------|-------------|
| L0 | Every site collapses to a cloud icon regardless of scope settings | None | Executive overviews, programme sponsors |
| L1 | Devices shown inside site containers, with role-based icons | Link role shown as a centre label (e.g. `wan`, `ibgp_peering`) | Architecture reviews, senior engineers |
| L2 | Same as L1, plus interface IDs at each link end | None (interface labels replace role labels) | Change management, interface-level planning |
| L3 | Same as L2 | None | Pair with `focus_site` for a single expanded site with full interface detail |

**L0 and collapse:** At L0, all sites are forced to abstract cloud nodes. `collapse_sites: ["*"]` in the scope has no additional effect at L0.

**Default:** If `detail_level` is omitted, it is a validation error — it must be specified.

**L2 vs L3:** The rendering behaviour of L2 and L3 is currently identical. L3 is reserved for a future expansion where a focus site gets full detail and surrounding sites get a reduced representation.

---

## Scope filters

Scope filters control which sites, devices, and links are included in a view. Filters are applied before rendering; excluded nodes and links are not drawn.

### Site selection

| Filter | Behaviour |
|--------|-----------|
| `sites: [a, b]` | Include only the listed sites. Devices in all other sites are excluded. |
| `focus_site: lon-dc1` | Show the named site with full device detail. All other sites collapse to cloud nodes. Overrides `sites` and `collapse_sites`. |
| `collapse_sites: [branch-*]` | Glob-match site names and collapse each matched site to a single cloud icon. Supports `*` wildcard. Can be combined with `sites` to collapse a subset of the included sites. |

When neither `sites` nor `focus_site` is set, all sites in the SoT are included.

### Device and link filtering

Filters are expressed as `key=value` strings. Multiple entries are OR'd: a device or link is excluded if it matches any expression.

| Filter | Supported keys | Example |
|--------|---------------|---------|
| `exclude_devices` | `role`, `site`, `vendor`, `model` | `exclude_devices: [role=oob_switch, site=lon-hub-01]` |
| `exclude_links` | `role`, `kind` | `exclude_links: [role=oob, kind=logical]` |

Devices that appear in the scope but have no visible links after filtering are automatically pruned. An isolated device will not appear.

### `connection_kinds` and `connection_roles`

These are view-level whitelists (not scope.exclude lists):

- `connection_kinds: [physical]` — include only physical links; logical links are excluded.
- `connection_roles: [fabric, uplink]` — include only links whose `role` is `fabric` or `uplink`.

Both can be combined. They apply before `exclude_links`.

---

## layout_overrides.yml

This file is managed by `netdraw extract` and should not be edited by hand. It stores per-view node positions from draw.io layouts so they survive SoT updates. New nodes added to the SoT are auto-placed; nodes with stored positions stay fixed. See the [draw.io workflow](../README.md#drawio-workflow) section of the README for usage.
