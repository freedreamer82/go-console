package console

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"sync"
	"time"
)

type connMap = map[*ssh.ServerConn][]ssh.Channel

type SSHConsole struct {
	mu                   *sync.RWMutex
	sshConfig            *ssh.ServerConfig
	listener             net.Listener
	consoles             []*Console
	connections          connMap
	keyPassPhrase        string
	callbackOnNewConsole OnNewConsole
	timeout              time.Duration
}

type SSHConsoleOption func(console *SSHConsole)

func WithOptionKeyPassphrase(passphrase string) SSHConsoleOption {
	return func(console *SSHConsole) {
		console.keyPassPhrase = passphrase
	}
}

func WithOptionConsoleTimeout(timeout time.Duration) SSHConsoleOption {
	return func(console *SSHConsole) {
		console.timeout = timeout
	}
}

func (c *SSHConsole) AddCallbackOnNewConsole(cb OnNewConsole) {
	c.callbackOnNewConsole = cb

}

func NewSSHConsoleWithPassword(
	hostPrivateKeyFile string,
	userToPassword map[string]string,
	opts ...SSHConsoleOption,
) (*SSHConsole, error) {

	console := &SSHConsole{
		mu:            &sync.RWMutex{},
		listener:      nil,
		consoles:      nil,
		connections:   make(connMap),
		keyPassPhrase: "",
	}

	for _, opt := range opts {
		opt(console)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			loginPassword, ok := userToPassword[conn.User()]
			if !ok {
				return nil, fmt.Errorf("unkown user: %q", conn.User())
			}
			if string(password) == loginPassword {
				//return &ssh.Permissions{Extensions: map[string]string{"": ""}}, nil
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", conn.User())
		},
		BannerCallback: nil,
	}

	keyBytes, err := ioutil.ReadFile(hostPrivateKeyFile)
	if err != nil {
		return nil, err
	}

	var signer ssh.Signer

	if console.keyPassPhrase == "" {
		signer, err = ssh.ParsePrivateKey(keyBytes)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(console.keyPassPhrase))
	}

	if err != nil {
		return nil, err
	}

	config.AddHostKey(signer)

	console.sshConfig = config

	return console, nil
}

func NewSSHConsoleWithCertificates(
	hostPrivateKeyFile string,
	authorizedKeysFile string,
	opts ...SSHConsoleOption,
) (*SSHConsole, error) {

	console := &SSHConsole{
		mu:            &sync.RWMutex{},
		listener:      nil,
		consoles:      nil,
		connections:   make(connMap),
		keyPassPhrase: "",
	}

	for _, opt := range opts {
		opt(console)
	}

	authorizedKeysBytes, err := ioutil.ReadFile(authorizedKeysFile)
	if err != nil {
		log.Printf("Failed to load authorized_keys, err: %v", err)
		return nil, err
	}

	authorizedKeysMap := map[string]bool{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			log.Fatal(err)
		}

		authorizedKeysMap[string(pubKey.Marshal())] = true
		authorizedKeysBytes = rest
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if authorizedKeysMap[string(pubKey.Marshal())] {
				return &ssh.Permissions{
					// Record the public key used for authentication.
					Extensions: map[string]string{
						"pubkey-fp": ssh.FingerprintSHA256(pubKey),
					},
				}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", conn.User())
		},
		BannerCallback: nil,
	}

	keyBytes, err := ioutil.ReadFile(hostPrivateKeyFile)
	if err != nil {
		return nil, err
	}

	var signer ssh.Signer

	if console.keyPassPhrase == "" {
		signer, err = ssh.ParsePrivateKey(keyBytes)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(console.keyPassPhrase))
	}

	if err != nil {
		return nil, err
	}

	config.AddHostKey(signer)

	console.sshConfig = config

	return console, nil
}

func (c *SSHConsole) Start(host string, port int, maxConnections int) error {
	listener, err := net.Listen("tcp4", host+":"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.listener = listener
	c.mu.Unlock()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		c.mu.RLock()
		numConnections := len(c.connections)
		c.mu.RUnlock()

		if numConnections >= maxConnections {
			log.Println("Max clients reached")
			if err := conn.Close(); err != nil {
				return err
			}
			continue
		}

		sshConnection, newChans, _, err := ssh.NewServerConn(
			conn,
			c.sshConfig,
		)
		if err != nil {
			log.Print(err.Error())
			continue
		}

		c.mu.Lock()
		c.connections[sshConnection] = make([]ssh.Channel, 0)
		c.mu.Unlock()

		log.Println("Connection from ", sshConnection.RemoteAddr(), " accepted")

		go func() {
			for chanReq := range newChans {
				req := chanReq

				go func() {
					if err := c.handleChanReq(sshConnection, req); err != nil {
						log.Println("Failed to handle channel request")
					}
				}()
			}
		}()
	}
}

func (c *SSHConsole) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, console := range c.consoles {
		console.Stop()
	}

	for conn, channels := range c.connections {
		for _, channel := range channels {
			if err := channel.Close(); err != nil {
				return err
			}
		}
		if err := conn.Close(); err != nil {
			return err
		}
	}
	c.connections = nil

	if err := c.listener.Close(); err != nil {
		return err
	}

	return nil
}

func (c *SSHConsole) closeChannel(conn *ssh.ServerConn, channel ssh.Channel) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channels, ok := c.connections[conn]; ok {
		activeChannels := 0
		for i := 0; i < len(channels); i++ {
			if channels[i] == channel {
				//EOF means connection already disconnected!
				if err := channel.Close(); err != nil && err.Error() != "EOF" {
					return err
				}
				channels[i] = nil
			} else if channels[i] != nil {
				activeChannels++
			}
		}

		if activeChannels == 0 {
			// Close main connection
			err := conn.Close()
			delete(c.connections, conn)
			//if errors.Is(err, errors.New("use of closed network connection")) {
			//	return nil
			//}
			return err
		}

		return nil
	}
	return errors.New("connection not found")
}

func (c *SSHConsole) handleChanReq(conn *ssh.ServerConn, req ssh.NewChannel) error {
	var err error

	if req.ChannelType() != "session" {
		err = req.Reject(ssh.Prohibited, "not a session request")
		if err != nil {
			log.Println("Failed to reject request")
			return err
		}
	}

	ch, _, err := req.Accept()
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.connections[conn] = append(c.connections[conn], ch)
	c.mu.Unlock()

	consoleIO := struct {
		io.ReadCloser
		io.Writer
		Flusher
	}{
		ch,
		ch,
		nil,
	}

	console := NewConsole(consoleIO)
	console.AddCallbackOnClose(func() {
		err := c.closeChannel(conn, ch)
		if err != nil {
			log.Println("Failed to close console channel, already closed?")
		}
	})

	if c.callbackOnNewConsole != nil {
		c.callbackOnNewConsole(console)
	}

	console.SetTimeout(c.timeout)

	c.mu.Lock()
	c.consoles = append(c.consoles, console)
	c.mu.Unlock()

	console.Start()

	log.Println("SSH channel opened ")

	return nil
}
