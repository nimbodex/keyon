package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadEmptyPathReturnsDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := Default()
	if cfg.Engine != def.Engine ||
		cfg.Network != def.Network ||
		cfg.Logging != def.Logging {
		t.Fatalf("expected defaults, got %+v", cfg)
	}
	if cfg.WAL != nil {
		t.Fatalf("expected WAL to be nil, got %+v", cfg.WAL)
	}
}

func TestLoadNoWALSection(t *testing.T) {
	path := writeTempConfig(t, `
engine:
  type: "in_memory"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WAL != nil {
		t.Fatalf("expected WAL nil, got %+v", cfg.WAL)
	}
}

func TestLoadEmptyWALSectionGetsDefaults(t *testing.T) {
	path := writeTempConfig(t, `
wal: {}
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WAL == nil {
		t.Fatal("expected WAL not nil")
	}
	def := DefaultWAL()
	if *cfg.WAL != def {
		t.Fatalf("expected %+v, got %+v", def, *cfg.WAL)
	}
}

func TestLoadPartialWALSection(t *testing.T) {
	path := writeTempConfig(t, `
wal:
  flushing_batch_size: 50
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WAL == nil {
		t.Fatal("expected WAL not nil")
	}
	def := DefaultWAL()
	if cfg.WAL.FlushingBatchSize != 50 {
		t.Fatalf("FlushingBatchSize: expected 50, got %d", cfg.WAL.FlushingBatchSize)
	}
	if cfg.WAL.FlushingBatchTimeout != def.FlushingBatchTimeout {
		t.Fatalf("FlushingBatchTimeout: expected %v, got %v", def.FlushingBatchTimeout, cfg.WAL.FlushingBatchTimeout)
	}
	if cfg.WAL.MaxSegmentSize != def.MaxSegmentSize {
		t.Fatalf("MaxSegmentSize: expected %v, got %v", def.MaxSegmentSize, cfg.WAL.MaxSegmentSize)
	}
	if cfg.WAL.DataDirectory != def.DataDirectory {
		t.Fatalf("DataDirectory: expected %v, got %v", def.DataDirectory, cfg.WAL.DataDirectory)
	}
}

func TestLoadFullWALSection(t *testing.T) {
	path := writeTempConfig(t, `
wal:
  flushing_batch_size: 100
  flushing_batch_timeout: "10ms"
  max_segment_size: "10MB"
  data_directory: "/data/spider/wal"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WAL == nil {
		t.Fatal("expected WAL not nil")
	}
	expected := WALConfig{
		FlushingBatchSize:    100,
		FlushingBatchTimeout: 10 * time.Millisecond,
		MaxSegmentSize:       "10MB",
		DataDirectory:        "/data/spider/wal",
	}
	if *cfg.WAL != expected {
		t.Fatalf("expected %+v, got %+v", expected, *cfg.WAL)
	}
}

func TestLoadBrokenYAML(t *testing.T) {
	path := writeTempConfig(t, "engine: [unterminated")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yml"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"512", 512, false},
		{"512B", 512, false},
		{"4KB", 4 * 1024, false},
		{"1MB", 1024 * 1024, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"2GB", 2 * 1024 * 1024 * 1024, false},
		{"4kb", 4 * 1024, false},
		{"abc", 0, true},
		{"", 0, true},
		{"-1KB", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseSize(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseSize(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSize(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSize(%q): expected %d, got %d", tc.in, tc.want, got)
		}
	}
}
