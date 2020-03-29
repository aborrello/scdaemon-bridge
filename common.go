package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// ScdaemonSocketFilename is the WK filename for the scdaemon socket descriptor.
const ScdaemonSocketFilename = "S.scdaemon"

// GpgConfBinary represents a gpgconf executable for either Windows or Linux
type GpgConfBinary string

var (
	gpgconfLinux GpgConfBinary = "gpgconf"
	gpgconfWin   GpgConfBinary = "gpgconf.exe"

	// ScdaemonSocketName is a runtime generated filepath to the WK location of the scdaemon socket descriptor.
	ScdaemonSocketName string
	log                *zap.SugaredLogger
	outputSink         io.Writer
)

func init() {
	outputSink = getOutputSink()
	log = getLogger()
}

// GetWindowsPath converts gpgconf output into a valid windows path.
func GetWindowsPath(path string) string {
	return strings.TrimSpace(strings.Replace(path, "%3a", ":", 1))
}

// GetWslPath converts a windows path into a WSL path.
func GetWslPath(path string) string {
	res, err := exec.Command("wslpath", "-a", path).Output()
	if err != nil {
		log.Fatalw("Unable to get WSL path", "path", path, "error", err)
	}
	return strings.TrimSpace(string(res))
}

// GetDirectory gets the specified key from the gpgconf --list-dirs command output.
func (gpgconf GpgConfBinary) GetDirectory(key string) (string, error) {
	return gpgconf.GetKey(key, "--list-dirs")
}

// GetBinary gets the binary with the specified key from gpgconf.
func (gpgconf GpgConfBinary) GetBinary(key string) (string, error) {
	return gpgconf.GetKey(key)
}

// GetKey runs gpgconf with the provided args and returns the value of the key provided.
func (gpgconf GpgConfBinary) GetKey(key string, args ...string) (string, error) {

	cmd := exec.Command(string(gpgconf), args...)
	res, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Error running gpgconf: %w", err)
	}

	prefix := fmt.Sprintf("%s:", key)
	scanner := bufio.NewScanner(bytes.NewReader(res))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), prefix) {
			return strings.TrimPrefix(scanner.Text(), prefix), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("Error scanning gpgconf output: %w", err)
	}

	return "", fmt.Errorf("Key was not found")
}
