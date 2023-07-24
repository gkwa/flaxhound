package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var enableLogging bool

func init() {
	flag.BoolVar(&enableLogging, "log", false, "Enable logging")
	flag.Parse()
}

func main() {
	var connStr string
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

	// Enable logging if the --log flag is provided
	if enableLogging {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Printf("Connecting to %s@%s:%d...", user, host, port)
	}

	// Connect to the SSH agent
	sshAgentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		log.Println("Failed to connect to SSH agent:", err)
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
		log.Println("Failed to establish SSH connection:", err)
		return
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		log.Println("Failed to create SSH session:", err)
		return
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Println("Failed to create stdin pipe:", err)
		return
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Println("Failed to create stdout pipe:", err)
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		log.Println("Failed to create stderr pipe:", err)
		return
	}

	wr := make(chan []byte, 10)

	go func() {
		for d := range wr {
			_, err := stdin.Write(d)
			if err != nil {
				log.Println("Failed to write to stdin:", err)
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

	if enableLogging {
		log.Println("SSH connection established. Running command: ls -l")
	}

	// Wait for the session to finish
	err = session.Run("ls -l")
	if err != nil {
		log.Println("Failed to run command:", err)
	}
}
