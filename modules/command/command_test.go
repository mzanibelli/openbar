package command_test

import (
	"errors"
	"fmt"
	"openbar/modules/command"
	"testing"
)

func TestCommand(t *testing.T) {
	tests := []struct {
		cmd []string
		out string
		err error
	}{
		{
			cmd: []string{"echo", "foo"},
			out: " foo ",
			err: nil,
		},
		{
			cmd: []string{"/bin/true"},
			out: "",
			err: nil,
		},
		{
			cmd: []string{"/bin/false"},
			out: "",
			err: errors.New("exit status 1"),
		},
		{
			cmd: []string{"cat", ""},
			out: "",
			err: errors.New("exit status 1: cat: '': No such file or directory"),
		},
		{
			cmd: []string{"sh", "-c", "echo foo; echo bar;"},
			out: " foo ",
			err: nil,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			cmd := command.New(test.cmd...)

			out, err := cmd()

			if out != test.out {
				t.Errorf("want: %q, got: %q", test.out, out)
			}
			if !comp(err, test.err) {
				t.Errorf("want: %v, got: %v", test.err, err)
			}
		})
	}
}

func comp(a, b error) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil && b != nil:
		return false
	case a != nil && b == nil:
		return false
	default:
		return a.Error() == b.Error()
	}
}
