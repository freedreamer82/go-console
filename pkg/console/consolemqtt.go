package console

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	mqttinfo "github.com/freedreamer82/mqtt-shell/pkg/info"
	mqttchat "github.com/freedreamer82/mqtt-shell/pkg/mqttchat"
	log "github.com/sirupsen/logrus"
	"io"
	"strings"
	"sync"
	"time"
)

const defaultPresentationMessage = "whoami"

const topicPrefix = "/mqtt-shell/"

var templateRxTopic = topicPrefix + "%s/cmd"
var templateTxTopic = topicPrefix + "%s/cmd/res"
var templateBeaconEvent = topicPrefix + "%s/event"

const topicBeaconRequest = topicPrefix + "whoami"
const defautMqttConsoleWelcome = "***    Welcome MQTT Console    ***"

type outMessage struct {
	msg        string
	clientUUID string
	cmdUUID    string
}

type mqttConsoleConnection struct {
	uuid        string
	lastCmdUuid string
	in          chan []byte
	out         *chan outMessage
	isUp        bool
}

func (conn *mqttConsoleConnection) Read(b []byte) (int, error) {
	data := <-conn.in
	if data == nil || string(data) == defaultPresentationMessage {
		return 0, nil
	}
	dataEOF := string(data) + "\r\n"
	n := copy(b, []byte(dataEOF))
	return n, nil
}

func (conn *mqttConsoleConnection) Close() error {
	conn.isUp = false
	conn.in <- []byte("onclose")
	close(conn.in)
	return nil
}

func (conn *mqttConsoleConnection) Write(b []byte) (int, error) {
	if conn.isUp {
		msg := strings.Trim(string(b), "\r\n")
		*conn.out <- outMessage{msg: msg, clientUUID: conn.uuid, cmdUUID: conn.lastCmdUuid}
	}
	return len(b), nil
}

type MqttConsole struct {
	mqttChat             *mqttchat.MqttChat
	connections          sync.Map
	consoles             sync.Map
	callbackOnNewConsole OnNewConsole
	timeout              time.Duration
	maxConnections       int
	chOut                chan outMessage
}

type MqttConsoleOption func(console *MqttConsole)

func WithOptionMqttConsoleMaxConnections(maxConnections int) MqttConsoleOption {
	return func(console *MqttConsole) {
		console.maxConnections = maxConnections
	}
}

func WithOptionMqttConsoleTimeout(timeout time.Duration) MqttConsoleOption {
	return func(console *MqttConsole) {
		console.timeout = timeout
	}
}

func (mqttConsole *MqttConsole) AddCallbackOnNewConsole(cb OnNewConsole) {
	mqttConsole.callbackOnNewConsole = cb
}

func (mqttConsole *MqttConsole) countConnections() int {
	size := 0
	mqttConsole.connections.Range(func(k, v interface{}) bool {
		size++
		return true
	})
	return size
}

func NewMqttConsole(mqttOption *mqtt.ClientOptions, instanceID string, opts ...MqttConsoleOption) *MqttConsole {

	mqttConsole := &MqttConsole{maxConnections: 0}

	txTopic := fmt.Sprintf(templateTxTopic, instanceID)
	rxTopic := fmt.Sprintf(templateRxTopic, instanceID)
	beaconTopic := fmt.Sprintf(templateBeaconEvent, instanceID)
	version := fmt.Sprintf("mqtt-%s", mqttinfo.VERSION)

	mqttChat := mqttchat.NewChat(mqttOption, rxTopic, txTopic, version, mqttchat.WithOptionBeaconTopic(beaconTopic, topicBeaconRequest))
	mqttChat.SetDataCallback(mqttConsole.onDataRx)

	out := make(chan outMessage, 100)

	mqttConsole.chOut = out
	mqttConsole.mqttChat = mqttChat

	for _, opt := range opts {
		opt(mqttConsole)
	}

	return mqttConsole
}

func (mqttConsole *MqttConsole) Start() {
	log.Info("START - MQTT Console")
	go mqttConsole.dataTx()
	mqttConsole.mqttChat.Start()
}

func (mqttConsole *MqttConsole) removeConsoleAndConnection(clientUUID string) {
	log.Infof("Close mqtt connection with client: %s", clientUUID)
	mqttConsole.connections.Delete(clientUUID)
	mqttConsole.consoles.Delete(clientUUID)
}

func (mqttConsole *MqttConsole) createNewConsoleAndConnection(clientUUID string) (*Console, *mqttConsoleConnection) {
	in := make(chan []byte, 20)
	conn := &mqttConsoleConnection{uuid: clientUUID, out: &mqttConsole.chOut, in: in, isUp: true}

	mqttConsole.connections.Store(clientUUID, conn)

	consoleIO := struct {
		io.ReadCloser
		io.Writer
		Flusher
	}{
		conn,
		conn,
		nil,
	}

	console := NewConsole(consoleIO, WithOptionCustomUUID(clientUUID))
	console.AddCallbackOnClose(func() {
		mqttConsole.removeConsoleAndConnection(clientUUID)
	})

	if mqttConsole.callbackOnNewConsole != nil {
		mqttConsole.callbackOnNewConsole(console)
	}

	console.SetTimeout(mqttConsole.timeout)

	mqttConsole.consoles.Store(clientUUID, console)

	log.Infof("Open mqtt connection with client: %s", clientUUID)

	return console, conn
}

func (mqttConsole *MqttConsole) onDataRx(data mqttchat.MqttJsonData) {

	if data.CmdUUID == "" || data.Cmd == "" || data.Data == "" || data.ClientUUID == "" {
		return
	}

	_, consoleExist := mqttConsole.consoles.Load(data.ClientUUID)

	if !consoleExist {
		if mqttConsole.maxConnections > 0 && mqttConsole.countConnections() >= mqttConsole.maxConnections {
			log.Warn("Max number of connection reached")
			return
		}
		newConsole, newConn := mqttConsole.createNewConsoleAndConnection(data.ClientUUID)
		*newConn.out <- outMessage{msg: " \r\n", clientUUID: data.ClientUUID, cmdUUID: data.CmdUUID}
		newConsole.SetWelcomeMessage(defautMqttConsoleWelcome)
		newConsole.Start()
		return
	}

	conn, connExist := mqttConsole.connections.Load(data.ClientUUID)

	if !connExist || conn == nil {
		log.Warn("Connection not found")
		return
	}

	mqttConn := conn.(*mqttConsoleConnection)
	mqttConn.lastCmdUuid = data.CmdUUID
	mqttConn.in <- []byte(data.Data)

}

func (mqttConsole *MqttConsole) dataTx() {
	for {
		select {
		case out := <-mqttConsole.chOut:
			outMsg := out.msg
			if outMsg != "" {
				if outMsg == "\r\n" {
					outMsg = ""
				}
				outMsg = strings.Replace(outMsg, ">", "", -1)
				mqttConsole.mqttChat.Transmit(outMsg, out.cmdUUID, out.clientUUID)
			}
		}
	}
}
