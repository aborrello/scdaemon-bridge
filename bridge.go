// +build linux

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
)

var (
	flagServer      bool
	flagMultiServer bool
	flagDaemon      bool
)

func mainBridge() {

	socketDir, err := gpgconfLinux.GetDirectory("socketdir")
	if err != nil {
		log.Fatalw("Failed to get socketdir from gpgconf", "error", err)
	}

	ScdaemonSocketName = path.Join(socketDir, ScdaemonSocketFilename)
	log.Debugw("Starting scdaemon relay", "socket_name", ScdaemonSocketName, "args", os.Args)

	flag.BoolVar(&flagServer, "server", false, "run in server mode (foreground)")
	flag.BoolVar(&flagMultiServer, "multiserver", false, "run in multi server mode (foreground)")
	flag.BoolVar(&flagDaemon, "daemon", false, "run in daemon mode (background)")
	flag.Parse()

	// Derive the path for the relay binary
	gpgWinHomeDir, err := gpgconfWin.GetDirectory("homedir")
	if err != nil {
		log.Fatalw("Unable to get Gpg4win home directory", "error", err)
	}
	relayBinary := path.Join(GetWslPath(GetWindowsPath(gpgWinHomeDir)), "scdaemon-relay.exe")

	// Start scdaemon in daemon mode
	daemon, err := StartScdaemon()
	if err != nil {
		log.Fatalw("Failed to start scdaemon", "error", err)
	}
	defer daemon.Process.Signal(os.Interrupt)

	if flagDaemon || flagMultiServer {
		// Start the assuan listner
		l, nonce, err := StartAssuanListener()
		if err != nil {
			log.Fatalw("Unable to start assuan listener", "error", err)
		}
		go ProxyAssuanRequests(l, nonce, relayBinary)
	}

	if flagServer || flagMultiServer {
		// Open a relay for stdin and stdout
		go StartRelay(relayBinary, os.Stdin, os.Stdout, os.Stderr)
	}

}

// StartScdaemon starts scdaemon in daemon mode.
func StartScdaemon() (*exec.Cmd, error) {

	scdaemonBinary, err := gpgconfWin.GetBinary("scdaemon")
	if err != nil {
		return nil, fmt.Errorf("Unable to get Gpg4win scdaemon binary path: %w", err)
	}
	scdaemonBinary = GetWslPath(GetWindowsPath(scdaemonBinary))

	args := []string{"--daemon"}
	if verboseLogging {
		args = append(args, "--debug-level", "guru")
	}

	cmd := exec.Command(scdaemonBinary, args...)
	err = cmd.Start()
	return cmd, err

}

// StartRelay opens the relay binary on the Windows environment with the specified in, out and error channels.
func StartRelay(relayBinary string, stdin io.Reader, stdout io.Writer, stderr io.Writer) {

	cmd := exec.Command(relayBinary)
	cmd.Stdin = NewSocketNameMiddleware(stdin, stdout)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		log.Errorw("Failed to start relay", "error", err)
		return
	}

	if err := cmd.Wait(); err != nil {
		log.Errorw("Relay exited during operation", "error", err)
	}

}

// SocketNameMiddleware implements io.Reader and overrides GETINFO socket_name, returning the
// bridged socket in the WSL space instead of the actual.
type SocketNameMiddleware struct {
	input       io.Reader
	output      io.Writer
	overrideCmd []byte
	overrideVal []byte
}

// NewSocketNameMiddleware creates a new SocketNameMiddleware for the provided input and output.
func NewSocketNameMiddleware(input io.Reader, output io.Writer) SocketNameMiddleware {

	return SocketNameMiddleware{
		input:       input,
		output:      output,
		overrideCmd: []byte("GETINFO socket_name\n"),
		overrideVal: []byte(fmt.Sprintf("D %s\nOK\n", ScdaemonSocketName)),
	}

}

func (middleware SocketNameMiddleware) Read(p []byte) (int, error) {

	n, err := middleware.input.Read(p)
	if err != nil {
		return -1, err
	}

	if bytes.HasPrefix(p, middleware.overrideCmd) {
		middleware.output.Write(middleware.overrideVal)
		return middleware.Read(p)
	}

	return n, nil

}

// StartAssuanListener creates a new assuan listener and accompanying socket wrapper descriptor.
func StartAssuanListener() (l net.Listener, nonce []byte, err error) {

	l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return
	}

	f, err := os.OpenFile(ScdaemonSocketName, os.O_WRONLY|os.O_CREATE, 0400)
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
func ProxyAssuanRequests(l net.Listener, nonce []byte, relayBinary string) error {

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Warnw("Unable to establish connection", "error", err)
			continue
		}

		verify := make([]byte, 16)
		n, err := conn.Read(verify)
		if err != nil {
			log.Warnw("Failed to read nonce: %w", err)
			continue
		}

		if bytes.Compare(verify, nonce) != 0 {
			log.Warnw("Invalid connection nonce", "len", n, "expected", nonce, "got", verify)
			continue
		}

		go StartRelay(relayBinary, conn, conn, os.Stderr)
	}

}
