package compute

import (
	"fmt"
	"log/slog"

	"github.com/nimbodex/keyon/internal/compute/command"
	"github.com/nimbodex/keyon/internal/compute/parser"
)

type Storage interface {
	Set(key, value string) error
	Get(key string) (string, error)
	Del(key string) error
}

type Compute interface {
	Handle(input string) (string, error)
}

type compute struct {
	storage Storage
	logger  *slog.Logger
}

func New(storage Storage, logger *slog.Logger) Compute {
	return &compute{storage: storage, logger: logger}
}

func (c *compute) Handle(input string) (string, error) {
	c.logger.Info("incoming request", slog.String("input", input))

	cmd, err := parser.Parse(input)
	if err != nil {
		c.logger.Error("parse error", slog.String("input", input), slog.String("error", err.Error()))
		return "", err
	}

	switch cmd.Type {
	case command.CmdSet:
		k, v := cmd.Args[0], cmd.Args[1]
		err := c.storage.Set(k, v)
		if err != nil {
			return "", err
		}
		return "OK", nil
	case command.CmdGet:
		val, err := c.storage.Get(cmd.Args[0])
		if err != nil {
			return "(nil)", err
		}
		return val, nil
	case command.CmdDel:
		err := c.storage.Del(cmd.Args[0])
		if err != nil {
			return "", err
		}
		return "OK", nil
	default:
		return "", fmt.Errorf("invalid request: %s", input)
	}
}
