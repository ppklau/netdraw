package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ppklau/netdraw/internal/config"
	"github.com/ppklau/netdraw/internal/layout"
	drawior "github.com/ppklau/netdraw/internal/renderers/drawio"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "extract <file.drawio>",
		Short: "Extract node positions from a draw.io file into layout_overrides.yml",
		Args:  cobra.ExactArgs(1),
		RunE:  runExtract,
	})
}

func runExtract(_ *cobra.Command, args []string) error {
	drawioPath := args[0]

	data, err := os.ReadFile(drawioPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", drawioPath, err)
	}

	positions, err := drawior.ExtractPositions(data)
	if err != nil {
		return fmt.Errorf("extracting positions from %s: %w", drawioPath, err)
	}

	// Derive view name from filename ("wan-overview.drawio" → "wan-overview")
	viewName := strings.TrimSuffix(filepath.Base(drawioPath), ".drawio")

	// Locate layout_overrides.yml via project config (same directory as views.yml).
	// Falls back to the parent of the draw.io file's directory.
	overridesPath := locateOverrides(drawioPath)

	overrides, err := layout.LoadFile(overridesPath)
	if err != nil {
		return fmt.Errorf("loading layout overrides: %w", err)
	}

	overrides.SetForView(viewName, positions)

	if err := layout.SaveFile(overridesPath, overrides); err != nil {
		return fmt.Errorf("saving layout overrides: %w", err)
	}

	fmt.Printf("extracted %d positions for view %q → %s\n", len(positions), viewName, overridesPath)
	return nil
}

// locateOverrides finds layout_overrides.yml by loading the project config (which
// gives the views.yml path) and placing the overrides file beside it. Falls back
// to the parent directory of the draw.io file.
func locateOverrides(drawioPath string) string {
	cfg, err := config.Load()
	if err == nil {
		viewsPath := resolveViewsPath(cfg)
		return filepath.Join(filepath.Dir(viewsPath), "layout_overrides.yml")
	}
	// Fallback: assume the draw.io file is in diagrams/ and overrides live one level up
	return filepath.Join(filepath.Dir(filepath.Dir(drawioPath)), "layout_overrides.yml")
}
