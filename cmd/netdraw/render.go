package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ppklau/netdraw/internal/config"
)

func init() {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render one or all views to SVG or draw.io format",
		Args:  cobra.NoArgs,
		RunE:  runRender,
	}

	cmd.Flags().StringP("view", "v", "", "view name to render")
	cmd.Flags().BoolP("all", "a", false, "render all views defined in views.yml")
	cmd.Flags().StringSliceP("format", "f", []string{"svg"}, "output format(s): svg, drawio")
	cmd.Flags().StringP("output", "o", "", "output directory (overrides .netdraw.yml)")
	cmd.Flags().StringP("sot", "s", "", "path to SoT root directory")
	cmd.Flags().String("adapter", "", "adapter type: flat (default)")
	cmd.MarkFlagsMutuallyExclusive("view", "all")

	rootCmd.AddCommand(cmd)
}

func runRender(cmd *cobra.Command, _ []string) error {
	viewName, _ := cmd.Flags().GetString("view")
	all, _ := cmd.Flags().GetBool("all")
	outputFlag, _ := cmd.Flags().GetString("output")
	sotFlag, _ := cmd.Flags().GetString("sot")
	adapterFlag, _ := cmd.Flags().GetString("adapter")

	if viewName == "" && !all {
		return fmt.Errorf("specify --view <name> or --all")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ApplyFlags(adapterFlag, sotFlag)
	if outputFlag != "" {
		cfg.Output = outputFlag
	}

	formats, _ := cmd.Flags().GetStringSlice("format")
	rendered, _, errs := renderOnce(context.Background(), cfg, viewName, all, formats)
	for _, path := range rendered {
		fmt.Printf("rendered: %s\n", path)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
