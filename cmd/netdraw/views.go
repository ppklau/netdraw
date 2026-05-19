package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/ppklau/netdraw/internal/views"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "views",
		Short: "List all views defined in views.yml",
		Args:  cobra.NoArgs,
		RunE:  runViews,
	})
}

func runViews(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	// cfg.Views is absolute when loaded from .netdraw.yml; otherwise fall back
	// to views.yml in the SoT root so `--sot` works without a config file.
	viewsPath := cfg.Views
	if !filepath.IsAbs(viewsPath) {
		viewsPath = filepath.Join(cfg.SoT, viewsPath)
	}
	vs, err := views.ParseFile(viewsPath)
	if err != nil {
		return fmt.Errorf("loading views: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLEVEL\tSCOPE")
	for _, v := range vs {
		fmt.Fprintf(w, "%s\t%s\t%s\n", v.Name, v.DetailLevel, scopeSummary(v))
	}
	return w.Flush()
}

func scopeSummary(v views.View) string {
	s := v.Scope
	if s.FocusSite != "" {
		return "focus: " + s.FocusSite
	}
	if len(s.Sites) > 0 {
		return "sites: " + strings.Join(s.Sites, ", ")
	}
	if len(s.CollapseSites) > 0 {
		return "collapse: " + strings.Join(s.CollapseSites, ", ")
	}
	return "all"
}
