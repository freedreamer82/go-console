# go-console

go console is a go lib to implement consoles, it includes:

- Std output console
- Telnet console
- SSH console
- Other Consoles can be added easily(BLE,Serial etc..)



```sh
[============================================================
           ______________________________________           
  ________|                                      |_______  
  \       |                Welcome               |      / 
   \      |                                      |     / 
   /      |______________________________________|     \ 
  /__________)                                (_________\ 
 
[============================================================
> Password?
>
>
>
```

### Std output console example
```sh
c := console.NewStdOutputConsole()
c.EnableLogin("root")
c.Start()
```

### Telnet console example
```sh
func onNewTelnetConsole(console *console.Console) {
	console.EnableLogin("root")
}

ct := console.NewTelnetConsole(telnetPort, 2)
ct.AddCallback(onNewTelnetConsole)
```

### SSH console example
with password
```sh
sshc, _ := console.NewSSHConsoleWithPassword(sshPrivateKeyPath, users, console.WithOptionKeyPassphrase(sshPrivateKeyPassPhrase))
go sshc.Start("localhost", sshPort, 2)
```
with pub key
```sh
sshc, _ := console.NewSSHConsoleWithCertificates(sshPrivateKeyPath, sshAuthorizedKeysPath, console.WithOptionKeyPassphrase(sshPrivateKeyPassPhrase))
go sshc.Start("localhost", sshPort, 2)
```

### go console api
handle operation
- func (c *Console) Start()
- func (c *Console) Stop()
- func (c *Console) AddCallbackOnClose(cb OnCLoseTaskCallback)
- func (c *Console) RemoveCallbackOnClose()
- func (c *Console) GetUUID() string
---------------------------------------
handle console commands (help and whoAmI already implemented)
- func (c *Console) AddConsoleCommand(cmd *ConsoleCommand)
- func (c *Console) RemoveConsoleCommand(cmd *ConsoleCommand)
---------------------------------------
handle login
- func (c *Console) EnableLogin(password string)
- func (c *Console) DisableLogin()
- func (c *Console) IsLoginEnabled() bool
- func (c *Console) IsUserLogged() bool
----------------------------------------
customize aspect
- func (c *Console) SetWelcomeMessage(welcome string)
- func (c *Console) SetTimeout(timeout time.Duration)
----------------------------------------
print
- func (c *Console) Print(a ...interface{}) (n int, err error)
- func (c *Console) Printf(format string, a ...interface{}) (n int, err error)
- func (c *Console) PrintWithoutLn(a ...interface{}) (n int, err error)
- func (c *Console) Println() (n int, err error)

### console commands
- func NewConsoleCommand(cmd string, handler ConsoleCommandHandler, help string) *ConsoleCommand

  [   type ConsoleCommandHandler func(console *Console, command *ConsoleCommand, args []string) CommandError ]
- func (c *ConsoleCommand) GetCommand() string
- func (c *ConsoleCommand) GetHelp() string
- func (c *ConsoleCommand) GetUserLevel() User
- func (c *ConsoleCommand) SetUserLevel(level User)

### example command implementation

```sh
hndEcho := func(c *Console, command *ConsoleCommand, args []string) {
  var echo = string
  for _,s := range args {
    echo = fmt.Sprintf("%s %s", echo, s)
  }
  c.Print(echo)
  return NO_ERR
}

echoCommand := NewConsoleCommand("echo", hndEcho, "display a text or a string to the standard output")
echoCommand.SetUserLevel(Guest)

myConsole.addConsoleCommand(echoCommand)
```

### example add command on new console callback

```sh
sshc, _ := console.NewSSHConsoleWithPassword(
  sshPrivateKeyPath, 
  users, 
  console.WithOptionKeyPassphrase(sshPrivateKeyPassPhrase)
)
  
func onNewConsoleAddCommand(console *console.Console) {
  console.addConsoleCommand(echoCommand)
}
sshc.AddCallbackOnNewConsole(onNewConsoleAddCommand)
go sshc.Start("localhost", sshPort, 2)
```

### example add timeout 

```sh
sshc, _ := console.NewSSHConsoleWithPassword(
  sshPrivateKeyPath, 
  users, 
  console.WithOptionKeyPassphrase(sshPrivateKeyPassPhrase)
  console.WithOptionConsoleTimeout(2*time.Minute)
)

go sshc.Start("localhost", sshPort, 2)
```

### run the example
(set parameters like psw, file path, ports on examples/server/main.go file)
```sh
cd example/server
go build

//console on std output
./server 0

//console telnet
./server 1

//console ssh (password)
./server 2

//console ssh (pub keys)
./server 3
```