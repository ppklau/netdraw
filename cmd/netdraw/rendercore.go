package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ppklau/netdraw/internal/config"
	"github.com/ppklau/netdraw/internal/graph"
	"github.com/ppklau/netdraw/internal/hldoverlay"
	"github.com/ppklau/netdraw/internal/layout"
	d2r "github.com/ppklau/netdraw/internal/renderers/d2"
	drawior "github.com/ppklau/netdraw/internal/renderers/drawio"
	"github.com/ppklau/netdraw/internal/schema"
	"github.com/ppklau/netdraw/internal/views"
)

// resolveViewsPath returns the effective path to views.yml, with a SoT-root fallback.
func resolveViewsPath(cfg *config.Config) string {
	if _, err := os.Stat(cfg.Views); err == nil {
		return cfg.Views
	}
	candidate := filepath.Join(cfg.SoT, filepath.Base(cfg.Views))
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return cfg.Views
}

// loadSoT loads all adapter data and applies the hld.yml overlay if present.
// This is the canonical place to call when you need the full merged SoT.
func loadSoT(cfg *config.Config) (
	devices []schema.Device,
	sites []schema.Site,
	regions []schema.Region,
	physLinks []schema.PhysicalLink,
	logLinks []schema.LogicalLink,
	err error,
) {
	adapter, err := newAdapter(cfg)
	if err != nil {
		return
	}
	devices, err = adapter.Devices()
	if err != nil {
		return
	}
	sites, err = adapter.Sites()
	if err != nil {
		return
	}
	regions, err = adapter.Regions()
	if err != nil {
		return
	}
	physLinks, err = adapter.PhysicalLinks()
	if err != nil {
		return
	}
	logLinks, err = adapter.LogicalLinks()
	if err != nil {
		return
	}

	// Apply hld.yml overlay unless the base adapter is already the standalone hld adapter.
	if cfg.Adapter != "hld" {
		hldPath := filepath.Join(cfg.SoT, "hld.yml")
		var merged *hldoverlay.Result
		merged, err = hldoverlay.Merge(devices, sites, physLinks, hldPath)
		if err != nil {
			err = fmt.Errorf("hld overlay: %w", err)
			return
		}
		devices = merged.Devices
		sites = merged.Sites
		physLinks = merged.PhysicalLinks
	}
	return
}

// renderOnce loads SoT data and renders the requested view(s) to the given formats.
// Provide viewName="" + all=true to render every view, or viewName!="" + all=false for one view.
// formats must be a non-empty slice of "svg" and/or "drawio".
// Returns rendered (all written paths) and changed (paths where content differed from disk).
func renderOnce(ctx context.Context, cfg *config.Config, viewName string, all bool, formats []string) (rendered, changed []string, errs []error) {
	devices, sites, regions, physLinks, logLinks, err := loadSoT(cfg)
	if err != nil {
		return nil, nil, []error{err}
	}

	g := graph.Build(devices, sites, regions, physLinks, logLinks)

	viewsPath := resolveViewsPath(cfg)
	allViews, err := views.ParseFile(viewsPath)
	if err != nil {
		return nil, nil, []error{err}
	}

	var toRender []*views.View
	if all {
		for i := range allViews {
			toRender = append(toRender, &allViews[i])
		}
	} else {
		v, ok := views.Find(allViews, viewName)
		if !ok {
			return nil, nil, []error{fmt.Errorf("view %q not found in %s", viewName, viewsPath)}
		}
		toRender = []*views.View{v}
	}

	overridesPath := filepath.Join(filepath.Dir(viewsPath), "layout_overrides.yml")
	overrides, err := layout.LoadFile(overridesPath)
	if err != nil {
		return nil, nil, []error{fmt.Errorf("loading layout overrides: %w", err)}
	}

	if err := os.MkdirAll(cfg.Output, 0o755); err != nil {
		return nil, nil, []error{fmt.Errorf("creating output directory: %w", err)}
	}

	for _, v := range toRender {
		filtered := views.Filter(g, v)
		positions := overrides.ForView(v.Name)

		for _, format := range formats {
			switch format {
			case "svg":
				script := d2r.Script(filtered, v, positions)
				svg, err := d2r.RenderSVG(ctx, script)
				if err != nil {
					errs = append(errs, fmt.Errorf("rendering view %q as SVG: %w", v.Name, err))
					continue
				}
				outPath := filepath.Join(cfg.Output, v.Name+".svg")
				r, c, err := writeIfChanged(outPath, svg)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				rendered = append(rendered, r)
				changed = append(changed, c...)

			case "drawio":
				drawioPositions := positions
				if len(drawioPositions) == 0 {
					if dagPos, dagErr := d2r.PositionsFromLayout(ctx, filtered, v, drawior.PageWidth, drawior.PageHeight); dagErr == nil && len(dagPos) > 0 {
						drawioPositions = dagPos
					}
				}
				xmlBytes, err := drawior.XML(filtered, v, drawioPositions)
				if err != nil {
					errs = append(errs, fmt.Errorf("rendering view %q as draw.io: %w", v.Name, err))
					continue
				}
				outPath := filepath.Join(cfg.Output, v.Name+".drawio")
				r, c, err := writeIfChanged(outPath, xmlBytes)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				rendered = append(rendered, r)
				changed = append(changed, c...)

			default:
				errs = append(errs, fmt.Errorf("unknown format %q (supported: svg, drawio)", format))
			}
		}
	}

	return rendered, changed, errs
}

// writeIfChanged writes content to path and returns (path, [path if changed], error).
func writeIfChanged(path string, content []byte) (string, []string, error) {
	existing, _ := os.ReadFile(path)
	oldHash := sha256.Sum256(existing)
	newHash := sha256.Sum256(content)

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", nil, fmt.Errorf("writing %s: %w", path, err)
	}

	var changed []string
	if oldHash != newHash {
		changed = append(changed, path)
	}
	return path, changed, nil
}
