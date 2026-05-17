package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/nimbodex/keyon/internal/compute"
	"github.com/nimbodex/keyon/internal/config"
	"github.com/nimbodex/keyon/internal/network"
	"github.com/nimbodex/keyon/internal/storage/engine"
	"github.com/nimbodex/keyon/internal/wal"
	"github.com/nimbodex/keyon/pkg/logger"
)

func Run(ctx context.Context, configPath string) error {
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

	var walInstance compute.WAL
	if cfg.WAL != nil {
		maxSeg, err := config.ParseSize(cfg.WAL.MaxSegmentSize)
		if err != nil {
			return fmt.Errorf("parse max_segment_size: %w", err)
		}

		if err := wal.Recover(cfg.WAL.DataDirectory, eng, log); err != nil {
			return fmt.Errorf("wal recover: %w", err)
		}

		w, err := wal.New(wal.Config{
			DataDir:        cfg.WAL.DataDirectory,
			BatchSize:      cfg.WAL.FlushingBatchSize,
			BatchTimeout:   cfg.WAL.FlushingBatchTimeout,
			MaxSegmentSize: int64(maxSeg),
		}, log)
		if err != nil {
			return fmt.Errorf("wal init: %w", err)
		}
		defer func() {
			if cerr := w.Close(); cerr != nil {
				log.Error("wal close", slog.Any("error", cerr))
			}
		}()
		walInstance = w
	}

	comp := compute.New(eng, walInstance, log)

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

	log.Info("server starting", slog.String("addr", cfg.Network.Address))
	if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("server: %w", err)
	}
	log.Info("server stopped")
	return nil
}
