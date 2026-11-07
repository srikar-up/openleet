package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"openleet/internal/config"
	"openleet/internal/eventbus"
	"openleet/internal/storage"
	"openleet/internal/ui"
	"openleet/internal/ui/settings"
	"openleet/internal/ui/theme"
)

// resolveStartView picks the launch screen. An explicit --view flag always
// wins; otherwise the general.startup setting is used, defaulting to home.
func resolveStartView(flagValue string, cfg *config.Config) string {
	name := strings.ToLower(strings.TrimSpace(flagValue))
	if name != "" {
		return name
	}
	return strings.ToLower(strings.TrimSpace(
		settings.Value(cfg, settings.Field{ID: "general.startup", Default: "home"}),
	))
}

func main() {
	view := flag.String("view", "", "initial screen: home, roadmap, solve, stats or settings (default: general.startup setting)")
	settingsSection := flag.String("settings-section", "", "settings category to open: general, appearance, ai, data, ...")
	themeName := flag.String("theme", "", "override the configured theme for this run")
	listThemes := flag.Bool("list-themes", false, "print available themes and exit")
	flag.Parse()

	if *listThemes {
		for _, palette := range theme.All() {
			fmt.Printf("%-18s %s\n", palette.Name, palette.Description)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	startupTheme := cfg.Theme
	if *themeName != "" {
		palette, ok := theme.Get(*themeName)
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown theme %q. Available: %s\n", *themeName, strings.Join(theme.Names(), ", "))
			os.Exit(2)
		}
		// --theme is a per-run override. Pin the saved preference first so a
		// schema migration cannot accidentally persist the override.
		if cfg.Settings == nil {
			cfg.Settings = make(map[string]string)
		}
		if _, ok := cfg.Settings["appearance.theme"]; !ok && startupTheme != "" {
			cfg.Settings["appearance.theme"] = startupTheme
		}
		cfg.Theme = palette.Name
	}

	// Apply schema defaults and retired-model migrations before the coach and
	// workspace read their settings. Persist the version bump once so the
	// migration survives a crash before Settings is opened.
	previousVersion := cfg.SettingsVersion
	settings.EnsureDefaults(cfg)
	if cfg.SettingsVersion != previousVersion {
		// Keep --theme as a runtime override; only migrate the durable theme.
		persisted := *cfg
		if *themeName != "" {
			persisted.Theme = startupTheme
		}
		_ = persisted.Save()
	}

	storageMgr, err := storage.NewManager(cfg.DataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing storage manager: %v\n", err)
		os.Exit(1)
	}

	appModel := ui.NewAppModel(cfg, storageMgr)

	// The event bus is the runtime pump between background producers (provider
	// tokens, geometry probes, logs) and the Elm loop. Producers never touch
	// models; the pump is the only caller of Program.Send, so every state
	// mutation still happens sequentially inside Update.
	hub := eventbus.NewHub()
	appModel.AttachBus(hub)

	startView := resolveStartView(*view, cfg)
	switch startView {
	case "roadmap":
		appModel.StartAt(ui.ViewRoadmap)
	case "solve":
		appModel.StartAt(ui.ViewSolve)
	case "stats":
		appModel.StartAt(ui.ViewStats)
	case "themes":
		// The theme gallery moved into Settings > Appearance; keep the flag
		// as a deep link so old scripts and muscle memory still land there.
		appModel.StartAt(ui.ViewSettings)
		appModel.OpenSettingsSection("appearance")
	case "settings":
		appModel.StartAt(ui.ViewSettings)
		if *settingsSection != "" {
			appModel.OpenSettingsSection(*settingsSection)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := tea.NewProgram(
		appModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithFPS(60),
	)

	pump := eventbus.NewPump(p, hub.Subscribe(eventbus.DefaultQueueCapacity), eventbus.PumpOptions{
		FrameInterval: eventbus.DefaultFrameInterval,
	})
	pump.Start(ctx)

	// Bubble Tea v1 cannot report window resizes on Windows at all, so the
	// geometry watcher polls the console buffer and publishes WindowSizeMsg
	// through the bus. On Unix SIGWINCH already covers this.
	var watcher *eventbus.Watcher
	if eventbus.NeedsResizePolling {
		watcher = eventbus.NewWatcher(hub, eventbus.WatchOptions{})
		watcher.Start(ctx)
	}

	_, runErr := p.Run()

	// Drain the pipeline before returning: cancel stops the loops, then we wait
	// so no goroutine outlives the terminal state they were writing to.
	cancel()
	pump.Wait()
	if watcher != nil {
		watcher.Wait()
	}
	hub.Close()
	appModel.Close()

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "OpenLeet terminated with error: %v\n", runErr)
		os.Exit(1)
	}
}
