package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const WkScdaemonSocketName = "S.scdaemon"

func GetWindowsScdaemonSocketFn() (string, error) {

	winSocketDir, err := gpgconfGetDirectory("gpgconf.exe", "socketdir")
	if err != nil {
		return "", fmt.Errorf("Failed to get win socketdir: %w", err)
	}

	socketDir, err := GetWslPath(strings.TrimSpace(strings.Replace(winSocketDir, "%3a", ":", 1)))
	if err != nil {
		return "", fmt.Errorf("Failed to convert win socketdir to WSL path: %w", err)
	}

	return filepath.Join(socketDir, WkScdaemonSocketName), nil

}

func GetWindowsScdaemonBinaryFn() (string, error) {

	binary, err := gpgconfGetBinary("gpgconf.exe", "scdaemon")
	if err != nil {
		return "", fmt.Errorf("Failed to get win scdaemon binary: %w", err)
	}

	return GetWslPath(strings.TrimSpace(strings.Replace(binary, "%3a", ":", 1)))

}

func GetLinuxScdaemonSocketFn() (string, error) {

	socketDir, err := gpgconfGetDirectory("gpgconf", "socketdir")
	if err != nil {
		return "", fmt.Errorf("Failed to get socketdir: %w", err)
	}

	return filepath.Join(socketDir, WkScdaemonSocketName), nil

}

func GetLinuxGnupgHomePath() (string, error) {

	homeDir, err := gpgconfGetDirectory("gpgconf", "homedir")
	if err != nil {
		return "", fmt.Errorf("Failed to get homedir: %w", err)
	}

	return homeDir, nil

}

// GetWslPath converts a windows path into a WSL path.
func GetWslPath(path string) (string, error) {
	res, err := exec.Command("wslpath", "-a", path).Output()
	if err != nil {
		return "", fmt.Errorf("Error while running wslpath cmd: %w", err)
	}
	return strings.TrimSpace(string(res)), err
}

// gpgconfGetDirectory gets the specified key from the gpgconf --list-dirs command output.
func gpgconfGetDirectory(gpgconf, key string) (string, error) {
	return gpgconfGetKey(gpgconf, key, "--list-dirs")
}

// gpgconfGetBinary gets the binary with the specified key from gpgconf.
func gpgconfGetBinary(gpgconf, key string) (string, error) {
	return gpgconfGetKey(gpgconf, key)
}

// gpgconfGetKey runs gpgconf with the provided args and returns the value of the key provided.
func gpgconfGetKey(gpgconf string, key string, args ...string) (string, error) {

	cmd := exec.Command(string(gpgconf), args...)
	res, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Error running gpgconf: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(res))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		tokens := strings.SplitN(scanner.Text(), ":", 3)
		if tokens[0] == key {
			return tokens[len(tokens)-1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("Error scanning gpgconf output: %w", err)
	}

	return "", fmt.Errorf("Key was not found")
}

// GetResolvAddr gets the Windows host IP address (to support WSL2) in a very hacky way...
func GetResolvAddr() (string, error) {

	cmd := exec.Command("awk", "/nameserver / {print $2; exit}", "/etc/resolv.conf")
	res, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Unable to get windows host addr: %w", err)
	}
	return strings.TrimSpace(string(res)), nil

}
