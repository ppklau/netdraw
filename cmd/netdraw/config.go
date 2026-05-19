package main

import (
	"fmt"

	"github.com/ppklau/netdraw/internal/adapters"
	flatadapter "github.com/ppklau/netdraw/internal/adapters/flat"
	hldadapter "github.com/ppklau/netdraw/internal/adapters/hld"
	perdeviceadapter "github.com/ppklau/netdraw/internal/adapters/perdevice"
	"github.com/ppklau/netdraw/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.PersistentFlags().String("sot", "", "SoT root directory (overrides .netdraw.yml and NETDRAW_SOT)")
	rootCmd.PersistentFlags().String("adapter", "", "adapter type: flat | perdevice | hld (overrides .netdraw.yml and NETDRAW_ADAPTER)")
}

// loadConfig resolves the active configuration from flags, env, and .netdraw.yml.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	sot, _ := cmd.Flags().GetString("sot")
	adapter, _ := cmd.Flags().GetString("adapter")
	cfg.ApplyFlags(adapter, sot)
	return cfg, nil
}

// newAdapter constructs the adapter specified by cfg.
func newAdapter(cfg *config.Config) (adapters.Adapter, error) {
	switch cfg.Adapter {
	case "flat":
		return flatadapter.New(cfg.SoT), nil
	case "perdevice":
		return perdeviceadapter.New(cfg.SoT), nil
	case "hld":
		return hldadapter.New(cfg.SoT), nil
	default:
		return nil, fmt.Errorf("unknown adapter %q (supported: flat, perdevice, hld)", cfg.Adapter)
	}
}
