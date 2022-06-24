package main

import (
	"fmt"
	"github.com/freedreamer82/go-console/pkg/console"
	"os"
	"strconv"
	"time"
)

const telnetPort = 6666
const sshPort = 5559

var users = map[string]string{
	"root":  "root",
	"guest": "1234",
	"test":  "qwerty",
}

const sshPrivateKeyPath = "examples/utils/server_rsa"
const sshPrivateKeyPassPhrase = "test"
const timeoutSec = 10

//client1 and client2 are authorized
const sshAuthorizedKeysPath = "examples/utils/authorized_keys"

const (
	StdOutput int = iota
	Telnet
	SSHPassword
	SSHPublicKey
)

//change this const to start another console
const example = StdOutput

func main() {

	ex := example

	if len(os.Args) >= 2 {
		ex, _ = strconv.Atoi(os.Args[1])
	}

	switch ex {
	case StdOutput:
		startStdOutputConsole()
	case Telnet:
		startTelnetConsole()
	case SSHPassword:
		startSSHPasswordConsole()
	case SSHPublicKey:
		startSSHPublicKeyConsole()
	}

}

func onNewTelnetConsole(console *console.Console) {
	console.EnableLogin("root")
}

func startStdOutputConsole() {
	fmt.Printf("Console On Std Output...")
	fmt.Println()
	c := console.NewStdOutputConsole()
	c.EnableLogin("root")
	c.SetTimeout(timeoutSec * time.Second)
	c.Start()
	select {}
}

func startTelnetConsole() {
	fmt.Printf("opening Telnet console on localhost %d", telnetPort)
	fmt.Println()
	ct := console.NewTelnetConsole(telnetPort, 2)
	ct.AddCallbackOnNewConsole(onNewTelnetConsole)
	ct.SetTimeout(timeoutSec * time.Second)
	select {}
}

func startSSHPasswordConsole() {
	fmt.Printf("opening SSH console on localhost %d", sshPort)
	fmt.Println()
	opt1 := console.WithOptionKeyPassphrase(sshPrivateKeyPassPhrase)
	opt2 := console.WithOptionConsoleTimeout(timeoutSec * time.Second)
	sshc, _ := console.NewSSHConsoleWithPassword(sshPrivateKeyPath, users, opt1, opt2)
	go sshc.Start("localhost", sshPort, 2)
	select {}
}

func startSSHPublicKeyConsole() {
	fmt.Printf("opening SSH console on localhost %d", sshPort)
	fmt.Println()
	opt1 := console.WithOptionKeyPassphrase(sshPrivateKeyPassPhrase)
	opt2 := console.WithOptionConsoleTimeout(timeoutSec * time.Second)
	sshc, _ := console.NewSSHConsoleWithCertificates(sshPrivateKeyPath, sshAuthorizedKeysPath, opt1, opt2)
	go sshc.Start("localhost", sshPort, 2)
	select {}
}
