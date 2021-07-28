package openbar_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"openbar"
	"sync"
	"testing"
	"time"
)

func TestOpenBar(t *testing.T) {
	wg := new(sync.WaitGroup)

	wg.Add(1)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	module := openbar.ModuleFunc(func() (string, error) {
		return "hello", nil
	})

	go func() {
		defer wg.Done()
		if err := openbar.Run(
			ctx,
			openbar.WithOutput(stdout),
			openbar.WithError(stderr),
			openbar.WithModule(module, 10*time.Hour),
		); err != nil {
			t.Error(err)
		}
	}()

	cancel()

	wg.Wait()

	if stderr.String() != "" {
		t.Error(stderr.String())
	}

	// Remove the last comma and close the infinite array.
	stdout.Truncate(stdout.Len() - 1)
	stdout.WriteByte(0x5D)

	t.Log(stdout.String())

	line1, err := stdout.ReadBytes(0x0A)
	if err != nil {
		t.Error(err)
	}

	line2, err := stdout.ReadBytes(0x0A)
	if !errors.Is(err, io.EOF) {
		t.Error(err)
	}

	if !json.Valid(line1) {
		t.Error("invalid header")
	}

	if !json.Valid(line2) {
		t.Error("invalid body")
	}
}
