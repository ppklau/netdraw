package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "netdraw",
	Short: "Network topology diagramming pipeline",
	Long: `NetDraw generates professional network diagrams from YAML source of truth files.

It sits above renderers (D2, draw.io) and provides:
  - A SoT-agnostic adapter framework
  - A network-native view language with detail levels and scope filters
  - SVG output that renders natively in GitHub
  - draw.io XML output for manual editing`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
