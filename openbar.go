// Package openbar is a simple status command for SwayWM.
// It implements sway-protocol(7).
package openbar

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"
	"syscall"
	"time"
)

// Header is the bar header according to sway-protocol(7).
type Header struct {
	Version     int  `json:"version"`
	ClickEvents bool `json:"click_events"`
	ContSignal  int  `json:"cont_signal"`
	StopSignal  int  `json:"stop_signal"`
}

var defaultHeader = Header{
	Version:     1,
	ClickEvents: false,
	ContSignal:  int(syscall.SIGCONT),
	StopSignal:  int(syscall.SIGSTOP),
}

// Block is one entry of the bar body according to sway-protocol(7).
// Only the required field is implemented.
type Block struct {
	FullText string `json:"full_text"`
}

// Module is a bar module that emits the content of a block.
type Module interface {
	FullText() (string, error)
}

// ModuleFunc is a function for the single-method interface Module.
type ModuleFunc func() (string, error)

// FullText implements Module for ModuleFunc.
func (f ModuleFunc) FullText() (string, error) {
	return f()
}

// Run starts emitting the JSON infinite array with the given configuration.
func Run(ctx context.Context, opts ...Option) error {
	cfg := new(config)

	// Parse configuration options.
	for _, opt := range opts {
		opt(cfg)
	}

	// If we can't print headers, exit early to avoid having already started
	// multiple goroutines that will leak.
	if err := print(cfg.out, defaultHeader, 0x0A, 0x5B); err != nil {
		return err
	}

	n := len(cfg.cells)

	// Create the scheduler and wait for all workers to terminate before
	// closing the output channel.
	scheduler := bootstrap(n)

	// Start one worker per module. This allows us to have variable refresh rate
	// for each and every one of them.
	for i, c := range cfg.cells {
		go scheduler.update(ctx, i, c.module, c.interval)
	}

	b := make([]Block, n)

	// Each time a screen update is required, mutate the bar body and print the new
	// output inside the infinite JSON array. No error handling here because we
	// don't want to prevent other modules from working.
	for res := range scheduler.out {
		b[res.idx].FullText = res.out
		debug(res.err)
		debug(print(cfg.out, b, 0x2C))
	}

	return nil
}

// A scheduler is responsible for coordination of the asynchronous updates for each
// module. Each time an update occurs, it is written to the scheduler's output channel.
type scheduler struct {
	wg  *sync.WaitGroup
	out chan result
}

// The result of a module update holding the module index and data to be
// printed as well as any processing error.
type result struct {
	idx int
	out string
	err error
}

// Create a scheduler of the given size.
func bootstrap(size int) scheduler {
	wg := new(sync.WaitGroup)
	wg.Add(size)

	out := make(chan result, size)

	go func() {
		defer close(out)
		wg.Wait()
	}()

	return scheduler{wg, out}
}

// Start a goroutine that will write result of a module at regular intervals. A
// first processing is performed on first call of this function to allow initial
// print of the bar.
func (s scheduler) update(ctx context.Context, i int, m Module, d time.Duration) {
	defer s.wg.Done()

	s.do(i, m)

	tck := time.NewTicker(d)

	for {
		select {
		case <-ctx.Done():
			return
		case <-tck.C:
			s.do(i, m)
		}
	}
}

// Process module output and write the result to the output channel.
func (s scheduler) do(idx int, m Module) {
	out, err := m.FullText()
	s.out <- result{idx, out, err}
}

// Print a log entry if there is an error.
func debug(err error) {
	if err != nil {
		log.Println(err)
	}
}

// Marshal the given value to JSON, concatenate additional trailing bytes and
// write them to the writer.
func print(w io.Writer, v interface{}, glue ...byte) error {
	json, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(json, glue...)); err != nil {
		return err
	}
	return nil
}

// This struct holds the global configuration.
type config struct {
	out   io.Writer
	cells []cell
}

// A cell is a module and the interval at which it must be updated.
type cell struct {
	module   Module
	interval time.Duration
}

// Option is an application setting.
type Option func(*config)

// WithOutput configures the output for the JSON data.
func WithOutput(w io.Writer) Option {
	return func(cfg *config) {
		cfg.out = w
	}
}

// WithError configures the output for the log entries.
func WithError(w io.Writer) Option {
	return func(cfg *config) {
		log.SetOutput(w)
	}
}

// WithModule configures a module. Modules are printed in the order they are
// passed through this function.
func WithModule(module Module, interval time.Duration) Option {
	return func(cfg *config) {
		cfg.cells = append(cfg.cells, cell{module, interval})
	}
}

// WithModuleFunc configures a module from an anonymous function.
func WithModuleFunc(f func() (string, error), interval time.Duration) Option {
	return WithModule(ModuleFunc(f), interval)
}
