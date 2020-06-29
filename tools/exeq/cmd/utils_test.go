package cmd

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestParseCmd(t *testing.T) {
	tests := []struct {
		args []string
		want ExeqCommand
	}{
		{[]string{"foo"},
			ExeqCommand{Name: "foo", Args: []string{}},
		},
		{
			[]string{"foo", "bar"},
			ExeqCommand{Name: "foo", Args: []string{"bar"}},
		},
		{
			[]string{"foo", "bar", "spam"},
			ExeqCommand{Name: "foo", Args: []string{"bar", "spam"}},
		},
		{
			[]string{"foo", ">/bar"},
			ExeqCommand{Name: "foo", Args: []string{}, StdoutFile: "/bar"},
		},
		{
			[]string{"foo", ">", "/bar"},
			ExeqCommand{Name: "foo", Args: []string{}, StdoutFile: "/bar"},
		},
		{
			[]string{"foo", "1>bar"},
			ExeqCommand{Name: "foo", Args: []string{}, StdoutFile: "bar"},
		},
		{
			[]string{"foo", "1>", "/bar"},
			ExeqCommand{Name: "foo", Args: []string{}, StdoutFile: "/bar"},
		},
		{
			[]string{"foo", "2>/bar"},
			ExeqCommand{Name: "foo", Args: []string{}, StderrFile: "/bar"},
		},
		{
			[]string{"foo", "2>", "/bar"},
			ExeqCommand{Name: "foo", Args: []string{}, StderrFile: "/bar"},
		},
		{
			[]string{"foo", ">/bar", "spam"},
			ExeqCommand{Name: "foo", Args: []string{"spam"}, StdoutFile: "/bar"},
		},
		{
			[]string{"foo", ">/out", "2>/err", "spam"},
			ExeqCommand{Name: "foo", Args: []string{"spam"}, StdoutFile: "/out", StderrFile: "/err"},
		},
		{
			[]string{"foo", ">", "/out", "2>", "/err", "spam"},
			ExeqCommand{Name: "foo", Args: []string{"spam"}, StdoutFile: "/out", StderrFile: "/err"},
		},
		{
			[]string{"foo", ">/out", "--life", "2", "-q", "2>", "/err", "spam"},
			ExeqCommand{Name: "foo", Args: []string{"--life", "2", "-q", "spam"}, StdoutFile: "/out", StderrFile: "/err"},
		},
	}

	for _, tc := range tests {
		name := tc.args[0]
		args := tc.args[1:]
		got, err := ParseExeqCommand(name, args)
		if err != nil {
			t.Errorf("ParseExeqCommand(%s, %v) returned an error: %v", name, args, err)
			continue
		}

		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("ParseRedisURI(%s, %v) = %+v, want %+v\n(-want,+got)\n%s", name, args, got, tc.want, diff)
		}
	}
}

func TestParseCmdErrors(t *testing.T) {
	tests := []struct {
		args []string
		err  error
		want ExeqCommand // if present, this is the eventual intended behavior
	}{
		{[]string{"foo>bar"},
			fmt.Errorf("unable to parse file redirect syntax for command, try adding space between command and redirect '%s' %v", "foo>bar", []string{}),
			ExeqCommand{Name: "foo", Args: []string{}, StdoutFile: "bar"},
		},
		{
			[]string{"foo", "-v>bar"},
			fmt.Errorf("unable to parse file redirect syntax for command, try adding space between command and redirect '%s' %v", "foo>bar", []string{}),
			ExeqCommand{Name: "foo", Args: []string{"-v"}, StdoutFile: "bar"},
		},
		{
			[]string{"foo", "|", "bar"},
			fmt.Errorf("pipe functionality not currently available'%s' %v", "foo", []string{"|", "bar"}),
			ExeqCommand{Name: "foo", Args: []string{"|", "bar"}, StdoutFile: "bar"},
		},
		{
			[]string{"foo|bar"},
			fmt.Errorf("pipe functionality not currently available'%s' %v", "foo|", []string{}),
			ExeqCommand{Name: "foo", Args: []string{"|", "bar"}, StdoutFile: "bar"},
		},
	}

	for _, tc := range tests {
		name := tc.args[0]
		args := tc.args[1:]
		_, err := ParseExeqCommand(name, args)
		if err == nil {
			t.Errorf("ParseExeqCommand(%s, %v) succeeded for malformed input, want error",
				name, args)
		}
	}
}
