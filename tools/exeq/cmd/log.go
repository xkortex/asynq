// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"github.com/hibiken/asynq"
	"github.com/spf13/cobra"
	"log"
)

var logCmd = &cobra.Command{
	Use:     "log [OPTIONS] -- NAME [ARGS]",
	Short:   "Submits a log to redis",
	Long: `exeq log will submit a log to redis.
	This is mostly a convenience function to easily get data into redis from commands,
	using the existing redis config`,
	Run:  submitLog,
}

func init() {
	rootCmd.AddCommand(submitCmd)
}

func submitLog(cmd *cobra.Command, args []string) {
	uri := "localhost:6379"
	r := asynq.RedisClientOpt{Addr: uri}
	asynq.NewClient(r)
	log.Printf("submit log%s: %v\n", uri, args)
	// todo: implement
}
