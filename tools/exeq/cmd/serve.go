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
	"os"
	"os/exec"
	"runtime"
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
	rootCmd.AddCommand(serveCmd)
	rootCmd.PersistentFlags().BoolP("privileged", "P", false, "Whitelist all commands - MAY BE RISKY")
	rootCmd.PersistentFlags().BoolP("echo", "e", false, "echo subprocess values to stdout/stderr")
	rootCmd.PersistentFlags().IntP("jobs", "j", runtime.NumCPU(), "run n jobs in parallel (default value depends on your device)")

}

func serve(cmd *cobra.Command, args []string) {
	privileged, _ := cmd.Flags().GetBool("privileged")
	nJobs, _ := cmd.Flags().GetInt("jobs")
	echo, _ := cmd.Flags().GetBool("echo")

	uri := viper.GetString("uri")
	db := viper.GetInt("db")
	password := viper.GetString("password")

	whitelist := args

	r := asynq.RedisClientOpt{Addr: uri, DB: db, Password: password}
	srv := asynq.NewServer(r, asynq.Config{
		Concurrency: nJobs,
	})
	checkCommand := func(name string) error {
		return CheckCommand(name, privileged, whitelist)
	}
	handler := func(ctx context.Context, t *asynq.Task) error {
		return HandleExeqCommand(ctx, t, checkCommand, echo)
	}
	log.Info().Str("uri", uri).
		Int("workers", nJobs).
		Bool("privileged", privileged).
		Strs("whitelist", whitelist).
		Msg("Starting exeq server")

	mux := asynq.NewServeMux()
	if privileged {
		mux.HandleFunc(ExeqCommand, handler) // handles any task which matches the prefix exec:command
		log.Info().Str("taskname", ExeqCommand).Msg("registered")

	} else {
		for _, name := range whitelist {
			taskname := ExeqCommand+":"+name
			log.Info().Str("taskname", taskname).Msg("registered")
			mux.HandleFunc(taskname, handler)
		}
	}

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

func HandleExeqCommand(ctx context.Context, t *asynq.Task, checkCommand func(string) error, echo bool) error {
	name, err := t.Payload.GetString("name")
	if err != nil {
		return err
	}
	args, err := t.Payload.GetStringSlice("args")
	if err != nil {
		return err
	}
	err = checkCommand(name)
	if err != nil {
		return err
	}
	log.Info().Str("name", name).Strs("args", args).Msg("Run command")

	var wg sync.WaitGroup
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if echo {
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

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
		// Mirror stdout/stderr to screen
		go func() {
			buf_out := bufio.NewWriter(os.Stdout)
			buf_err := bufio.NewWriter(os.Stderr)
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
					buf_out.Write(chunk_out)
					buf_out.Flush()
				case chunk_err = <-chErr:
					buf_err.Write(chunk_err)
					buf_err.Flush()

				}
				chunk_out = nil
				chunk_err = nil
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
	log.Debug().Str("name", name).Msg("complete")
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
