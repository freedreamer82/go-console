package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// Handles TC connection and perform synchorinization:
// TCP -> Stdout and Stdin -> TCP
func tcp_con_handle(con net.Conn) {
	chan_to_stdout := stream_copy(con, os.Stdout)
	chan_to_remote := stream_copy(os.Stdin, con)
	select {
	case <-chan_to_stdout:
		log.Println("Remote connection is closed")
	case <-chan_to_remote:
		log.Println("Local program is terminated")
	}
}

// Performs copy operation between streams: os and tcp streams
func stream_copy(src io.Reader, dst io.Writer) <-chan int {
	buf := make([]byte, 1024)
	sync_channel := make(chan int)
	go func() {
		defer func() {
			if con, ok := dst.(net.Conn); ok {
				con.Close()
				log.Printf("Connection from %v is closed\n", con.RemoteAddr())
			}
			sync_channel <- 0 // Notify that processing is finished
		}()
		for {
			var nBytes int
			var err error
			nBytes, err = src.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("Read error: %s\n", err)
				}
				break
			}
			tosend := string(buf[0:nBytes])
			eol := string("\r")
			_, err = dst.Write([]byte(strings.TrimSuffix(tosend, "\n")))
			_, err = dst.Write([]byte(eol))
			if err != nil {
				log.Fatalf("Write error: %s\n", err)
			}
		}
	}()
	return sync_channel
}

func main() {

	var destinationPort string

	var host string

	flag.Parse()
	if flag.NFlag() == 0 && flag.NArg() == 0 {
		fmt.Println("console-tool [hostname ] [port[s]]")
		flag.Usage()
		os.Exit(1)
	}

	if flag.NArg() < 2 {
		log.Println("[hostname ] [port] are mandatory arguments")
		os.Exit(1)
	}

	if _, err := strconv.Atoi(flag.Arg(1)); err != nil {
		log.Println("Destination port shall be not empty and have integer value")
		os.Exit(1)
	}
	host = flag.Arg(0)
	destinationPort = fmt.Sprintf(":%v", flag.Arg(1))

	log.Println("Hostname:", host)
	log.Println("Port:", destinationPort)

	if host != "" {
		con, err := net.Dial("tcp", host+destinationPort)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("Connected to", host+destinationPort)
		tcp_con_handle(con)
	} else {
		flag.Usage()
	}

}
