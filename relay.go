package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

var (
	// ScdaemonSocketName is a runtime generated filepath to the WK location of the scdaemon socket descriptor.
	ScdaemonSocketName string
	log                zap.SugaredLogger
)

func mainRelay() {

	socketDir, err := gpgconfWin.GetDirectory("socketdir")
	if err != nil {
		log.Fatalw("Failed to get socketdir from gpgconf", "error", err)
	}

	ScdaemonSocketName = GetWindowsPath(path.Join(socketDir, ScdaemonSocketFilename))
	log.Debugw("Starting scdaemon relay", "socket_name", ScdaemonSocketName)

	conn, err := DialAssuan(ScdaemonSocketName)
	if err != nil {
		log.Fatalw("Error opening TCP connection to assuan server", "error", err)
	}
	defer conn.Close()

	go func(conn net.Conn, in io.Reader) {
		_, err := io.Copy(conn, in)
		if err != nil {
			log.Warnw("Failed to copy from input to connection", "error", err)
		}
	}(conn, os.Stdin)

	if _, err := io.Copy(os.Stdout, conn); err != nil {
		log.Warnw("Failed to copy from connection to output", "error", err)
	}

}

// DialAssuan attempts to open a TCP connection to an assuan server based on a socket wrapper descriptor.
func DialAssuan(fn string) (net.Conn, error) {

	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("Unable to open assuan socket definition file: %w", err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("Failed to read data from assuan socket definition file: %w", err)
	}

	var port int
	var nonce [16]byte
	reader := bytes.NewBuffer(data)

	portStr, err := reader.ReadString('\n')
	if err == nil {
		port, err = strconv.Atoi(strings.TrimSpace(portStr))
	}
	if err != nil {
		return nil, fmt.Errorf("Unable to get assuan server port from definition: %w", err)
	}

	if n, err := reader.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("Failed to read nonce from assuan socket definition: %w", err)
	} else if n != 16 {
		return nil, fmt.Errorf("Read incorrect number of bytes for nonce: Expected 16, got %d (0x%X)", n, nonce)
	}

	conn, err := net.Dial("tcp", net.JoinHostPort("localhost", fmt.Sprint(port)))
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to assuan server: %w", err)
	}

	if _, err := conn.Write(nonce[:]); err != nil {
		return nil, fmt.Errorf("Error sending nonce to assuan server: %w", err)
	}

	return conn, nil

}
