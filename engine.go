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
	Opts   EngineOptions

	OutputChan chan string
}

type EngineOptions struct {
	Depth   int
	MultiPV int
}

func NewEngine(enginePath string) (*Engine, error) {
	proc, err := NewProcess(enginePath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	opts := EngineOptions{
		Depth:   20,
		MultiPV: 1,
	}

	return &Engine{
		Path:       enginePath,
		Proc:       proc,
		Ctx:        ctx,
		Cancel:     cancel,
		Opts:       opts,
		OutputChan: make(chan string),
	}, nil
}

func (e *Engine) Run() error {
	if err := e.Proc.Start(); err != nil {
		return err
	}

	e.SendCommand("uci")
	go e.ReadOutput()

	return nil
}

func (e *Engine) SendCommand(command string, args ...any) {
	formattedCmd := fmt.Sprintf(command, args...)
	fmt.Fprintln(e.Proc.stdin, formattedCmd)
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

func (e *Engine) SetOption(name string, value any) error {
	switch name {
	case "depth":
		depth, ok := value.(int)
		if !ok {
			return fmt.Errorf(
				"option 'depth' requires integer value, got %T",
				value,
			)
		}
		e.Opts.Depth = depth

	case "multipv":
		mulitpv, ok := value.(int)
		if !ok {
			return fmt.Errorf(
				"option 'multipv' requires integer value, got %T",
				value,
			)
		} else if mulitpv > 256 || mulitpv < 1 {
			return fmt.Errorf(
				"multipv value must be between 1 and 256 (got %d)",
				mulitpv,
			)

		}
		e.Opts.MultiPV = mulitpv
		e.SendCommand("setoption name MultiPV value %d",
			e.Opts.MultiPV)

	default:
		return fmt.Errorf("invalid option '%s'", name)
	}
	return nil
}
