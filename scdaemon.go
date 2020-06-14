package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/dolmen-go/contextio"
)

type Scdaemon struct {
	cancelFunc context.CancelFunc
	cmd        *exec.Cmd
	ctx        context.Context
	host       string
	nonce      [16]byte
	port       int
}

func StartScdaemon(ctx context.Context) (*Scdaemon, error) {

	// Get the gpg4win binary
	scdaemonBinary, err := GetWindowsScdaemonBinaryFn()
	if err != nil {
		return nil, fmt.Errorf("Unable to get Gpg4win scdaemon binary path: %w", err)
	}

	// Prepare and start the scdaemon daemon thread
	args := []string{"--daemon"}
	if VerboseOutput {
		args = append(args, "--debug-level", "guru")
	}

	cmd := exec.Command(scdaemonBinary, args...)
	if VerboseOutput {
		cmd.Stdout = log.Writer()
		cmd.Stderr = log.Writer()
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Error while starting scdaemon binary: %w", err)
	}

	// Get the ip address for the windows host
	host, err := GetResolvAddr()
	if err != nil {
		return nil, fmt.Errorf("Failed to get scdaemon host IP: %w", err)
	}
	log.Printf("Using windows host %s", host)

	// Create the scdaemon object and add the child context with cancel
	scd := Scdaemon{
		host: host,
		cmd:  cmd,
	}
	scd.ctx, scd.cancelFunc = context.WithCancel(ctx)

	// Get the expected windows scdaemon socket filepath
	scdaemonSocketFn, err := GetWindowsScdaemonSocketFn()
	if err != nil {
		return nil, fmt.Errorf("Unable to get Gpg4win scdaemon socket path: %w", err)
	}
	log.Printf("Reading assuan configuration from socket definition %s", scdaemonSocketFn)

	f, err := os.Open(scdaemonSocketFn)
	if err != nil {
		return nil, fmt.Errorf("Unable to open assuan socket definition file: %w", err)
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("Failed to read data from assuan socket definition file: %w", err)
	}

	reader := bytes.NewBuffer(data)

	// Extract port
	portStr, err := reader.ReadString('\n')
	if err == nil {
		scd.port, err = strconv.Atoi(strings.TrimSpace(portStr))
	}
	if err != nil {
		return nil, fmt.Errorf("Unable to get assuan server port from definition: %w", err)
	}

	// Extract nonce
	if n, err := reader.Read(scd.nonce[:]); err != nil {
		return nil, fmt.Errorf("Failed to read nonce from assuan socket definition: %w", err)
	} else if n != 16 {
		return nil, fmt.Errorf("Read incorrect number of bytes for nonce: Expected 16, got %d (0x%X)", n, scd.nonce)
	}

	log.Printf("Using port %d and nonce %x pair", scd.port, scd.nonce)

	// Listen for a signal to close the process
	go func(scd *Scdaemon) {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		for sig := range c {
			log.Printf("Received interupt signal: %+v", sig)
			scd.Close()
		}
	}(&scd)

	return &scd, nil

}

func (scd *Scdaemon) Dial() (net.Conn, error) {

	// Dial server
	conn, err := net.Dial("tcp", net.JoinHostPort(scd.host, fmt.Sprint(scd.port)))
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to assuan server: %w", err)
	}

	// Send nonce to opened connection
	if _, err := conn.Write(scd.nonce[:]); err != nil {
		return nil, fmt.Errorf("Error sending nonce to assuan server: %w", err)
	}

	return conn, nil

}

func (scd *Scdaemon) Connect(in io.Reader, out io.Writer) {

	conn, err := scd.Dial()
	if err != nil {
		log.Printf("Failed to dial scdaemon: %s", err)
		return
	}

	ctx, cancel := context.WithCancel(scd.ctx)
	go ProxyWithContext(ctx, cancel, conn, in)
	go ProxyWithContext(ctx, cancel, out, conn)

	select {
	case <-ctx.Done():
		log.Println("Closing connection")
		conn.Close()
	}

}

func (scd *Scdaemon) Close() error {
	scd.cancelFunc()
	return scd.cmd.Process.Signal(os.Interrupt)
}

func ProxyWithContext(ctx context.Context, close context.CancelFunc, dst io.Writer, src io.Reader) {
	_, err := io.Copy(contextio.NewWriter(ctx, dst), contextio.NewReader(ctx, src))
	if err != nil {
		log.Printf("Copy operation failed: %s", err)
	}
	close()
}
