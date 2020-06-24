// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pubCmd = &cobra.Command{
	Use:     "pub [OPTIONS] -- NAME [ARGS]",
	Aliases: []string{"p", "publish", "sub"},
	Short:   "Publishes a command (and args) to the queue",
	Long: `exeq pub will enqueue a command to be run by workers.

The first positional argument is the name of the executable.
Any subsequent arguments are the arguments to the command. 
If sub has any flags, they should be separated from the start of the command by --

Example: exeq sub ls /`,
	Args: cobra.MinimumNArgs(1),
	Run:  publish,
}

func init() {
	rootCmd.AddCommand(pubCmd)
}

func publish(cmd *cobra.Command, args []string) {
	uri := viper.GetString("uri")
	db := viper.GetInt("db")
	password := viper.GetString("password")

	r := asynq.RedisClientOpt{Addr: uri, DB: db, Password: password}
	client := asynq.NewClient(r)
	log.Info().Str("uri", uri).Strs("args", args).Msg("Publishing")
	t1 := NewExecCmd(args[0], args[1:])

	// Process the task immediately.
	err := client.Enqueue(t1)
	if err != nil {
		log.Fatal().Err(err).Msg("Enqueue failed")
	}
}
