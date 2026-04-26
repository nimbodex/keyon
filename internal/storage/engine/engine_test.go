package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/nimbodex/keyon/pkg/logger"
)

func TestEngine(t *testing.T) {
	l := logger.Nop()

	t.Run("set and get", func(t *testing.T) {
		engine := NewEngine(l)
		err := engine.Set("key", "value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		val, err := engine.Get("key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "value" {
			t.Errorf("expected %q, got %q", "value", val)
		}
	})

	t.Run("get missing key", func(t *testing.T) {
		engine := NewEngine(l)
		_, err := engine.Get("key")
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("set del get", func(t *testing.T) {
		engine := NewEngine(l)
		err := engine.Set("key", "value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = engine.Del("key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = engine.Get("key")
		if !errors.Is(err, ErrKeyNotFound) {
			t.Fatalf("expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("set overwrite", func(t *testing.T) {
		engine := NewEngine(l)
		err := engine.Set("key", "value1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = engine.Set("key", "value2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		val, err := engine.Get("key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "value2" {
			t.Errorf("expected %q, got %q", "value2", val)
		}
	})

	t.Run("del missing key", func(t *testing.T) {
		engine := NewEngine(l)
		err := engine.Del("missing")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestEngineParallelAccess(t *testing.T) {
	var wg sync.WaitGroup

	l := logger.Nop()
	engine := NewEngine(l)

	wg.Add(100)

	for i := range 100 {
		go func(id int) {
			defer wg.Done()

			key := fmt.Sprintf("k-%d", id)
			err := engine.Set(key, fmt.Sprintf("v-%d", id))
			if err != nil {
				t.Errorf("k-%d: unexpected error: %v", id, err)
			}

			val, err := engine.Get(key)
			if err != nil {
				t.Errorf("k-%d: unexpected error: %v", id, err)
			}
			if want := fmt.Sprintf("v-%d", id); val != want {
				t.Errorf("k-%d: want %q, got %q", id, want, val)
			}

			if id%2 == 0 {
				if err := engine.Del(key); err != nil {
					t.Errorf("k-%d: del: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	for i := range 100 {
		key := fmt.Sprintf("k-%d", i)
		_, err := engine.Get(key)

		if i%2 == 0 {
			if !errors.Is(err, ErrKeyNotFound) {
				t.Errorf("k-%d: expected ErrKeyNotFound, got %v", i, err)
			}
		} else {
			if err != nil {
				t.Errorf("k-%d: expected nil error, got %v", i, err)
			}
		}
	}
}
