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
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

// serverCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an exeq server",
	Long:  `todo`,

	Run: serve,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.PersistentFlags().BoolP("privileged", "P", false, "Whitelist all commands - MAY BE RISKY")
	rootCmd.PersistentFlags().IntP("jobs", "j", runtime.NumCPU(), "run n jobs in parallel (default value depends on your device)")

}

func serve(cmd *cobra.Command, args []string) {
	nJobs, _ := cmd.Flags().GetInt("jobs")
	privileged, _ := cmd.Flags().GetBool("privileged")
	uri := "localhost:6379"
	r := asynq.RedisClientOpt{Addr: uri}
	srv := asynq.NewServer(r, asynq.Config{
		Concurrency: nJobs,
	})
	checkCommand := func(name string) error {
		return CheckCommand(name, privileged, args)
	}
	handler := func(ctx context.Context, t *asynq.Task) error {
		return HandleExeqCommand(ctx, t, checkCommand)
	}
	log.Printf("Staring exeq server on %s with %d workers\n", uri, nJobs)
	if privileged {
		log.Println("RUNNING IN PRIVILEGED mode")
	} else {
		log.Printf("Whitelisted executables: %v \n", args)
	}
	mux := asynq.NewServeMux()
	mux.HandleFunc(ExeqCommand, handler)

	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
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

func HandleExeqCommand(ctx context.Context, t *asynq.Task, checkCommand func(string) error) error {
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
	log.Printf("Running command: `%s %v`\n", name, args)

	var wg sync.WaitGroup
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	// todo: make this quiet-able
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
					fmt.Fprintf(os.Stderr, "Error killing subprocess %d during "+
						"context cancel: %v\n", cmd.Process.Pid, err)
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

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				log.Printf("Exit Status: %d\n", status.ExitStatus())
			}
		} else {
			log.Printf("cmd.Wait error: %v\n", err)
		}
	}
	wg.Wait()
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
