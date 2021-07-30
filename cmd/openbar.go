// Package main contains the command for running OpenBar.
// It support a JSON runtime configuration allowing to output shell commands into
// the bar.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/syslog"
	"openbar"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		must(errors.New("usage: openbar <path>"))
	}

	stderr, err := syslog.New(syslog.LOG_ERR, os.Args[0])
	must(err)

	opts, err := parse(os.Args[1])
	must(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	)

	must(openbar.Run(ctx, opts...))
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
		opt, err := option(e.Command, e.Interval)
		if err != nil {
			return nil, err
		}
		res[i] = opt
	}

	return res, nil
}

// Create an option from an entry of the configuration file.
func option(cmd []string, interval string) (openbar.Option, error) {
	i, err := time.ParseDuration(interval)
	if err != nil {
		return nil, err
	}
	return openbar.WithModule(module(cmd), i), nil
}

// Create a bar module executing the given shell command.
func module(cmd []string) openbar.ModuleFunc {
	return func() (string, error) {
		//nolint:gosec
		out, err := exec.Command(cmd[0], cmd[1:]...).Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
}

// Exit on error.
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
