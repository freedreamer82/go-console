package console

import (
	terminal "golang.org/x/term"
	"io"
	"os"
	"os/signal"
)

type StdOutputConsole struct {
	*Console
	oldstate *terminal.State
	chexit   chan os.Signal
}

func (c *StdOutputConsole) onExit() {

	terminal.Restore(0, c.oldstate)
	os.Exit(0)
}

func NewStdOutputConsole() *StdOutputConsole {

	screen := struct {
		io.ReadCloser
		io.Writer
		Flusher
	}{os.Stdin, os.Stdout, nil}

	c := StdOutputConsole{Console: NewConsole(screen)}

	c.AddCallbackOnClose(c.onExit)
	c.chexit = make(chan os.Signal, 1)
	signal.Notify(c.chexit, os.Interrupt)
	return &c
}

//func (s *StdOutputConsole) setupCloseHandler() {
//	c := make(chan os.Signal)
//	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
//	go func() {
//		<-c
//		s.Stop()
//		terminal.Restore(0, s.oldstate)
//		fmt.Println("\r- Ctrl+C pressed in Terminal")
//		os.Exit(0)
//	}()
//}

func (c *StdOutputConsole) Start() bool {

	if !terminal.IsTerminal(0) || !terminal.IsTerminal(1) {
		return false
	}
	var err error

	c.oldstate, err = terminal.MakeRaw(0)
	if err != nil {
		return false
	}

	//c.setupCloseHandler()

	//select {}
	return c.Console.Start()
}
