package replication

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nimbodex/keyon/internal/network"
	"github.com/nimbodex/keyon/internal/wal"
)

type SlaveConfig struct {
	MasterAddress string
	DataDir       string
	SyncInterval  time.Duration
	DialTimeout   time.Duration
}

type Slave struct {
	cfg     SlaveConfig
	applier wal.Applier
	logger  *slog.Logger

	applied map[string]struct{}
}

func NewSlave(cfg SlaveConfig, applier wal.Applier, logger *slog.Logger) (*Slave, error) {
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = time.Second
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 30 * time.Second
	}

	s := &Slave{
		cfg:     cfg,
		applier: applier,
		logger:  logger,
		applied: make(map[string]struct{}),
	}

	if err := s.loadApplied(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Slave) loadApplied() error {
	entries, err := os.ReadDir(s.cfg.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if wal.IsSegmentName(e.Name()) {
			s.applied[e.Name()] = struct{}{}
		}
	}
	return nil
}

func (s *Slave) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.SyncInterval)
	defer ticker.Stop()

	if err := s.Sync(ctx); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("replication sync", slog.Any("error", err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.Sync(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("replication sync", slog.Any("error", err))
			}
		}
	}
}

func (s *Slave) Sync(ctx context.Context) error {
	client, err := network.Dial(s.cfg.MasterAddress, s.cfg.DialTimeout)
	if err != nil {
		return fmt.Errorf("dial master: %w", err)
	}
	defer client.Close()

	names, err := s.requestList(client)
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, ok := s.applied[name]; ok {
			continue
		}
		if err := s.fetchSegment(client, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Slave) requestList(client *network.Client) ([]string, error) {
	resp, err := client.Send(cmdList)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	payload, err := parseResponse(resp)
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, nil
	}
	return strings.Fields(payload), nil
}

func (s *Slave) fetchSegment(client *network.Client, name string) error {
	resp, err := client.Send(cmdGet + " " + name)
	if err != nil {
		return fmt.Errorf("get segment %s: %w", name, err)
	}
	payload, err := parseResponse(resp)
	if err != nil {
		return fmt.Errorf("get segment %s: %w", name, err)
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("decode segment %s: %w", name, err)
	}

	if err := s.persistAndApply(name, data); err != nil {
		return err
	}

	s.applied[name] = struct{}{}
	s.logger.Info("replicated segment", slog.String("segment", name), slog.Int("bytes", len(data)))
	return nil
}

func (s *Slave) persistAndApply(name string, data []byte) error {
	if err := os.MkdirAll(s.cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	path := filepath.Join(s.cfg.DataDir, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write segment %s: %w", name, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename segment %s: %w", name, err)
	}

	if _, err := wal.ReplayFile(path, s.applier, s.logger); err != nil {
		return fmt.Errorf("replay segment %s: %w", name, err)
	}
	return nil
}

func parseResponse(resp string) (string, error) {
	if msg, ok := strings.CutPrefix(resp, respError); ok {
		return "", errors.New(strings.TrimSpace(msg))
	}
	if resp == respOK {
		return "", nil
	}
	if payload, ok := strings.CutPrefix(resp, respOK+" "); ok {
		return payload, nil
	}
	return "", fmt.Errorf("unexpected response: %q", resp)
}
