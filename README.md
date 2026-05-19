# NetDraw

A network topology diagramming pipeline that transforms YAML source-of-truth files into production-quality SVG and draw.io diagrams.

NetDraw sits above rendering engines (D2, draw.io) and adds network-native semantics: it understands device roles, link types, sites, and regions, and provides a view language that lets you define *what to show* independently of how the network is structured. Diagrams render natively in GitHub (SVG) and can be opened in draw.io for manual layout editing.

## How it works

```
devices.yml          ─┐
sites.yml             ├──► netdraw render ──► diagrams/*.svg
physical_links.yml    │                   └──► diagrams/*.drawio
logical_links.yml    ─┘
views.yml               (view definitions: scope, detail level, title)
layout_overrides.yml    (optional: pinned positions from draw.io edits)
```

Views are defined in `views.yml`. Each view specifies a detail level (L0–L3), a scope (which sites or roles to include), and filters (exclude OOB links, show only fabric, etc.). NetDraw generates one diagram per view.

## Installation

Requires Go 1.24 or later.

```bash
go install github.com/ppklau/netdraw/cmd/netdraw@latest
```

Or build from source:

```bash
git clone https://github.com/ppklau/netdraw
cd netdraw
go build ./cmd/netdraw
```

This produces a single `./netdraw` binary with no runtime dependencies.

## Quick start

```bash
# Scaffold a new project with example YAML files
netdraw init

# Render all views to SVG
netdraw render --all --output diagrams/

# Render a single view as both SVG and draw.io
netdraw render --view wan-overview --format svg,drawio --output diagrams/

# List available views
netdraw views

# Validate SoT and view definitions without rendering
netdraw validate
```

See [docs/design-workflow.md](docs/design-workflow.md) for a full walkthrough from
concept HLD to as-built diagrams using the `hld` → `flat` adapter progression.

## YAML source files

See [docs/yaml-reference.md](docs/yaml-reference.md) for the full field-by-field reference, including which fields affect rendering, link colour tables, and detail level semantics.

Quick example:

```yaml
# devices.yml
devices:
  - name: lon-dc1-spine-01
    role: spine
    vendor: arista
    site: lon-dc1
    status: active

# views.yml
views:
  - name: lon-dc1-fabric
    detail_level: L2
    connection_kinds: [physical]
    connection_roles: [fabric, uplink]
    scope:
      focus_site: lon-dc1
    title:
      text: LON-DC1 Fabric Topology
      classification: Internal
```

## Adapters

NetDraw is SoT-agnostic. The adapter translates your source of truth into the normalised
graph that views and renderers consume.

| Adapter | When to use | SoT structure |
|---------|-------------|---------------|
| `flat` | Default. Hand-maintained YAML or new designs. | `devices.yml`, `sites.yml`, `physical_links.yml`, `logical_links.yml` in one directory |
| `hld` | Architect/concept stage. Roles and counts, no named devices. | Single `hld.yml` |
| `perdevice` | Custom IaC pipelines. One YAML file per device, links derived from device config. | `regions/`, `sites/`, `devices/<site>/<host>.yml` |

### HLD overlay

Any adapter can be augmented with an `hld.yml` overlay. Drop an `hld.yml` alongside
your existing SoT files and NetDraw merges concept-mode additions on top of the live SoT
without any changes to `.netdraw.yml`. Useful for sketching new infrastructure against
an existing network.

Link references in the overlay use a disambiguation convention:
- `site/role` (contains `/`) → concept synthetic device
- `bare-device-name` (no `/`) → must be an existing device in the base SoT

### `netdraw expand` — HLD to flat YAML

After the architect's concept phase, design engineers use `netdraw expand` to scaffold
the flat YAML files at `status: planned`:

```bash
netdraw expand          # reads hld.yml, writes sites.yml, devices.yml, physical_links.yml
netdraw expand --force  # overwrite existing files
```

Devices get synthetic hostnames (`lon-dc2-spine-01` through `lon-dc2-spine-04`).
Physical links are generated as a full cross-product of each role pair.
The design engineer then fills in vendor, model, and interface assignments and sets
`adapter: flat` in `.netdraw.yml`.

## Configuration

NetDraw reads config from CLI flags, then environment variables, then `.netdraw.yml`, then defaults.

```yaml
# .netdraw.yml
adapter: flat       # SoT adapter: flat (default) | hld | perdevice
sot: .              # path to directory containing devices.yml, etc.
views: views.yml
output: diagrams/
```

Environment variables: `NETDRAW_ADAPTER`, `NETDRAW_SOT`.

## Commands

| Command | Description |
|---------|-------------|
| `netdraw init` | Scaffold a new project with example files |
| `netdraw expand` | Generate `devices.yml` and `physical_links.yml` from `hld.yml` |
| `netdraw render --view <name>` | Render a single view |
| `netdraw render --all` | Render all views in views.yml |
| `netdraw render --format svg,drawio` | Render in multiple output formats |
| `netdraw views` | List views and their detail levels |
| `netdraw validate` | Validate SoT and views (strict mode) |
| `netdraw validate --mode warn` | Validate and emit warnings without failing |
| `netdraw watch --view <name>` | Watch YAML files and re-render on every save |
| `netdraw watch --all` | Watch and re-render all views on every save |
| `netdraw extract <file.drawio>` | Write node positions from a draw.io file to layout_overrides.yml |

## Live preview workflow

Watch mode gives you a near-instant edit → preview loop with no custom extension required:

1. Install the [SVG Preview](https://marketplace.visualstudio.com/items?itemName=SimonSiefke.svg-preview) extension in VS Code (or any extension that auto-refreshes SVG files on disk change).

2. Start the watcher in the VS Code integrated terminal:
   ```bash
   netdraw watch --view wan-overview --output diagrams/
   ```

3. Open `diagrams/wan-overview.svg` in a preview panel (right side).

4. Edit any YAML file and save — the preview refreshes within ~1 second.

`watch` only prints `rendered:` when the output content actually changes, so unchanged saves are silent. Any validation errors are printed to stderr without stopping the watcher.

## draw.io workflow

draw.io output lets you refine layouts manually. Positions are persisted in `layout_overrides.yml` so they survive SoT updates.

```bash
# 1. Render the draw.io file
netdraw render --view lon-dc1-fabric --format drawio --output diagrams/

# 2. Open diagrams/lon-dc1-fabric.drawio in draw.io, reposition nodes, save

# 3. Extract the new positions back to layout_overrides.yml
netdraw extract diagrams/lon-dc1-fabric.drawio

# 4. Re-render — nodes appear in the positions you set
netdraw render --view lon-dc1-fabric --format svg,drawio --output diagrams/
```

`layout_overrides.yml` is keyed by view name, so each view's positions are independent. New nodes added to the SoT are auto-placed; existing nodes with stored positions stay where you put them.

## JSON Schema support

JSON Schema files in `schemas/` enable IDE autocomplete and inline validation for all YAML files. To enable in VS Code, add to `.vscode/settings.json`:

```json
{
  "yaml.schemas": {
    "./schemas/devices.schema.json": "devices.yml",
    "./schemas/sites.schema.json": "sites.yml",
    "./schemas/physical_links.schema.json": "physical_links.yml",
    "./schemas/logical_links.schema.json": "logical_links.yml",
    "./schemas/regions.schema.json": "regions.yml",
    "./schemas/views.schema.json": "views.yml"
  }
}
```

---

## Developer guide

### Prerequisites

- Go 1.24+
- D2 is embedded via the Go module (no separate install needed)

### Build

```bash
go build ./cmd/netdraw        # build binary to ./netdraw
go build ./...                # build all packages
```

### Test

```bash
go test ./...
go test -v ./internal/views
go test -v ./internal/renderers/d2
```

### Project structure

```
cmd/netdraw/          CLI commands (Cobra)
internal/
  adapters/           SoT adapter interface
    flat/             Flat YAML adapter (devices.yml / sites.yml / *_links.yml)
    hld/              HLD adapter — thin wrapper around hldoverlay (standalone hld.yml)
    perdevice/        Per-device YAML adapter (one file per device, links derived)
  hldoverlay/         Overlay engine — merges hld.yml concept entities onto any SoT
  config/             Config resolution (CLI → env → file → defaults)
  graph/              Normalized topology graph
  icons/              Icon resolution (role + vendor → SVG data URI)
  layout/             layout_overrides.yml read/write, position persistence
  renderers/
    d2/               D2 script generation and SVG rendering
    drawio/           draw.io XML generation and position extraction
  schema/             Shared types (Device, Site, Link, Region)
  validator/          SoT consistency validation
  views/              views.yml parser, scope filtering, detail level logic
examples/             Example topology (flat YAML, used by netdraw init)
schemas/              JSON Schema files for all YAML inputs
docs/                 Design workflow guide
```

### GitHub Actions — auto-render on push

Commit SVG diagrams as CI artefacts so they render in GitHub pull requests:

```yaml
# .github/workflows/render.yml
name: Render diagrams
on:
  push:
    paths:
      - '**.yml'

jobs:
  render:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - run: go install github.com/ppklau/netdraw/cmd/netdraw@latest

      - run: netdraw render --all --output diagrams/

      - name: Commit rendered diagrams
        run: |
          git config user.name  "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add diagrams/
          git diff --cached --quiet || git commit -m "chore: render diagrams [skip ci]"
          git push
```

### Adding a SoT adapter

Implement the `adapters.Adapter` interface:

```go
type Adapter interface {
    Devices()       ([]schema.Device,       error)
    Sites()         ([]schema.Site,         error)
    Regions()       ([]schema.Region,       error)
    PhysicalLinks() ([]schema.PhysicalLink, error)
    LogicalLinks()  ([]schema.LogicalLink,  error)
}
```

Register the adapter name in `cmd/netdraw/config.go`.

### Key dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `oss.terrastruct.com/d2` | Diagram scripting and SVG rendering |
| `github.com/fsnotify/fsnotify` | Cross-platform filesystem watcher |
| `gopkg.in/yaml.v3` | YAML parsing |
