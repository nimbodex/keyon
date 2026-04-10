package command

type Type int

const (
	CmdSet Type = iota
	CmdGet
	CmdDel
)

type Command struct {
	Type Type
	Args []string // SET: [key, value]; GET/DEL: [key]
}
