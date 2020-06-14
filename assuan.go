package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"os"
)

// StartAssuanListener creates a new assuan listener and accompanying socket wrapper descriptor.
func StartAssuanListener() (l net.Listener, nonce []byte, err error) {

	l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return
	}

	fn, err := GetLinuxScdaemonSocketFn()
	if err != nil {
		return
	}

	log.Printf("Using socket fn: %s", fn)

	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0400)
	if err != nil {
		return
	}
	defer f.Close()

	port := l.Addr().(*net.TCPAddr).Port

	nonce = make([]byte, 16)
	_, err = rand.Read(nonce)
	if err != nil {
		return
	}

	log.Printf("Listening on port %d protected by nonce %x", port, nonce)

	socket := bufio.NewWriter(f)

	_, err = socket.WriteString(fmt.Sprintf("%d\n", port))
	if err != nil {
		return
	}

	_, err = socket.Write(nonce)
	if err != nil {
		return
	}

	err = socket.Flush()
	return

}

// ProxyAssuanRequests accepts new assuan requests and opens new relay connections. Additionally,
// it verifies the nonce, simulating a regular assuan server.
func ProxyAssuanRequests(daemon *Scdaemon, l net.Listener, nonce []byte) {

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("Could not accept connection: %s", err)
			break
		}

		verify := make([]byte, 16)
		n, err := conn.Read(verify)
		if err != nil {
			log.Printf("Failed to read nonce: %s", err)
			continue
		}

		if bytes.Compare(verify, nonce) != 0 {
			log.Printf("Invalid connection nonce: %+v != %+v (len: %d)", verify, nonce, n)
			continue
		}

		log.Printf("Received connection")
		go daemon.Connect(conn, conn)
	}

}
