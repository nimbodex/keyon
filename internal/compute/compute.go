package compute

type CommandType int

const (
	CmdSet CommandType = iota
	CmdGet
	CmdDel
)

type Command struct {
	Type CommandType
	Args []string // SET: [key, value]; GET/DEL: [key]
}
