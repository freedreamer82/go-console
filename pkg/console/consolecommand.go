package console

type ConsoleCommandHandler func(console *Console, command *ConsoleCommand, args []string) CommandError

type ConsoleCommand struct {
	handler   ConsoleCommandHandler
	help      string
	cmd       string
	levelUser User
}

type CommandError string

const CMD_NOT_FOUND CommandError = "Command Not Found!"
const BAD_FORMAT CommandError = "Bad Format!"
const N0_ERR CommandError = ""

func NewConsoleCommand(cmd string, handler ConsoleCommandHandler, help string) *ConsoleCommand {
	c := ConsoleCommand{help: help, handler: handler, cmd: cmd, levelUser: Root}
	return &c
}

func (c *ConsoleCommand) GetCommand() string {
	return c.cmd
}

func (c *ConsoleCommand) GetHelp() string {
	return c.help
}

func (c *ConsoleCommand) GetUserLevel() User {
	return c.levelUser
}

func (c *ConsoleCommand) SetUserLevel(level User) {
	c.levelUser = level
}
