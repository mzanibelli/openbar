// Package openbar is a simple status command for SwayWM.
// It implements sway-protocol(7).
package openbar

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
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
	if err := write(cfg.out, defaultHeader, 0x0A, 0x5B); err != nil {
		return err
	}

	n := len(cfg.cells)

	// Create the scheduler and wait for all workers to terminate before
	// closing the output channel.
	scheduler := bootstrap(n)

	// Start one worker per module. This allows us to have variable refresh rate
	// for each and every one of them.
	for i, c := range cfg.cells {
		go scheduler.update(ctx, i, c.module, c.interval, jitter(cfg.jitter))
	}

	b := make([]Block, n)

	// Each time a screen update is required, mutate the bar body and print the new
	// output inside the infinite JSON array. No error handling here because we
	// don't want to prevent other modules from working.
	for res := range scheduler.out {
		b[res.idx].FullText = res.out
		debug(res.err)
		debug(write(cfg.out, b, 0x2C))
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

const (
	broadcast = syscall.SIGUSR1 // Reload all modules.
	sigRtMin  = 0x22            // Minimum reload signal value for a single module.
	sigRtMax  = 0x40            // Maximum reload signal value for a single module.
)

// The function responsible for periodically updating cells. It performs an
// initial execution delayed with a random jitter to spread the load upon booting
// Sway. Then, modules are updated according to their respective intervals or when
// a signal is received. A SIGUSR1 signal will trigger a refresh for all modules
// whereas each module can be individually reloaded with SIGRTMIN+i.
func (s scheduler) update(ctx context.Context, i int, m Module, d, j time.Duration) {
	defer s.wg.Done()

	s.wait(i)

	t1 := time.NewTimer(j)
	defer t1.Stop()

	// Initialize the ticker with a higher interval than the jitter timer to allow
	// first paint to only be triggered by the timer. Then, receiving on the timer
	// channel will reset the ticker's duration to its normal value.
	t2 := time.NewTicker(j + 1)
	defer t2.Stop()

	sigc, id := make(chan os.Signal, 1), sigRtMin+((i+1)%sigRtMax)
	signal.Notify(sigc, broadcast, syscall.Signal(id))
	defer close(sigc)
	defer signal.Stop(sigc)

	for {
		select {
		case <-ctx.Done():
			return

		// A normal tick occurs.
		case <-t2.C:

		// When the jitter timer finishes, reset the ticker so the jitter offset
		// affects future updates. This avoids having modules with the same interval
		// updating exactly at the same time (and also sets the correct ticker interval
		// which was temporarily overridden at initialization phase).
		case <-t1.C:
			t2.Reset(d)

		// When activating a manual refresh for all modules, spread execution with
		// jitter and cancel upcoming ticks by resetting the timer. This avoids performing
		// the update twice in a row. Since jitter can span a few seconds, display a text
		// showing to the user the module is reloading. For single module reloads,
		// simply execute as fast as possible to minimize the time to visual feedback
		// as this feature is often used to match another action that happened in the
		// system (ie. user changed volume, we want to update the volume cell without any
		// other visual artifact, we don't care about doing this twice).
		case sig := <-sigc:
			if sig != broadcast {
				break
			}
			s.wait(i)
			time.Sleep(j)
			t2.Reset(d)
		}

		s.do(i, m)
	}
}

// Process module output and write the result to the output channel.
func (s scheduler) do(idx int, m Module) {
	out, err := m.FullText()
	s.out <- result{idx, out, err}
}

const placeholder = "..."

// Display a placeholder to inform user refresh instruction has been received.
func (s scheduler) wait(idx int) {
	s.out <- result{idx, placeholder, nil}
}

var initRand sync.Once

// Return a random duration lesser than the given maximum.
func jitter(max int) time.Duration {
	if max == 0 {
		return 0
	}
	initRand.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
	//nolint:gosec
	return time.Duration(rand.Intn(max)) * time.Millisecond
}

// Print a log entry if there is an error.
func debug(err error) {
	if err != nil {
		log.Println(err)
	}
}

// Marshal the given value to JSON, concatenate additional trailing bytes and
// write them to the writer.
func write(w io.Writer, v interface{}, glue ...byte) error {
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
	out    io.Writer
	jitter int
	cells  []cell
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

// WithJitter configures the maximum time (in ms) over which modules will delay
// their updates.
func WithJitter(jitter int) Option {
	return func(cfg *config) {
		cfg.jitter = jitter
	}
}
