package gofish

import (
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
}

func NewProcess(path string) (*Process, error) {
	if path == "" {
		return nil, fmt.Errorf("engine path cannot be empty")
	}

	if info, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("engine not found at path: %s", path)
		}
		return nil, fmt.Errorf("cannot access engine at %s: %w", path, err)
	} else if info.IsDir() {
		return nil, fmt.Errorf("engine path is a directory: %s", path)
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
	}, nil
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

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
	if p.isClosed {
		p.mu.Unlock()
		return nil
	}
	p.isClosed = true
	p.mu.Unlock()

	fmt.Fprintln(p.stdin, "quit")
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

	p.cleanupResources()
	return mainErr
}

func (p *Process) cleanupResources() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stdin != nil {
		p.stdin.Close()
		p.stdin = nil
	}
	if p.stdout != nil {
		p.stdout.Close()
		p.stdout = nil
	}
	if p.stderr != nil {
		p.stderr.Close()
		p.stderr = nil
	}
}
