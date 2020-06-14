// +build linux

package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

var (
	flagServer      bool
	flagMultiServer bool
	flagDaemon      bool
)

func main() {

	// Log to file if binary built with debug flag
	if VerboseOutput {
		homedir, err := GetLinuxGnupgHomePath()
		if err != nil {
			log.Panicln("Could not get gnupg homedir:", err)
		}

		f, err := os.OpenFile(filepath.Join(homedir, "scdaemon-bridge.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Panicln("Unable to open debug sink:", err)
		}

		log.SetOutput(f)
		defer f.Close()
	} else {
		log.SetOutput(ioutil.Discard)
	}

	flag.BoolVar(&flagServer, "server", false, "run in server mode (foreground)")
	flag.BoolVar(&flagMultiServer, "multi-server", false, "run in multi server mode (foreground)")
	flag.BoolVar(&flagDaemon, "daemon", false, "run in daemon mode (background)")
	flag.Parse()

	log.Printf(`Starting scdaemon-bridge\nDebug (build flag): %t\nArgs: %+v`, VerboseOutput, flag.Args())

	// Start scdaemon in daemon mode
	daemon, err := StartScdaemon(context.Background())
	if err != nil {
		log.Fatalf("Failed to start scdaemon: %s", err)
	}
	defer daemon.Close()

	if flagDaemon || flagMultiServer {

		log.Println("Attaching relay to assuan listener")
		l, nonce, err := StartAssuanListener()
		if err != nil {
			log.Fatalf("Unable to start assuan listener: %s", err)
		}
		defer l.Close()

		go ProxyAssuanRequests(daemon, l, nonce)

	}

	if flagServer || flagMultiServer {

		log.Println("Attaching relay to stdin/stdout")
		// Open a relay for stdin and stdout
		go daemon.Connect(os.Stdin, os.Stdout)

	}

	daemon.cmd.Wait()

}

// StartRelay opens the relay binary on the Windows environment with the specified in, out and error channels.
// func StartRelay(relayBinary string, wg *sync.WaitGroup, stdin io.Reader, stdout io.Writer, stderr io.Writer) {

// 	defer wg.Done()

// 	cmd := exec.Command(relayBinary)
// 	cmd.Stdin = NewSocketNameMiddleware(stdin, stdout)
// 	cmd.Stdout = stdout
// 	cmd.Stderr = stderr

// 	if err := cmd.Start(); err != nil {
// 		log.Errorw("Failed to start relay", "error", err)
// 		return
// 	}

// 	if err := cmd.Wait(); err != nil {
// 		log.Errorw("Relay exited during operation", "error", err)
// 	}

// }

// SocketNameMiddleware implements io.Reader and overrides GETINFO socket_name, returning the
// bridged socket in the WSL space instead of the actual.
// type SocketNameMiddleware struct {
// 	input       io.Reader
// 	output      io.Writer
// 	overrideCmd []byte
// 	overrideVal []byte
// }

// NewSocketNameMiddleware creates a new SocketNameMiddleware for the provided input and output.
// func NewSocketNameMiddleware(input io.Reader, output io.Writer) SocketNameMiddleware {

// 	return SocketNameMiddleware{
// 		input:       input,
// 		output:      output,
// 		overrideCmd: []byte("GETINFO socket_name\n"),
// 		overrideVal: []byte(fmt.Sprintf("D %s\nOK\n", ScdaemonSocketName)),
// 	}

// }

// func (middleware SocketNameMiddleware) Read(p []byte) (int, error) {

// 	n, err := middleware.input.Read(p)
// 	if err != nil {
// 		return -1, err
// 	}

// 	if bytes.HasPrefix(p, middleware.overrideCmd) {
// 		middleware.output.Write(middleware.overrideVal)
// 		return middleware.Read(p)
// 	}

// 	return n, nil

// }
