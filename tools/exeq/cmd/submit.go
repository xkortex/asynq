// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"github.com/hibiken/asynq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
)

var submitCmd = &cobra.Command{
	Use:     "sub [OPTIONS] -- NAME [ARGS]",
	Aliases: []string{"s", "sub", "subm"},
	Short:   "Submits a command (and args) to the queue",
	Long: `exeq submit will enqueue a command to be run by workers.

The first positional argument is the name of the executable.
Any subsequent arguments are the arguments to the command. 
If sub has any flags, they should be separated from the start of the command by --

Example: exeq sub ls /`,
	Args: cobra.MinimumNArgs(1),
	Run:  submit,
}

func init() {
	rootCmd.AddCommand(submitCmd)
}

func submit(cmd *cobra.Command, args []string) {
	uri := viper.GetString("uri")
	db := viper.GetInt("db")
	password := viper.GetString("password")

	r := asynq.RedisClientOpt{Addr: uri, DB: db, Password: password}
	client := asynq.NewClient(r)
	log.Printf("Submitting to %s: %v\n", uri, args)
	t1 := NewExecCmd(args[0], args[1:])

	// Process the task immediately.
	err := client.Enqueue(t1)
	if err != nil {
		log.Fatal(err)
	}
}
