package compute

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/nimbodex/keyon/internal/compute/command"
)

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeStorage struct {
	mu     sync.Mutex
	data   map[string]string
	getErr error
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{data: make(map[string]string)}
}

func (s *fakeStorage) Set(k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
	return nil
}

func (s *fakeStorage) Get(k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return "", s.getErr
	}
	v, ok := s.data[k]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (s *fakeStorage) Del(k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
	return nil
}

type fakeWAL struct {
	mu      sync.Mutex
	calls   []command.Type
	err     error
}

func (w *fakeWAL) WriteCmd(typ command.Type, args []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.calls = append(w.calls, typ)
	return nil
}

func TestHandleSetNoWAL(t *testing.T) {
	storage := newFakeStorage()
	c := New(storage, nil, newLogger())

	res, err := c.Handle("SET k v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != "OK" {
		t.Fatalf("expected OK, got %q", res)
	}
	if storage.data["k"] != "v" {
		t.Fatalf("expected k=v, got %q", storage.data["k"])
	}
}

func TestHandleSetWithWAL(t *testing.T) {
	storage := newFakeStorage()
	wal := &fakeWAL{}
	c := New(storage, wal, newLogger())

	if _, err := c.Handle("SET k v"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if len(wal.calls) != 1 || wal.calls[0] != command.CmdSet {
		t.Fatalf("expected one SET in WAL, got %v", wal.calls)
	}
	if storage.data["k"] != "v" {
		t.Fatalf("storage not updated")
	}
}

func TestHandleDelWithWAL(t *testing.T) {
	storage := newFakeStorage()
	_ = storage.Set("k", "v")
	wal := &fakeWAL{}
	c := New(storage, wal, newLogger())

	if _, err := c.Handle("DEL k"); err != nil {
		t.Fatalf("del: %v", err)
	}
	if len(wal.calls) != 1 || wal.calls[0] != command.CmdDel {
		t.Fatalf("expected one DEL in WAL, got %v", wal.calls)
	}
	if _, ok := storage.data["k"]; ok {
		t.Fatal("expected k to be deleted")
	}
}

func TestHandleGetSkipsWAL(t *testing.T) {
	storage := newFakeStorage()
	_ = storage.Set("k", "v")
	wal := &fakeWAL{}
	c := New(storage, wal, newLogger())

	res, err := c.Handle("GET k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if res != "v" {
		t.Fatalf("expected v, got %q", res)
	}
	if len(wal.calls) != 0 {
		t.Fatalf("expected no WAL calls for GET, got %v", wal.calls)
	}
}

func TestReadOnlyRejectsWrites(t *testing.T) {
	storage := newFakeStorage()
	_ = storage.Set("k", "v")
	c := NewReadOnly(storage, newLogger())

	if _, err := c.Handle("SET k2 v2"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly on SET, got %v", err)
	}
	if _, err := c.Handle("DEL k"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly on DEL, got %v", err)
	}
	if _, ok := storage.data["k2"]; ok {
		t.Fatal("storage must not be modified on read-only SET")
	}
	if _, ok := storage.data["k"]; !ok {
		t.Fatal("storage must not be modified on read-only DEL")
	}

	res, err := c.Handle("GET k")
	if err != nil {
		t.Fatalf("read-only GET should work: %v", err)
	}
	if res != "v" {
		t.Fatalf("expected v, got %q", res)
	}
}

func TestHandleWALErrorSkipsStorage(t *testing.T) {
	storage := newFakeStorage()
	wal := &fakeWAL{err: errors.New("disk on fire")}
	c := New(storage, wal, newLogger())

	_, err := c.Handle("SET k v")
	if err == nil {
		t.Fatal("expected error from WAL")
	}
	if _, ok := storage.data["k"]; ok {
		t.Fatal("storage must remain untouched when WAL fails")
	}

	_, err = c.Handle("DEL anykey")
	if err == nil {
		t.Fatal("expected error from WAL on DEL")
	}
}
