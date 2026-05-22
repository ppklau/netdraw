# Example diagram gallery

All diagrams rendered from the [example SoT](../examples/) — a fictional three-region network spanning `lon-dc1`, `nyc-dc1`, `sng-dc1` (datacenters) and `lon-hub-01` (campus).

---

## Global WAN overview (L0)

All sites collapsed to abstract nodes. Only inter-site WAN links shown.

View definition: [`wan-overview`](../examples/views.yml)

![Global WAN Overview](diagrams/wan-overview.svg)

---

## EMEA WAN detail (L1)

LON-DC1 expanded at full detail; NYC and SNG collapsed for WAN context. OOB and fabric links excluded.

View definition: [`emea-wan-detail`](../examples/views.yml)

![EMEA WAN Detail](diagrams/emea-wan-detail.svg)

---

## LON-DC1 fabric topology (L2)

Spine-leaf fabric with border routers and firewalls. Interface labels on every link. OOB switch excluded.

View definition: [`lon-dc1-fabric`](../examples/views.yml)

![LON-DC1 Fabric Topology](diagrams/lon-dc1-fabric.svg)

---

## NYC-DC1 fabric topology (L2)

View definition: [`nyc-dc1-fabric`](../examples/views.yml)

![NYC-DC1 Fabric Topology](diagrams/nyc-dc1-fabric.svg)

---

## SNG-DC1 fabric topology (L2)

Includes two planned links to `sng-dc1-leaf-03` (rendered in a distinct style). Validation mode set to `warn`.

View definition: [`sng-dc1-fabric`](../examples/views.yml)

![SNG-DC1 Fabric Topology](diagrams/sng-dc1-fabric.svg)

---

## Global BGP topology (L1)

Logical links only. Scoped to `spine` and `border_router` roles across all three datacenters. Shows iBGP within each site and eBGP between sites.

View definition: [`global-bgp`](../examples/views.yml)

![Global BGP Topology](diagrams/global-bgp.svg)

---

## London HQ campus topology (L2)

Edge router, firewall, dual core switches, and access layer. Physical links only.

View definition: [`lon-hub-01-campus`](../examples/views.yml)

![London HQ Campus Topology](diagrams/lon-hub-01-campus.svg)
