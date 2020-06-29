// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// serverCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Start an exeq server",
	Long:    `todo`,
	Aliases: []string{"s", "server"},
	Run:     serve,
}

func init() {
	serveCmd.PersistentFlags().StringVarP(&serveQueues, "serveQueues", "S", "exeq", "Serve on specific queue(s), ;-delimited. Can set priority e.g. 'foo;bar=5' (default queue is exeq)")
	serveCmd.PersistentFlags().BoolP("privileged", "P", false, "Whitelist all commands - MAY BE RISKY")
	serveCmd.PersistentFlags().BoolP("allow_write", "W", false, "Allow file redirect in command - MAY BE RISKY")
	serveCmd.PersistentFlags().BoolP("echo", "e", false, "echo subprocess values to stdout/stderr")
	serveCmd.PersistentFlags().IntP("jobs", "j", runtime.NumCPU(), "run n jobs in parallel (default value depends on your device)")
	viper.BindPFlag("serveQueues", serveCmd.PersistentFlags().Lookup("serveQueues"))
	rootCmd.AddCommand(serveCmd)

}


func serve(cmd *cobra.Command, args []string) {
	privileged, _ := cmd.Flags().GetBool("privileged")
	allow_write, _ := cmd.Flags().GetBool("allow_write")
	nJobs, _ := cmd.Flags().GetInt("jobs")
	echo, _ := cmd.Flags().GetBool("echo")

	uri := viper.GetString("uri")
	db := viper.GetInt("db")
	password := viper.GetString("password")
	serveQueues_s := viper.GetString("serveQueues")
	serveQueues := strings.Split(serveQueues_s, ";")

	queues := map[string]int{} // todo: improve dedicated command queue handling
	for _, q := range serveQueues {
		parts := strings.Split(q, "=")
		priority := 5
		queueName := ""
		if len(parts) > 2 {
			log.Error().Str("string", q).Msg("Could not parse queue priority, too many parts")
		} else if len(parts) == 2 {
			i, err := strconv.Atoi(parts[1])
			if err != nil {
				log.Error().Err(err).Str("string", q).Msg("Could not parse queue priority, failed to parse int")
			} else {
				priority = i
				queueName = parts[0]
			}
		} else {
			queueName = parts[0]
		}
		queues[queueName] = priority
		log.Info().Str("queue", queueName).Int("priority", priority).Msg("registered")
	}
	if len(queues) == 0 {
		log.Fatal().Msg("No queues registered, check the serveQueues option")
	}

	whitelist := args

	if !privileged && len(whitelist) == 0 {
		log.Fatal().Msg("No whitelisted executables and privileged flag not set")
	}

	r := asynq.RedisClientOpt{Addr: uri, DB: db, Password: password}
	srv := asynq.NewServer(r, asynq.Config{
		Concurrency: nJobs,
		Queues:      queues,
	})
	checkCommand := func(name string) error {
		return CheckCommand(name, privileged, whitelist)
	}
	handler := func(ctx context.Context, t *asynq.Task) error {
		return HandleExeqCommand(ctx, t, checkCommand, echo, allow_write)
	}
	log.Info().Str("uri", uri).
		Int("workers", nJobs).
		Bool("privileged", privileged).
		Msg("Starting exeq server")

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypenameExeqCommand, handler) // handles any task which matches the prefix exec:command
	log.Debug().Str("taskname", TypenameExeqCommand).
		Strs("whitelist", whitelist).
		Msg("registered")

	if err := srv.Run(mux); err != nil {
		log.Fatal().Err(err)
	}
}

// Determine if command is allowed to be run.
// This is a security feature
func CheckCommand(name string, privileged bool, whitelist []string) error {
	if privileged {
		return nil
	}
	isWhitelisted := false
	// todo: actually make a config/interface for this, also add to some sort of check
	for _, s := range whitelist {
		if s == name {
			isWhitelisted = true
		}
	}
	if isWhitelisted {
		return nil
	}
	return fmt.Errorf("`%s` is not a whitelisted executable. Allowed: %v\n", name, whitelist)
}

// This is the primary callback for processing commands
// It must be curried to a func of (Context, Task) -> (error) to be accepted by the server callback handler.
func HandleExeqCommand(ctx context.Context, t *asynq.Task, checkCommand func(string) error, echo bool, allow_write bool) error {
	ecmd, err := UnpackCommand(t)
	if err != nil {
		return err
	}
	err = checkCommand(ecmd.Name)

	if err != nil {
		return err
	}
	log.Info().Str("name", ecmd.Name).Strs("args", ecmd.Args).
		Str("stdout", ecmd.StdoutFile).Str("stderr", ecmd.StderrFile).Msg("Run command")

	var wg sync.WaitGroup
	cmd := exec.Command(ecmd.Name, ecmd.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if echo || allow_write {
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		fStruct, err := getFileStruct(ecmd.StdoutFile, ecmd.StderrFile, allow_write)
		if err != nil {
			return err
		}
		defer fStruct.EClose()

		w, err := getMultiWriters(fStruct, echo)
		if err != nil {
			return err
		}

		chOut := make(chan []byte)
		chErr := make(chan []byte)
		defer close(chOut)
		defer close(chErr)

		wg.Add(2)
		go ScannerChannel(stdout, chOut, &wg)
		go ScannerChannel(stderr, chErr, &wg)

		done := make(chan struct{})
		defer close(done)

		cmd.Start()
		// Mirror stdout/stderr to screen and/or file. This could use some work
		go func() {
			var chunk_out, chunk_err []byte
			for {

				select {
				case <-done:
					return
				case <-ctx.Done():
					err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					if err != nil {
						log.Warn().Int("pid", cmd.Process.Pid).Err(err).Msg("Error killing subprocess during context cancel")
					}
					return
				case chunk_out = <-chOut:
					_, err := w.out.Write(chunk_out)
					if err != nil {
						log.Panic().Err(err).Msg("Error writing to stdout files")
					}
				case chunk_err = <-chErr:
					_, err := w.err.Write(chunk_err)
					if err != nil {
						log.Panic().Err(err).Msg("Error writing to stderr files")
					}
				}
				chunk_out = nil
				chunk_err = nil
				w.out.Flush()
				w.err.Flush()
			}
		}()
	} else {
		cmd.Start()
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				log.Warn().Int("ExitStatus", status.ExitStatus()).Msg("Exit Status")
			}
		} else {
			log.Warn().Err(err).Msg("cmd.Wait error")
		}
	}
	wg.Wait()
	log.Debug().Str("name", ecmd.Name).Msg("complete")
	return nil
}

// ScanLines is a split function for a Scanner that returns each line of
// text, stripped of any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `(\r\n|\r|\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
func ScanRLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// todo: swap with regex
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0 : i+1], nil
	}
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		// We have a \r terminated line
		return i + 1, data[0 : i+1], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func ScannerChannel(r io.Reader, c chan<- []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Split(ScanRLines)
	for {
		if !scanner.Scan() {
			break
		}
		c <- scanner.Bytes()
	}
}
