package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/nimbodex/keyon/internal/compute"
	"github.com/nimbodex/keyon/internal/config"
	"github.com/nimbodex/keyon/internal/network"
	"github.com/nimbodex/keyon/internal/replication"
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

	isSlave := cfg.Replication != nil && cfg.Replication.ReplicaType == config.ReplicaTypeSlave
	isMaster := cfg.Replication != nil && cfg.Replication.ReplicaType == config.ReplicaTypeMaster
	if cfg.Replication != nil && !isSlave && !isMaster {
		return fmt.Errorf("invalid replica_type: %q", cfg.Replication.ReplicaType)
	}

	eng := engine.NewEngine(log)

	dataDir := config.DefaultWAL().DataDirectory
	if cfg.WAL != nil {
		dataDir = cfg.WAL.DataDirectory
	}

	if cfg.WAL != nil || isSlave {
		if err := wal.Recover(dataDir, eng, log); err != nil {
			return fmt.Errorf("wal recover: %w", err)
		}
	}

	var comp compute.Compute
	var currentSegment func() string
	if isSlave {
		comp = compute.NewReadOnly(eng, log)
	} else {
		var walInstance compute.WAL
		if cfg.WAL != nil {
			maxSeg, err := config.ParseSize(cfg.WAL.MaxSegmentSize)
			if err != nil {
				return fmt.Errorf("parse max_segment_size: %w", err)
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
			currentSegment = w.CurrentSegment
		}
		comp = compute.New(eng, walInstance, log)
	}

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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	if isMaster {
		startMaster(ctx, &wg, cancel, cfg, dataDir, currentSegment, msgSize, log)
	}

	if isSlave {
		if err := startSlave(ctx, &wg, cancel, cfg, dataDir, eng, log); err != nil {
			cancel()
			wg.Wait()
			return err
		}
	}

	log.Info("server starting", slog.String("addr", cfg.Network.Address))
	srvErr := srv.Start(ctx)
	cancel()
	wg.Wait()
	if srvErr != nil && !errors.Is(srvErr, context.Canceled) {
		return fmt.Errorf("server: %w", srvErr)
	}
	log.Info("server stopped")
	return nil
}

func startMaster(
	ctx context.Context,
	wg *sync.WaitGroup,
	cancel context.CancelFunc,
	cfg config.Config,
	dataDir string,
	current func() string,
	msgSize int,
	log *slog.Logger,
) {
	master := replication.NewMaster(dataDir, current, log)
	replSrv := network.NewServer(network.Config{
		Address:        cfg.Replication.MasterAddress,
		MaxConnections: cfg.Network.MaxConnections,
		MaxMessageSize: msgSize,
		IdleTimeout:    cfg.Network.IdleTimeout,
	}, master.Handle, log)

	log.Info("replication master starting", slog.String("addr", cfg.Replication.MasterAddress))
	wg.Go(func() {
		if err := replSrv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("replication master", slog.Any("error", err))
			cancel()
		}
	})
}

func startSlave(
	ctx context.Context,
	wg *sync.WaitGroup,
	cancel context.CancelFunc,
	cfg config.Config,
	dataDir string,
	eng *engine.Engine,
	log *slog.Logger,
) error {
	slave, err := replication.NewSlave(replication.SlaveConfig{
		MasterAddress: cfg.Replication.MasterAddress,
		DataDir:       dataDir,
		SyncInterval:  cfg.Replication.SyncInterval,
	}, eng, log)
	if err != nil {
		return fmt.Errorf("init replication slave: %w", err)
	}

	log.Info("replication slave starting",
		slog.String("master", cfg.Replication.MasterAddress),
		slog.Duration("interval", cfg.Replication.SyncInterval),
	)
	wg.Go(func() {
		if err := slave.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("replication slave", slog.Any("error", err))
			cancel()
		}
	})
	return nil
}
