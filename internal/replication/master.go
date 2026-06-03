package replication

import (
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nimbodex/keyon/internal/wal"
)

type Master struct {
	dataDir string
	current func() string
	logger  *slog.Logger
}

func NewMaster(dataDir string, current func() string, logger *slog.Logger) *Master {
	if current == nil {
		current = func() string { return "" }
	}
	return &Master{
		dataDir: dataDir,
		current: current,
		logger:  logger,
	}
}

func (m *Master) Handle(req string) string {
	fields := strings.Fields(req)
	if len(fields) == 0 {
		return respError + " empty request"
	}

	switch fields[0] {
	case cmdList:
		return m.handleList()
	case cmdGet:
		if len(fields) != 2 {
			return respError + " GET requires a segment name"
		}
		return m.handleGet(fields[1])
	default:
		return respError + " unknown command"
	}
}

func (m *Master) handleList() string {
	entries, err := os.ReadDir(m.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return respOK
		}
		m.logger.Error("replication list", slog.Any("error", err))
		return respError + " cannot list segments"
	}

	current := m.current()
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !wal.IsSegmentName(name) {
			continue
		}
		if name == current {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return respOK
	}
	return respOK + " " + strings.Join(names, " ")
}

func (m *Master) handleGet(name string) string {
	if !wal.IsSegmentName(name) || strings.ContainsAny(name, "/\\") {
		return respError + " invalid segment name"
	}

	data, err := os.ReadFile(filepath.Join(m.dataDir, name))
	if err != nil {
		m.logger.Error("replication get", slog.String("segment", name), slog.Any("error", err))
		return respError + " cannot read segment"
	}

	return respOK + " " + base64.StdEncoding.EncodeToString(data)
}
