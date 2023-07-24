package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {
	var connStr string
	flag.StringVar(&connStr, "conn", "", "Connection string in the format 'username@hostname:port'")
	flag.Parse()

	// Check if the positional argument is provided
	args := flag.Args()
	if len(args) > 0 {
		// Use the first positional argument as the connection string
		connStr = args[0]
	}

	// Check if the connection string is provided
	if connStr == "" {
		fmt.Println("Connection string not provided. Please use '-conn' flag or positional argument 'username@hostname:port'.")
		return
	}

	// Split the connection string into username, hostname, and optional port
	parts := strings.Split(connStr, "@")
	if len(parts) != 2 {
		fmt.Println("Invalid connection string format. Please use 'username@hostname:port'.")
		return
	}

	user := parts[0]
	hostWithPort := parts[1]

	var host string
	var port int

	// Split the host string to separate hostname and port
	hostParts := strings.Split(hostWithPort, ":")
	host = hostParts[0]

	// If a port is specified, parse it. Otherwise, use the default port 22.
	if len(hostParts) > 1 {
		port, _ = strconv.Atoi(hostParts[1])
	} else {
		port = 22 // Default SSH port
	}

	var err error

	// Connect to the SSH agent
	sshAgentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer sshAgentConn.Close()

	agentClient := agent.NewClient(sshAgentConn)

	hostkeyCallback := ssh.InsecureIgnoreHostKey()

	conf := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: hostkeyCallback,
		Auth: []ssh.AuthMethod{
			// Use the private keys from the SSH agent for authentication
			ssh.PublicKeysCallback(agentClient.Signers),
		},
	}

	var conn *ssh.Client

	conn, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), conf)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer conn.Close()

	var session *ssh.Session
	session, err = conn.NewSession()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer session.Close()

	var stdin io.WriteCloser
	var stdout, stderr io.Reader

	stdin, err = session.StdinPipe()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	stdout, err = session.StdoutPipe()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	stderr, err = session.StderrPipe()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	wr := make(chan []byte, 10)

	go func() {
		for d := range wr {
			_, err := stdin.Write(d)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// Wait for the session to finish
	err = session.Run("ls -l")
	if err != nil {
		fmt.Println(err.Error())
	}
}
