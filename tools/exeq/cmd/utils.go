// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"strings"
)

func UnpackCommand(t *asynq.Task) (cmd ExeqCommand, err error) {
	cmd.Name, err = t.Payload.GetString("name")
	if err != nil {
		return cmd, err
	}
	cmd.Args, err = t.Payload.GetStringSlice("args")
	if err != nil {
		return cmd, err
	}
	cmd.StdoutFile, err = t.Payload.GetString("stdoutFile")
	if err != nil {
		return cmd, err
	}
	cmd.StderrFile, err = t.Payload.GetString("stderrFile")
	if err != nil {
		return cmd, err
	}
	return
}

// this is a work in progress
func ParseExeqCommand(name string, args []string) (cmd ExeqCommand, err error) {
	cmd.Name = name
	i := 0
	arg := ""
	tmpArgs := []string{}
	newArgs := []string{}
	if len(strings.Split(name, ">")) > 1 {
		return cmd, fmt.Errorf("unable to parse file redirect syntax for command, try adding space between command and redirect '%s' %v", name, args)
	}
	if len(strings.Split(name, "|")) > 1 {
		return cmd, fmt.Errorf("pipe functionality not currently available'%s' %v", name, args)
	}

	// force input into a more standard syntax to make parsing easier, with space between > and filename
	for _, arg := range args {
		if len(strings.Split(arg, "|")) > 1 {
			return cmd, fmt.Errorf("pipe functionality not currently available'%s' %v", name, args)
		}
		switch len(arg) {
		case 0:
			// pass
		case 1:
			fallthrough
		case 2:
			tmpArgs = append(tmpArgs, arg)
		default:
			parts := strings.Split(arg, ">")
			switch len(parts) {
			case 1:
				tmpArgs = append(tmpArgs, arg)
			case 2:
				if len(parts[0]) == 0 { // `>/some/file`
					tmpArgs = append(tmpArgs, ">", parts[1])
				} else if len(parts[0]) == 1 { // `1>/some/file`
					if parts[0] == "1" || parts[0] == "2" {
						tmpArgs = append(tmpArgs, parts[0]+">", parts[1])
					}
				} else { // `ls>/some/file`, currently not parseable
					return cmd, fmt.Errorf("unable to parse file redirect syntax for command, try adding space between command and redirect '%s' %v", name, args)
				}
			case 3:
				if len(parts[0]) == 0 && len(parts[1]) == 0 { // >>/some/file
					tmpArgs = append(tmpArgs, ">>", parts[1])
				}
			default:
				return cmd, fmt.Errorf("unable to parse file redirect syntax for command '%s' %v", name, args)
			}

		}
	}
	// this feels super clunky but is probably the simplest way to deal with state. probably should use a state machine
	for i = 0; i < len(tmpArgs); i++ {
		arg = tmpArgs[i]
		switch arg {
		case ">":
			fallthrough
		case "1>":
			if cmd.StdoutFile != "" {
				return cmd, fmt.Errorf("duplicate stdout redirect for command '%s' %v", name, tmpArgs)
			}
			if len(tmpArgs) > i+1 {
				cmd.StdoutFile = tmpArgs[i+1]
			} else {
				return cmd, fmt.Errorf("unable to parse file redirect syntax for command '%s' %v", name, tmpArgs)
			}
			i++
		case "2>":
			if cmd.StderrFile != "" {
				return cmd, fmt.Errorf("duplicate stderr redirect for command '%s' %v", name, tmpArgs)
			}
			if len(tmpArgs) > i+1 {
				cmd.StderrFile = tmpArgs[i+1]
			} else {
				return cmd, fmt.Errorf("unable to parse file redirect syntax for command '%s' %v", name, tmpArgs)
			}
			i++
		case ">>":
			return cmd, fmt.Errorf(">> is currently not supported''")
		default:
			// try to catch some of the edge cases
			if len(arg) == 0 {
				continue
			}
			newArgs = append(newArgs, arg)
		}
	}
	cmd.Args = newArgs
	return
}

func NewExecCmd(ecmd ExeqCommand) *asynq.Task {
	return asynq.NewTask(TypenameExeqCommand, map[string]interface{}{
		"name": ecmd.Name, "args": ecmd.Args, "stdoutFile": ecmd.StdoutFile, "stderrFile": ecmd.StderrFile})
}

type BufferedWriter interface {
	io.Writer
	Flush() error
}

type logFileStruct struct {
	out   *os.File
	err   *os.File
	fnOut string
	fnErr string
}

type muxBufferedWriter struct {
	out multiBufferedWriter
	err multiBufferedWriter
}

type multiBufferedWriter struct {
	writers []BufferedWriter
}

// basically same as io.MultiWriter.Write(
func (t *multiBufferedWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

// like Flush() but for multiWriter
func (t *multiBufferedWriter) Flush() (err error) {
	for _, w := range t.writers {
		err = w.Flush()
		if err != nil {
			return
		}
	}
	return nil
}

func MultiWriter(writers ...BufferedWriter) BufferedWriter {
	allWriters := make([]BufferedWriter, 0, len(writers))
	for _, w := range writers {
		allWriters = append(allWriters, w)
	}
	return &multiBufferedWriter{allWriters}
}

// Closes all associated files. Catches and logs any errors
func (fls *logFileStruct) EClose() {
	if fls.out != nil {
		err := fls.out.Close()
		if err != nil {
			log.Error().Err(err).Str("stdoutFile", fls.fnOut).Msg("Failed to close file")
		}
	}
	if fls.err != nil {
		err := fls.err.Close()
		if err != nil {
			log.Error().Err(err).Str("stderrFile", fls.fnOut).Msg("Failed to close file")
		}
	}
}

// get optional os.Files from filenames
// Don't forget to defer Close() !
func getFileStruct(stdoutFile string, stderrFile string, allow_write bool) (fs *logFileStruct, err error) {
	if !allow_write {
		return &logFileStruct{}, nil
	}
	fs = &logFileStruct{}
	if stdoutFile != "" {
		fileOut, err := os.Create(stdoutFile)
		if err != nil {
			return nil, err
		}
		fs.out = fileOut
		fs.fnOut = stdoutFile
	}
	if stderrFile != "" {
		fileErr, err := os.Create(stderrFile)
		if err != nil {
			return nil, err
		}
		fs.err = fileErr
		fs.fnErr = stderrFile
	}
	return fs, nil
}

// sets up our output multiplexing
func getMultiWriters(fs *logFileStruct, echo bool) (w *muxBufferedWriter, err error) {
	w = &muxBufferedWriter{}
	if fs.out != nil {
		w.out.writers = append(w.out.writers, bufio.NewWriter(fs.out))
	}
	if fs.err != nil {
		w.err.writers = append(w.err.writers, bufio.NewWriter(fs.err))
	}
	if echo {
		w.out.writers = append(w.out.writers, bufio.NewWriter(os.Stdout))
		w.err.writers = append(w.err.writers, bufio.NewWriter(os.Stderr))
	}

	return w, nil
}
