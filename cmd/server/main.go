package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nimbodex/keyon/internal/compute"
	"github.com/nimbodex/keyon/internal/config"
	"github.com/nimbodex/keyon/internal/network"
	"github.com/nimbodex/keyon/internal/storage/engine"
	"github.com/nimbodex/keyon/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, closer, err := logger.NewLogger(cfg.Logging.Level, cfg.Logging.Output)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() {
		if cerr := closer.Close(); cerr != nil {
			fmt.Fprintln(os.Stderr, "close logger:", cerr)
		}
	}()

	msgSize, err := config.ParseSize(cfg.Network.MaxMessageSize)
	if err != nil {
		return fmt.Errorf("parse max_message_size: %w", err)
	}

	eng := engine.NewEngine(log)
	comp := compute.New(eng, log)

	handler := func(req string) string {
		res, err := comp.Handle(req)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return res
	}

	srv := network.NewServer(network.Config{
		Address:        cfg.Network.Address,
		MaxConnections: cfg.Network.MaxConnections,
		MaxMessageSize: msgSize,
		IdleTimeout:    cfg.Network.IdleTimeout,
	}, handler, log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info("server starting", slog.String("addr", cfg.Network.Address))
	if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("server: %w", err)
	}
	log.Info("server stopped")
	return nil
}
