// Package main contains the command for running OpenBar.
// It support a JSON runtime configuration allowing to output shell commands into
// the bar.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/syslog"
	"openbar"
	"openbar/modules/command"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args...); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[0], err)
		os.Exit(1)
	}
}

// Delegate work here because os.Exit would prevent deferred calls to be
// executed.
func run(args ...string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(args) < 2 {
		return fmt.Errorf("usage: %s PATH", args[0])
	}

	stderr, err := syslog.New(syslog.LOG_ERR, args[0])
	if err != nil {
		return err
	}

	opts, err := parse(args[1])
	if err != nil {
		return err
	}

	sigc := make(chan os.Signal, 1)

	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		defer cancel()
		<-sigc
	}()

	opts = append(
		opts,
		openbar.WithOutput(os.Stdout),
		openbar.WithError(stderr),
		openbar.WithJitter(2000),
	)

	return openbar.Run(ctx, opts...)
}

// Parse a JSON configuration file with each entry of the array being an object
// with `command` and `interval` defined.
func parse(path string) ([]openbar.Option, error) {
	fd, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	data, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	type entry struct {
		Command  []string `json:"command"`
		Interval string   `json:"interval"`
	}

	entries := make([]entry, 0)
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	res := make([]openbar.Option, len(entries))
	for i, e := range entries {
		duration, err := time.ParseDuration(e.Interval)
		if err != nil {
			return nil, err
		}

		res[i] = openbar.WithModuleFunc(
			command.New(e.Command...),
			duration,
		)
	}

	return res, nil
}
