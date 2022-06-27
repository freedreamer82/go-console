package console

import (
	"errors"
	"fmt"
	"github.com/lithammer/shortuuid/v3"
	log "github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
	"io"
	"strings"
	"time"
)

const prompt = "> "
const eol = "\r\n"

const defaultWelcome = "============================================================" + eol +
	"           ______________________________________           " + eol +
	"  ________|                                      |_______  " + eol +
	"  \\       |                Welcome               |      / " + eol +
	"   \\      |                                      |     / " + eol +
	"   /      |______________________________________|     \\ " + eol +
	"  /__________)                                (_________\\ " + eol +
	" " + eol +
	"============================================================" + eol

type Bitmask uint32

func (f Bitmask) HasFlag(flag Bitmask) bool { return f&flag != 0 }
func (f *Bitmask) AddFlag(flag Bitmask)     { *f |= flag }
func (f *Bitmask) ClearFlag(flag Bitmask)   { *f &= ^flag }
func (f *Bitmask) ToggleFlag(flag Bitmask)  { *f ^= flag }

const (
	LOGIN_ENABLED Bitmask = 1 << iota
	USER_LOGGED
)

type User int

type OnNewConsole func(*Console)
type OnCloseTaskCallback func()

const (
	Guest User = iota
	Root
)

type Flusher interface {
	Flush() error
}

type ConsoleI struct {
	io.ReadCloser
	io.Writer
	Flusher
}

type Console struct {
	term             *terminal.Terminal
	eol              string
	quit             chan bool
	welcome          string
	password         string
	mask             Bitmask
	commands         []*ConsoleCommand
	userLevel        User
	iorw             ConsoleI
	onclose          OnCloseTaskCallback
	timeout          time.Duration
	lastActivitytime time.Time
	uuid             string
}

type ConsoleOption func(console *Console)

func WithOptionCustomUUID(uuid string) ConsoleOption {
	return func(console *Console) {
		console.uuid = uuid
	}
}

func NewConsole(iorw ConsoleI, opts ...ConsoleOption) *Console {

	c := Console{term: terminal.NewTerminal(iorw, prompt), eol: eol, mask: 0,
		welcome: defaultWelcome, userLevel: Root, iorw: iorw, onclose: nil}

	cmdhelp := NewConsoleCommand("help", c.printhelp, "show help")
	cmdWamI := NewConsoleCommand("whoAmI", c.cmdWamI, "user level")
	c.commands = append(c.commands, cmdhelp)
	c.commands = append(c.commands, cmdWamI)
	c.quit = make(chan bool, 2)
	c.uuid = shortuuid.New()
	c.timeout = 0
	c.lastActivitytime = time.Now()

	for _, opt := range opts {
		opt(&c)
	}

	c.AddCallbackOnClose(c.dummyCb)
	log.Printf("Open Console %s", c.uuid)
	return &c
}

func (c *Console) GetIO() ConsoleI {
	return c.iorw
}

func (c *Console) GetUUID() string {
	return c.uuid
}

func (c *Console) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *Console) dummyCb() {

}

func (c *Console) AddCallbackOnClose(cb OnCloseTaskCallback) {
	c.onclose = cb
}

func (c *Console) RemoveCallbackOnClose() {
	c.onclose = nil
}

func (c *Console) AddConsoleCommand(cmd *ConsoleCommand) bool {
	c.commands = append(c.commands, cmd)
	return true
}

func (c *Console) removeCmdByIndex(index int) []*ConsoleCommand {
	return append(c.commands[:index], c.commands[index+1:]...)
}

func (c *Console) findCmdIndex(cmd *ConsoleCommand) int {

	for idx, v := range c.commands {
		if v == cmd {
			return idx
		}
	}

	return -1

}

func (c *Console) RemoveConsoleCommand(cmd *ConsoleCommand) bool {
	idx := c.findCmdIndex(cmd)
	if idx >= 0 {
		c.commands = c.removeCmdByIndex(idx)
		return true
	}
	return false
}

func (c *Console) setEol(eol string) {
	c.eol = eol
}

func (c *Console) enablePrompt(status bool) {
	if status {
		c.term.SetPrompt(prompt)
	} else {
		c.term.SetPrompt("")
	}
}

func (c *Console) EnableLogin(password string) {

	c.mask.ToggleFlag(LOGIN_ENABLED)
	c.enablePrompt(false)
	c.password = password

}

func (c *Console) DisableLogin() {
	c.mask.ClearFlag(LOGIN_ENABLED)
	c.enablePrompt(true)

}

func (c *Console) IsLoginEnabled() bool {
	return c.mask.HasFlag(LOGIN_ENABLED)
}

func (c *Console) IsUserLogged() bool {
	return c.mask.HasFlag(USER_LOGGED)
}

func (c *Console) handleLogin(cmd string) bool {

	if cmd == c.password {
		c.mask.ToggleFlag(USER_LOGGED)
		c.enablePrompt(true)
		c.Print("Authenticated")
		return true
	}
	return false
}

func (c *Console) handleCommand(cmd string) bool {

	subs := strings.Split(cmd, " ")
	command2exec := subs[0]
	err := CMD_NOT_FOUND
	for _, i := range c.commands {
		if i.GetCommand() == command2exec && c.userLevel >= i.GetUserLevel() {
			err = i.handler(c, i, subs[1:])
		}
	}

	if err != N0_ERR {
		c.Print(err)
	}
	return true
}

func (c *Console) SetWelcomeMessage(welcome string) {
	c.welcome = welcome
}

func (c *Console) cmdWamI(console *Console, command *ConsoleCommand, args []string) CommandError {
	c.Printf("User Level = %s"+eol, c.userLevel)
	return N0_ERR
}

func (c *Console) printhelp(console *Console, command *ConsoleCommand, args []string) CommandError {

	c.Print("######   LIST OF CONSOLE'S CMD  #######")
	for _, i := range c.commands {
		if c.userLevel >= i.GetUserLevel() {
			c.Printf("---------------------------------------" + eol)
			c.Printf("+ %s "+eol+" # %s #"+eol, i.GetCommand(), i.GetHelp())
		}
	}

	return N0_ERR
}

func (c *Console) Start() bool {
	go c.task()

	return true
}

func (c *Console) Stop() {
	c.quit <- true
	//send and eol to force Readline to quit
	c.iorw.Writer.Write([]byte(c.eol))
	c.flush()
	if c.iorw.Close != nil {
		c.iorw.Close()
	}
}

func (c *Console) checkTimeout() error {

	if c.timeout != 0 && time.Now().Sub(c.lastActivitytime) >= c.timeout {
		defer func() {
			c.Print("Timeout Expired")
			c.Stop()
		}()

		//we must close this connection....

		return errors.New("connection closed")
	}

	return nil
}

func (c *Console) checkTimeoutTask() {
	for {
		timer := time.NewTimer(time.Second * 20)
		select {

		case <-timer.C:
			e := c.checkTimeout()
			if e != nil {
				log.Debugf("Closing go routine checing timeout for uuid %s", c.uuid)
				return
			}
		}
	}
}

func (c *Console) task() error {

	defer c.onclose()

	c.Print(c.welcome)

	go c.checkTimeoutTask()

	for {
		select {
		case <-c.quit:
			log.Printf("Quit Console %s", c.uuid)
			return errors.New("Exit Console task")
		default:
			if c.IsLoginEnabled() && !c.IsUserLogged() {
				c.PrintWithoutLn("Password?")
				pwd, e := c.term.ReadPassword("")
				if e == io.EOF {
					return nil
				}
				if c.IsLoginEnabled() && !c.IsUserLogged() {
					if !c.handleLogin(pwd) {
						continue
					}
				}

			}
			line, err := c.term.ReadLine()
			if err == io.EOF {
				log.Printf("Quit Console , EOF readline - %s", c.uuid)
				return nil
			}
			if err != nil {
				log.Printf("Quit Console , Err: %s - %s", err.Error(), c.uuid)
				return err
			}
			c.lastActivitytime = time.Now()

			if line == "" {
				c.flush()
				continue
			}

			if c.IsLoginEnabled() && !c.IsUserLogged() {
				c.handleLogin(line)

				//} else if c.IsUserLogged() {
			} else {
				c.handleCommand(line)

			}
		}
	}

}

func isValid(v interface{}) bool {
	if v == nil {
		return false
	}
	return true
}

func (c *Console) flush() {
	if isValid(c.iorw.Flusher) {
		c.iorw.Flush()
	}
}

func (c *Console) Print(a ...interface{}) (n int, err error) {
	defer c.flush()
	return fmt.Fprintln(c.term, a...)
}

func (c *Console) Printf(format string, a ...interface{}) (n int, err error) {
	defer c.flush()
	return fmt.Fprintf(c.term, format, a...)
}

func (c *Console) PrintWithoutLn(a ...interface{}) (n int, err error) {
	defer c.flush()
	return fmt.Fprint(c.term, a...)
}

func (c *Console) Println() (n int, err error) {
	defer c.flush()
	return fmt.Fprintln(c.term)
}
