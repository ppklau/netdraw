package main

import (
	"fmt"
	"os"

	"github.com/ppklau/netdraw/internal/validator"
	"github.com/spf13/cobra"
)


func init() {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate SoT and views.yml without rendering",
		Args:  cobra.NoArgs,
		RunE:  runValidate,
	}
	cmd.Flags().String("mode", "strict", "validation mode: strict or warn")
	rootCmd.AddCommand(cmd)
}

func runValidate(cmd *cobra.Command, _ []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	if mode != "strict" && mode != "warn" {
		return fmt.Errorf("--mode must be strict or warn, got %q", mode)
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	devices, sites, regions, physLinks, logLinks, err := loadSoT(cfg)
	if err != nil {
		return err
	}

	result := validator.Validate(devices, sites, regions, physLinks, logLinks)

	if len(result.Issues) == 0 {
		fmt.Printf("ok  %d devices, %d sites, %d regions, %d physical links, %d logical links\n",
			len(devices), len(sites), len(regions), len(physLinks), len(logLinks))
		return nil
	}

	label := "error"
	if mode == "warn" {
		label = "warn"
	}
	for _, issue := range result.Issues {
		fmt.Fprintf(os.Stderr, "%s: %s\n", label, issue.Message)
	}

	if mode == "strict" {
		return fmt.Errorf("validation failed: %d issue(s)", len(result.Issues))
	}

	fmt.Printf("ok  %d devices, %d sites, %d regions, %d physical links, %d logical links (%d warning(s))\n",
		len(devices), len(sites), len(regions), len(physLinks), len(logLinks), len(result.Issues))
	return nil
}
