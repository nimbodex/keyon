package config

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var errInvalidSizeFormat = errors.New("invalid size format, expected <number>[B|KB|MB|GB]")

var sizeFormatRegex = regexp.MustCompile(`(?i)^(\d+)\s*(B|KB|MB|GB)?$`)

type Config struct {
	Engine  EngineConfig  `yaml:"engine"`
	Network NetworkConfig `yaml:"network"`
	Logging LoggingConfig `yaml:"logging"`
	WAL     *WALConfig    `yaml:"wal"`
}

type EngineConfig struct {
	Type string `yaml:"type"`
}

type NetworkConfig struct {
	Address        string        `yaml:"address"`
	MaxConnections int           `yaml:"max_connections"`
	MaxMessageSize string        `yaml:"max_message_size"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

type WALConfig struct {
	FlushingBatchSize    int           `yaml:"flushing_batch_size"`
	FlushingBatchTimeout time.Duration `yaml:"flushing_batch_timeout"`
	MaxSegmentSize       string        `yaml:"max_segment_size"`
	DataDirectory        string        `yaml:"data_directory"`
}

func Default() Config {
	return Config{
		Engine: EngineConfig{
			Type: "in_memory",
		},
		Network: NetworkConfig{
			Address:        "127.0.0.1:3223",
			MaxConnections: 100,
			MaxMessageSize: "4KB",
			IdleTimeout:    5 * time.Minute,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Output: "stdout",
		},
	}
}

func DefaultWAL() WALConfig {
	return WALConfig{
		FlushingBatchSize:    100,
		FlushingBatchTimeout: 10 * time.Millisecond,
		MaxSegmentSize:       "10MB",
		DataDirectory:        "./data/wal",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, err
	}

	applyDefaults(&cfg)

	return cfg, nil
}

func ParseSize(s string) (int, error) {
	subs := sizeFormatRegex.FindStringSubmatch(s)

	if subs == nil {
		return 0, errInvalidSizeFormat
	}

	sizeStr := subs[1]
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return 0, err
	}

	unit := strings.ToUpper(subs[2])

	switch unit {
	case "B", "":
		return size, nil
	case "KB":
		return size * 1024, nil
	case "MB":
		return size * 1024 * 1024, nil
	case "GB":
		return size * 1024 * 1024 * 1024, nil
	default:
		return 0, errInvalidSizeFormat
	}
}

func applyDefaults(cfg *Config) {
	def := Default()

	if cfg.Engine.Type == "" {
		cfg.Engine.Type = def.Engine.Type
	}

	if cfg.Network.Address == "" {
		cfg.Network.Address = def.Network.Address
	}
	if cfg.Network.MaxConnections == 0 {
		cfg.Network.MaxConnections = def.Network.MaxConnections
	}
	if cfg.Network.MaxMessageSize == "" {
		cfg.Network.MaxMessageSize = def.Network.MaxMessageSize
	}
	if cfg.Network.IdleTimeout == 0 {
		cfg.Network.IdleTimeout = def.Network.IdleTimeout
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = def.Logging.Level
	}
	if cfg.Logging.Output == "" {
		cfg.Logging.Output = def.Logging.Output
	}

	if cfg.WAL != nil {
		walDef := DefaultWAL()
		if cfg.WAL.FlushingBatchSize == 0 {
			cfg.WAL.FlushingBatchSize = walDef.FlushingBatchSize
		}
		if cfg.WAL.FlushingBatchTimeout == 0 {
			cfg.WAL.FlushingBatchTimeout = walDef.FlushingBatchTimeout
		}
		if cfg.WAL.MaxSegmentSize == "" {
			cfg.WAL.MaxSegmentSize = walDef.MaxSegmentSize
		}
		if cfg.WAL.DataDirectory == "" {
			cfg.WAL.DataDirectory = walDef.DataDirectory
		}
	}
}
