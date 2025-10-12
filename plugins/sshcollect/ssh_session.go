
package sshcollect

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// InteractiveSession manages a stateful SSH shell session.
type InteractiveSession struct {
	Client  *ssh.Client
	Session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
}

// Connect establishes an SSH connection.
func (s *InteractiveSession) Connect(user, pass, host string, port int) error {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	s.Client = client
	return nil
}

// Shell starts a shell and sets up I/O pipes.
func (s *InteractiveSession) Shell() error {
	session, err := s.Client.NewSession()
	if err != nil {
		return err
	}
	s.Session = session

	s.Stdin, err = session.StdinPipe()
	if err != nil {
		return err
	}
	s.Stdout, err = session.StdoutPipe()
	if err != nil {
		return err
	}

	// Start the remote shell
	return session.Shell()
}

// Close cleans up the session and client connection.
func (s *InteractiveSession) Close() {
	if s.Session != nil {
		s.Session.Close()
	}
	if s.Client != nil {
		s.Client.Close()
	}
}

// Send writes a command to the shell's stdin.
func (s *InteractiveSession) Send(cmd string) error {
	_, err := s.Stdin.Write([]byte(cmd + "\n"))
	return err
}

// WaitFor reads from stdout until a regex pattern is matched or a timeout occurs.
func (s *InteractiveSession) WaitFor(pattern string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	var output strings.Builder
	reader := bufio.NewReader(s.Stdout)
	
	// Channel to signal when a match is found
	matchChan := make(chan bool, 1)
	errChan := make(chan error, 1)

	go func() {
		for {
			rune, _, err := reader.ReadRune()
			if err != nil {
				errChan <- err
				return
			}
			output.WriteRune(rune)
			if re.MatchString(output.String()) {
				matchChan <- true
				return
			}
		}
	}()

	select {
	case <-matchChan:
		return output.String(), nil
	case err := <-errChan:
		return output.String(), err
	case <-time.After(15 * time.Second):
		return output.String(), fmt.Errorf("timeout waiting for pattern: %s", pattern)
	}
}

