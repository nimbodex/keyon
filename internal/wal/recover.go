package wal

import (
	"bufio"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nimbodex/keyon/internal/compute/command"
)

type Applier interface {
	Set(key, val string) error
	Del(key string) error
}

func Recover(dir string, applier Applier, logger *slog.Logger) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), segmentPrefix) && strings.HasSuffix(e.Name(), segmentSuffix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	totalEntries := 0
	for _, name := range names {
		applied, err := replayFile(filepath.Join(dir, name), applier, logger)
		if err != nil {
			return err
		}
		totalEntries += applied
	}

	logger.Info("wal recovery done",
		slog.Int("files", len(names)),
		slog.Int("entries", totalEntries),
	)
	return nil
}

func ReplayFile(path string, applier Applier, logger *slog.Logger) (int, error) {
	return replayFile(path, applier, logger)
}

func IsSegmentName(name string) bool {
	return strings.HasPrefix(name, segmentPrefix) && strings.HasSuffix(name, segmentSuffix)
}

func replayFile(path string, applier Applier, logger *slog.Logger) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	applied := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		entry, err := Decode(line)
		if err != nil {
			logger.Error("wal recover: skip broken line",
				slog.String("file", path),
				slog.String("line", string(line)),
				slog.Any("error", err),
			)
			continue
		}
		if err := apply(applier, entry); err != nil {
			logger.Error("wal recover: apply failed",
				slog.String("file", path),
				slog.Any("error", err),
			)
			continue
		}
		applied++
	}
	if err := scanner.Err(); err != nil {
		return applied, err
	}
	return applied, nil
}

func apply(applier Applier, e Entry) error {
	switch e.Type {
	case command.CmdSet:
		return applier.Set(e.Args[0], e.Args[1])
	case command.CmdDel:
		return applier.Del(e.Args[0])
	default:
		return errors.New("wal recover: unsupported entry type")
	}
}
