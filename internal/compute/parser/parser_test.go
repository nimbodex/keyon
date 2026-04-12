package parser

import (
	"errors"
	"testing"

	"github.com/nimbodex/keyon/internal/compute/command"
)

func TestParse(t *testing.T) {
	t.Run("Valid SET command", func(t *testing.T) {
		expected := command.NewCommand(command.CmdSet, []string{"key", "value"})
		cmd, err := Parse("SET key value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cmd.Equal(expected) {
			t.Errorf("expected %+v, got %+v", expected, cmd)
		}
	})

	t.Run("Valid GET command", func(t *testing.T) {
		expected := command.NewCommand(command.CmdGet, []string{"key"})
		cmd, err := Parse("GET key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cmd.Equal(expected) {
			t.Errorf("expected %+v, got %+v", expected, cmd)
		}
	})

	t.Run("Valid DEL command", func(t *testing.T) {
		expected := command.NewCommand(command.CmdDel, []string{"key"})
		cmd, err := Parse("DEL key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cmd.Equal(expected) {
			t.Errorf("expected %+v, got %+v", expected, cmd)
		}
	})

	t.Run("SET command with whitespaces", func(t *testing.T) {
		expected := command.NewCommand(command.CmdSet, []string{"key", "value"})
		cmd, err := Parse(" SET  key  value ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !cmd.Equal(expected) {
			t.Errorf("expected %+v, got %+v", expected, cmd)
		}
	})

	t.Run("Unknown SET command", func(t *testing.T) {
		_, err := Parse("set k v")
		if !errors.Is(err, ErrUnknownCommand) {
			t.Fatalf("expected ErrUnknownCommand, got %v", err)
		}
	})

	t.Run("SET command with not enough arguments", func(t *testing.T) {
		_, err := Parse("SET k")
		if !errors.Is(err, ErrWrongNumArgs) {
			t.Fatalf("expected ErrWrongNumArgs, got %v", err)
		}
	})

	t.Run("SET command with extra arguments", func(t *testing.T) {
		_, err := Parse("SET k v extra")
		if !errors.Is(err, ErrWrongNumArgs) {
			t.Fatalf("expected ErrWrongNumArgs, got %v", err)
		}
	})

	t.Run("SET command with invalid arguments", func(t *testing.T) {
		_, err := Parse("GET k!")
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("expected ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("Empty input", func(t *testing.T) {
		_, err := Parse("")
		if !errors.Is(err, ErrEmptyInput) {
			t.Fatalf("expected ErrEmptyInput, got %v", err)
		}

		_, err = Parse(" ")
		if !errors.Is(err, ErrEmptyInput) {
			t.Fatalf("expected ErrEmptyInput, got %v", err)
		}
	})
}
