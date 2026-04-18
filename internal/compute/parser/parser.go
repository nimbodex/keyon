package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nimbodex/keyon/internal/compute/command"
)

var (
	ErrEmptyInput      = errors.New("empty input")
	ErrUnknownCommand  = errors.New("unknown command")
	ErrWrongNumArgs    = errors.New("wrong number of arguments")
	ErrInvalidArgument = errors.New("invalid argument")
)

var validArgRegex = regexp.MustCompile(`^\w+$`)

func Parse(input string) (command.Command, error) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return command.Command{}, ErrEmptyInput
	}

	args := tokens[1:]

	var expectedArgs int
	var cmdType command.Type

	switch tokens[0] {
	case "SET":
		cmdType = command.CmdSet
		expectedArgs = 2
	case "GET":
		cmdType = command.CmdGet
		expectedArgs = 1
	case "DEL":
		cmdType = command.CmdDel
		expectedArgs = 1
	default:
		return command.Command{}, ErrUnknownCommand
	}

	if len(args) != expectedArgs {
		return command.Command{}, ErrWrongNumArgs
	}

	for _, arg := range args {
		if !validArgRegex.MatchString(arg) {
			return command.Command{}, fmt.Errorf("%w: %s", ErrInvalidArgument, arg)
		}
	}

	return command.Command{
		Type: cmdType,
		Args: args,
	}, nil
}
