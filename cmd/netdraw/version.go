package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var buildVersion = "dev"

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("netdraw %s\n", buildVersion)
		},
	})
}
