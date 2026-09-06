package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/sirupsen/logrus"
    "raven2/internal/config"
    "raven2/internal/database"
    "raven2/internal/metrics"
    "raven2/internal/monitoring"
    "raven2/internal/web"
)

func main() {
    configFile := flag.String("config", "config.yaml", "Configuration file path")
    version := flag.Bool("version", false, "Show version information")
    testConfig := flag.Bool("test", false, "Load and validate the configuration, then exit (0 = ok, 1 = error). Does not start the daemon or touch the database.")
    flag.Parse()

    if *version {
        fmt.Printf("Raven Network Monitoring v2.0.0\nBuild: %s\n", getBuildInfo())
        os.Exit(0)
    }

    // Load configuration
    cfg, err := config.Load(*configFile)
    if err != nil {
        if *testConfig {
            fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
            os.Exit(1)
        }
        logrus.Fatalf("Failed to load config: %v", err)
    }

    if *testConfig {
        if errs := monitoring.ValidateConfig(cfg); len(errs) > 0 {
            for _, e := range errs {
                fmt.Fprintf(os.Stderr, "Configuration error: %v\n", e)
            }
            os.Exit(1)
        }
        fmt.Printf("Configuration OK: %d host(s), %d check(s)\n", len(cfg.Hosts), len(cfg.Checks))
        os.Exit(0)
    }

    // Setup logging
    setupLogging(cfg.Logging)

    logrus.WithFields(logrus.Fields{
        "config_file": *configFile,
        "port":        cfg.Server.Port,
        "workers":     cfg.Server.Workers,
    }).Info("Starting Raven monitoring system")

    // Initialize database
    store, err := database.NewExtendedBoltStore(cfg.Database.Path)
    if err != nil {
        logrus.Fatalf("Failed to initialize database: %v", err)
    }
    defer store.Close()

    // Initialize metrics
    metricsCollector := metrics.NewCollector(store)

    // Initialize monitoring engine
    engine, err := monitoring.NewEngine(cfg, store, metricsCollector)
    if err != nil {
        logrus.Fatalf("Failed to initialize monitoring engine: %v", err)
    }

    // Initialize web server
    webServer := web.NewServer(cfg, store, engine, metricsCollector)

    // Start services
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start monitoring engine
    go engine.Start(ctx)

    // Start web server
    go webServer.Start(ctx)

    // Wait for shutdown or reload signals. SIGHUP is on its own channel and
    // handled in-process (RefreshConfigWithPurge) rather than left to its
    // default disposition, which terminates the process - and since
    // systemd treats SIGHUP as a "clean" signal, Restart=on-failure won't
    // even bring it back up afterwards.
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    hupChan := make(chan os.Signal, 1)
    signal.Notify(hupChan, syscall.SIGHUP)

waitLoop:
    for {
        select {
        case sig := <-sigChan:
            logrus.WithField("signal", sig).Info("Received shutdown signal")
            break waitLoop
        case <-hupChan:
            logrus.Info("Received SIGHUP, reloading configuration")
            if err := engine.RefreshConfigWithPurge(); err != nil {
                logrus.WithError(err).Error("Configuration reload failed")
            } else {
                logrus.Info("Configuration reloaded")
            }
        }
    }

    // Graceful shutdown: stop background loops first, then let the engine
    // and web server drain synchronously rather than exiting after a fixed
    // sleep and hoping they finished.
    cancel()

    engine.Stop()

    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()
    if err := webServer.Stop(shutdownCtx); err != nil {
        logrus.WithError(err).Error("Error during web server shutdown")
    }

    logrus.Info("Shutdown complete")
}

func setupLogging(cfg config.LoggingConfig) {
    level, err := logrus.ParseLevel(cfg.Level)
    if err != nil {
        level = logrus.InfoLevel
    }
    logrus.SetLevel(level)

    // systemd sets JOURNAL_STREAM when a unit's stdout/stderr go straight
    // to the journal (systemd.exec(5)), and journald stamps every line it
    // receives itself, so our own timestamp would just double up with the
    // one journalctl already shows.
    underJournal := os.Getenv("JOURNAL_STREAM") != ""

    if cfg.Format == "json" {
        logrus.SetFormatter(&logrus.JSONFormatter{
            DisableTimestamp: underJournal,
        })
    } else {
        logrus.SetFormatter(&logrus.TextFormatter{
            FullTimestamp:    true,
            DisableTimestamp: underJournal,
        })
    }
}

func getBuildInfo() string {
    return "dev-build" // This would be replaced by build system
}

