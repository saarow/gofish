package gofish

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
)

type Engine struct {
	Path   string
	Proc   *Process
	Ctx    context.Context
	Cancel context.CancelFunc

	OutputChan chan string
}

func NewEngine(enginePath string) (*Engine, error) {
	proc, err := NewProcess(enginePath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		Path:       enginePath,
		Proc:       proc,
		Ctx:        ctx,
		Cancel:     cancel,
		OutputChan: make(chan string),
	}, nil
}

func (e *Engine) Run() error {
	if err := e.Proc.Start(); err != nil {
		return err
	}
	return nil
}

func (e *Engine) SendCommand(command string) {
	fmt.Fprintln(e.Proc.stdin, command)
}

func (e *Engine) ReadOutput() {
	scanner := bufio.NewScanner(e.Proc.stdout)

	for scanner.Scan() {
		e.OutputChan <- scanner.Text()
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		// TODO: error handling
		return
	}
}
