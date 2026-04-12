package command

import "slices"

type Type int

const (
	CmdSet Type = iota
	CmdGet
	CmdDel
)

type Command struct {
	Type Type
	Args []string
}

func NewCommand(typ Type, args []string) *Command {
	return &Command{
		Type: typ,
		Args: args,
	}
}

func (c *Command) Equal(other *Command) bool {
	return c.Type == other.Type && slices.Equal(c.Args, other.Args)
}
