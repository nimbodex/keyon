package engine

import (
	"errors"
	"testing"

	"github.com/nimbodex/keyon/pkg/logger"
)

func TestEngine(t *testing.T) {
	l := logger.NewLogger(false)

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
