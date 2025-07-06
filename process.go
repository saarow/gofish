// Package gofish
package gofish

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	enginePath string
	mu         sync.Mutex
	isClosed   bool

	outputCh chan string
	errorCh  chan error
}

func NewProcess(path string) (*Process, error) {
	if path == "" {
		return nil, fmt.Errorf("engine path cannot be empty")
	}

	cmd := exec.Command(path)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()

		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()

		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	return &Process{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		enginePath: path,
		outputCh:   make(chan string),
		errorCh:    make(chan error),
	}, nil
}

func (p *Process) Start() error {
	if err := p.cmd.Start(); err != nil {
		p.cleanupResources()
		return fmt.Errorf(
			"failed to start the engine '%s': %w",
			p.enginePath,
			err,
		)
	}

	return nil
}

func (p *Process) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isClosed {
		return nil
	}

	p.Write("quit")
	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- p.cmd.Wait()
	}()

	var mainErr error
	select {
	case err := <-done:
		if err != nil {
			mainErr = fmt.Errorf(
				"engine exited with an error during shutdown: %w",
				err,
			)
		}

	case <-time.After(3 * time.Second):
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		mainErr = fmt.Errorf(
			"engine did not respond to quit command, force killed",
		)
	}

	p.isClosed = true
	p.cleanupResources()
	return mainErr
}

func (p *Process) Write(command string) error {
	if !p.isClosed {
		_, err := fmt.Fprintln(p.stdin, command)
		return err
	}
	return fmt.Errorf("either stdin pipe or process is closed")
}

func (p *Process) Read() {
	scanner := bufio.NewScanner(p.stdout)

	for scanner.Scan() {
		p.outputCh <- scanner.Text()
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		p.errorCh <- err
	}
}

func (p *Process) cleanupResources() {
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.stderr != nil {
		p.stderr.Close()
	}

	close(p.outputCh)
	close(p.errorCh)
}
