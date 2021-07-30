package command

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// New returns a new command module.
func New(args ...string) func() (string, error) {
	return func() (string, error) {
		return do(args...)
	}
}

func do(args ...string) (string, error) {
	//nolint:gosec
	cmd := exec.Command(args[0], args[1:]...)

	// Buffer standard output and standard error to allow later processing.
	stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	// If the command fails, include full error in message.
	if err := cmd.Run(); err != nil {
		return "", verbose(err, line(stderr))
	}

	return pad(line(stdout)), nil
}

// Read the first line of text until carriage return or EOF.
// Panic if any other error occurs.
func line(b *bytes.Buffer) string {
	res, err := b.ReadString(0x0A)
	if err != nil && !errors.Is(err, io.EOF) {
		panic(err)
	}
	return res
}

// Add information to an error, only if it's not empty.
func verbose(err error, info string) error {
	clean := strings.TrimSpace(info)
	switch clean {
	case "":
		return err
	default:
		return fmt.Errorf("%w: %s", err, clean)
	}
}

// Pad string with leading and trailing spaces for readability.
func pad(v string) string {
	clean := strings.TrimSpace(v)
	switch clean {
	case "":
		return ""
	default:
		return fmt.Sprintf(" %s ", clean)
	}
}
