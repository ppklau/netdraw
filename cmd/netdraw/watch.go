package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	"github.com/ppklau/netdraw/internal/config"
)

func init() {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch YAML files and re-render on change",
		Args:  cobra.NoArgs,
		RunE:  runWatch,
	}

	cmd.Flags().StringP("view", "v", "", "view name to watch")
	cmd.Flags().BoolP("all", "a", false, "watch all views defined in views.yml")
	cmd.Flags().StringSliceP("format", "f", []string{"svg"}, "output format(s): svg, drawio")
	cmd.Flags().StringP("output", "o", "", "output directory (overrides .netdraw.yml)")
	cmd.Flags().StringP("sot", "s", "", "path to SoT root directory")
	cmd.Flags().String("adapter", "", "adapter type: flat (default)")
	cmd.MarkFlagsMutuallyExclusive("view", "all")

	rootCmd.AddCommand(cmd)
}

func runWatch(cmd *cobra.Command, _ []string) error {
	viewName, _ := cmd.Flags().GetString("view")
	all, _ := cmd.Flags().GetBool("all")
	formats, _ := cmd.Flags().GetStringSlice("format")
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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer watcher.Close()

	if err := watchDirRecursive(watcher, cfg.SoT); err != nil {
		return fmt.Errorf("watching SoT %s: %w", cfg.SoT, err)
	}

	// Watch views.yml parent directory if it's not already under the SoT root.
	viewsPath := resolveViewsPath(cfg)
	viewsDir := filepath.Dir(viewsPath)
	if !strings.HasPrefix(viewsDir, cfg.SoT) {
		if err := watcher.Add(viewsDir); err != nil {
			return fmt.Errorf("watching views directory %s: %w", viewsDir, err)
		}
	}

	ctx := context.Background()

	fmt.Println("netdraw watch — initial render...")
	doRender(ctx, cfg, viewName, all, formats, true)
	fmt.Printf("watching %s (Ctrl+C to stop)\n", cfg.SoT)

	const debounce = 200 * time.Millisecond
	var timer *time.Timer
	trigger := make(chan struct{}, 1)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			fmt.Println("\nstopping")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isYAMLChange(event) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				select {
				case trigger <- struct{}{}:
				default:
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-trigger:
			doRender(ctx, cfg, viewName, all, formats, false)
		}
	}
}

// doRender runs renderOnce and prints results. If printAll is true, prints all
// rendered paths (initial run); otherwise prints only paths whose content changed.
func doRender(ctx context.Context, cfg *config.Config, viewName string, all bool, formats []string, printAll bool) {
	rendered, changed, errs := renderOnce(ctx, cfg, viewName, all, formats)

	paths := changed
	if printAll {
		paths = rendered
	}
	for _, p := range paths {
		fmt.Printf("rendered: %s\n", p)
	}
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "error: %v\n", e)
	}
}

func isYAMLChange(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return false
	}
	name := event.Name
	return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
}

func watchDirRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}
