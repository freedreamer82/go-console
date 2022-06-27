package main

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/freedreamer82/go-console/examples/utils"
	"github.com/freedreamer82/go-console/pkg/console"
	log "github.com/sirupsen/logrus"
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
const timeoutSec = 20

//client1 and client2 are authorized
const sshAuthorizedKeysPath = "examples/utils/authorized_keys"

const (
	StdOutput int = iota
	Telnet
	SSHPassword
	SSHPublicKey
	Mqtt
)

//change this const to start another console
const example = Mqtt

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
	case Mqtt:
		startMqttConsole()
	}

}

func onNewConsole(console *console.Console) {
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
	ct.AddCallbackOnNewConsole(onNewConsole)
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

func startMqttConsole() {
	fmt.Printf("opening Mqtt console")
	fmt.Println()
	clientOps := mqtt.NewClientOptions()
	addr := fmt.Sprintf("tcp://%s:%d", utils.BrokerHost, utils.BrokerPort)
	log.Info("Connecting to : " + addr)
	clientOps.AddBroker(addr)
	if utils.BrokerUser != "" && utils.BrokerPassword != "" {
		clientOps.SetUsername(utils.BrokerUser)
		clientOps.SetPassword(utils.BrokerPassword)
	}
	opt1 := console.WithOptionMqttConsoleMaxConnections(3)
	opt2 := console.WithOptionMqttConsoleTimeout(timeoutSec * time.Second)
	mqttConsole := console.NewMqttConsole(clientOps, "test", opt1, opt2)
	mqttConsole.AddCallbackOnNewConsole(onNewConsole)
	go mqttConsole.Start()
	select {}
}
