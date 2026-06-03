package replication

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nimbodex/keyon/internal/network"
	"github.com/nimbodex/keyon/internal/storage/engine"
	"github.com/nimbodex/keyon/pkg/logger"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func startMasterServer(t *testing.T, dir string, current func() string) string {
	t.Helper()
	addr := freePort(t)
	master := NewMaster(dir, current, logger.Nop())
	srv := network.NewServer(network.Config{
		Address:        addr,
		MaxMessageSize: 1 << 20,
	}, master.Handle, logger.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return addr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("master server did not start on %s", addr)
	return ""
}

func newSlave(t *testing.T, addr, dir string, eng *engine.Engine) *Slave {
	t.Helper()
	s, err := NewSlave(SlaveConfig{
		MasterAddress: addr,
		DataDir:       dir,
		SyncInterval:  10 * time.Millisecond,
		DialTimeout:   time.Second,
	}, eng, logger.Nop())
	if err != nil {
		t.Fatalf("new slave: %v", err)
	}
	return s
}

func TestSlaveSyncFetchesAndApplies(t *testing.T) {
	masterDir := t.TempDir()
	writeSegment(t, masterDir, "wal_1.log", "SET a 1\n")
	writeSegment(t, masterDir, "wal_2.log", "SET b 2\nDEL a\n")
	addr := startMasterServer(t, masterDir, nil)

	slaveDir := t.TempDir()
	eng := engine.NewEngine(logger.Nop())
	slave := newSlave(t, addr, slaveDir, eng)

	if err := slave.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if _, err := eng.Get("a"); err == nil {
		t.Fatal("expected a to be deleted")
	}
	if v, err := eng.Get("b"); err != nil || v != "2" {
		t.Fatalf("expected b=2, got %q (err=%v)", v, err)
	}

	for _, name := range []string{"wal_1.log", "wal_2.log"} {
		if _, err := os.Stat(filepath.Join(slaveDir, name)); err != nil {
			t.Fatalf("expected %s persisted on slave: %v", name, err)
		}
	}
}

func TestSlaveSkipsCurrentSegment(t *testing.T) {
	masterDir := t.TempDir()
	writeSegment(t, masterDir, "wal_1.log", "SET a 1\n")
	writeSegment(t, masterDir, "wal_2.log", "SET b 2\n")
	addr := startMasterServer(t, masterDir, func() string { return "wal_2.log" })

	slaveDir := t.TempDir()
	eng := engine.NewEngine(logger.Nop())
	slave := newSlave(t, addr, slaveDir, eng)

	if err := slave.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if v, err := eng.Get("a"); err != nil || v != "1" {
		t.Fatalf("expected a=1, got %q (err=%v)", v, err)
	}
	if _, err := eng.Get("b"); err == nil {
		t.Fatal("active segment wal_2.log must not be replicated")
	}
	if _, err := os.Stat(filepath.Join(slaveDir, "wal_2.log")); !os.IsNotExist(err) {
		t.Fatal("active segment must not be persisted on slave")
	}
}

func TestSlaveRestartSkipsAlreadyApplied(t *testing.T) {
	masterDir := t.TempDir()
	writeSegment(t, masterDir, "wal_1.log", "SET a 1\n")
	writeSegment(t, masterDir, "wal_2.log", "SET b 2\n")
	addr := startMasterServer(t, masterDir, nil)

	slaveDir := t.TempDir()
	writeSegment(t, slaveDir, "wal_1.log", "SET a 1\n")

	eng := engine.NewEngine(logger.Nop())
	slave := newSlave(t, addr, slaveDir, eng)

	if err := slave.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	if _, err := eng.Get("a"); err == nil {
		t.Fatal("wal_1.log already present locally must not be re-applied")
	}
	if v, err := eng.Get("b"); err != nil || v != "2" {
		t.Fatalf("expected b=2, got %q (err=%v)", v, err)
	}
}

func TestSlaveSyncIdempotent(t *testing.T) {
	masterDir := t.TempDir()
	writeSegment(t, masterDir, "wal_1.log", "SET a 1\n")
	addr := startMasterServer(t, masterDir, nil)

	slaveDir := t.TempDir()
	eng := engine.NewEngine(logger.Nop())
	slave := newSlave(t, addr, slaveDir, eng)

	for range 3 {
		if err := slave.Sync(context.Background()); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}
	if v, err := eng.Get("a"); err != nil || v != "1" {
		t.Fatalf("expected a=1, got %q (err=%v)", v, err)
	}
}

func TestSlaveRunStopsOnContext(t *testing.T) {
	masterDir := t.TempDir()
	writeSegment(t, masterDir, "wal_1.log", "SET a 1\n")
	addr := startMasterServer(t, masterDir, nil)

	slaveDir := t.TempDir()
	eng := engine.NewEngine(logger.Nop())
	slave := newSlave(t, addr, slaveDir, eng)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- slave.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if v, err := eng.Get("a"); err == nil && v == "1" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("slave did not replicate within deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("run returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestSlaveSyncDialError(t *testing.T) {
	slaveDir := t.TempDir()
	eng := engine.NewEngine(logger.Nop())
	slave, err := NewSlave(SlaveConfig{
		MasterAddress: freePort(t),
		DataDir:       slaveDir,
		SyncInterval:  10 * time.Millisecond,
		DialTimeout:   200 * time.Millisecond,
	}, eng, logger.Nop())
	if err != nil {
		t.Fatalf("new slave: %v", err)
	}

	if err := slave.Sync(context.Background()); err == nil {
		t.Fatal("expected dial error when master is down")
	}
}
