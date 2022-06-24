package console

import (
	"bufio"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type telnetClient struct {
	console *Console
	uuid    string
	quit    chan bool
}

type TelnetConsole struct {
	mu                   *sync.RWMutex
	clients              []telnetClient
	port                 int
	maxclient            int
	callbackOnNewConsole OnNewConsole
	timeout              time.Duration
}

func (c *TelnetConsole) socketServer(port int) {

	listen, err := net.Listen("tcp4", ":"+strconv.Itoa(port))

	if err != nil {
		log.Fatalf("Socket listen port %d failed,%s", port, err)
		os.Exit(1)
	}

	defer listen.Close()

	//log.Printf("Begin listen port: %d\r\n", port)

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatalln(err)
			continue
		}

		if len(c.clients) > c.maxclient {
			conn.Close()
			fmt.Printf("MAX clients reached! %d/%d", c.clients, c.maxclient)
			fmt.Println()
			continue
		}
		go c.handler(conn)
	}

}

func (c *TelnetConsole) oncClose(uuid string) {
	for _, cl := range c.clients {
		if uuid == cl.uuid {
			cl.quit <- true
		}
	}
}

func (c *TelnetConsole) AddCallbackOnNewConsole(cb OnNewConsole) {
	c.callbackOnNewConsole = cb

}
func (c *TelnetConsole) RemoveCallbackOnNewConsole() {
	c.callbackOnNewConsole = nil

}

func (c *TelnetConsole) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *TelnetConsole) handler(conn net.Conn) {

	var (
		r = bufio.NewReader(conn)
		w = bufio.NewWriter(conn)
	)

	rc := struct {
		io.Reader
		io.Closer
	}{
		r, conn,
	}

	io := struct {
		io.ReadCloser
		io.Writer
		Flusher
	}{rc, w, w}

	console := NewConsole(io)
	uuid := console.uuid
	quit := make(chan bool)
	client := telnetClient{console: console, uuid: uuid, quit: quit}

	c.mu.Lock()
	c.clients = append(c.clients, client)
	c.mu.Unlock()

	if c.callbackOnNewConsole != nil {
		c.callbackOnNewConsole(console)
	}

	console.AddCallbackOnClose(func() {
		quit <- true
	})
	console.SetTimeout(c.timeout)
	console.Start()

	select {
	case <-quit:
		break
	}

	c.mu.Lock()
	for idx, cl := range c.clients {
		if cl.uuid == uuid {
			c.clients = append(c.clients[0:idx], c.clients[idx+1:]...)
			break
		}
	}
	c.mu.Unlock()

}

func NewTelnetConsole(port int, maxclient int) *TelnetConsole {
	c := TelnetConsole{mu: &sync.RWMutex{}, port: port, maxclient: maxclient}
	c.callbackOnNewConsole = nil
	go c.socketServer(port)
	return &c
}
