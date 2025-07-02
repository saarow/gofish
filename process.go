// Package gofish
package gofish

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu     sync.Mutex
	closed bool
}

func NewProcess(enginePath string) (*Process, error) {
	if enginePath == "" {
		return nil, fmt.Errorf("engine path cannot be empty")
	}

	cmd := exec.Command(enginePath)

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

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()

		return nil, fmt.Errorf(
			"failed to start the engine '%s': %w",
			enginePath,
			err,
		)
	}

	return &Process{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

func (p *Process) closePipes() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	if p.stderr != nil {
		p.stderr.Close()
	}
}

func (p *Process) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	if p.stdin != nil {
		fmt.Fprintln(p.stdin, "quit")
	}

	var err error
	if p.cmd != nil {
		done := make(chan error, 1)
		defer close(done)
		go func() {
			done <- p.cmd.Wait()
		}()

		select {
		case err = <-done:
		case <-time.After(3 * time.Second):
			if p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
			err = fmt.Errorf(
				"engine did not respond to quit command, force killed",
			)
		}
	}

	p.closePipes()
	return err
}
