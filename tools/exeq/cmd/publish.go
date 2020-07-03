// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"github.com/go-redis/redis/v7"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynq/internal/rdb"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"regexp"
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
	pubCmd.PersistentFlags().StringP("queue", "Q", "exeq", "Publish to a specific queue (default queue is exeq)")
	pubCmd.PersistentFlags().StringP("broadcast", "B", "", "Publish a job to all currently listening servers (hostname matching pattern)")
	viper.BindPFlag("queue", pubCmd.PersistentFlags().Lookup("queue"))
	rootCmd.AddCommand(pubCmd)

}

// publish one command
func publishOne(client *asynq.Client, ecmd *ExeqCommand, queue string) (err error) {
	log.Info().Str("uri", uri).Str("name", ecmd.Name).Strs("args", ecmd.Args).Str("queue", queue).
		Str("stdout", ecmd.StdoutFile).Str("stderr", ecmd.StderrFile).Msg("Publishing")
	t1 := NewExecCmd(*ecmd)

	// Process the task immediately.
	err = client.Enqueue(t1, asynq.Queue(queue))
	if err != nil {
		return err
	}
	return nil
}

func publish(cmd *cobra.Command, args []string) {
	uri := viper.GetString("uri")
	db := viper.GetInt("db")
	password := viper.GetString("password")
	queue := viper.GetString("queue")
	var err error

	log.Debug().Strs("args", args).Msg("Raw input")
	ecmd, err := ParseExeqCommand(args[0], args[1:])
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse input command")
	}

	rOpt := asynq.RedisClientOpt{Addr: uri, DB: db, Password: password}
	client := asynq.NewClient(rOpt)

	if broadcast, err := cmd.Flags().GetString("broadcast"); err != nil {
		log.Error().Err(err).Str("key", "broadcast").Msg("Failed to parse CLI value")
	} else if broadcast != "" {
		rpat, err := regexp.Compile(broadcast)
		if err != nil {
			log.Fatal().Err(err).Str("broadcast", broadcast).Msg("Failed to compile `broadcast` as regex")
		}
		r := rdb.NewRDB(redis.NewClient(&redis.Options{
			Addr:     uri,
			DB:       db,
			Password: password,
		}))
		servers, err := r.ListServers()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get list of servers")
		}
		targetHosts := make([]string, 0, len(servers))
		for _, server := range servers {
			if s := rpat.FindStringIndex(server.Host); s != nil {
				targetHosts = append(targetHosts, server.Host)
			}
		}
		if len(targetHosts) == 0 {
			log.Fatal().Str("broadcast", broadcast).Msg("No server host names matched broadcast regex")
		}
		for _, host := range targetHosts {
			queue = "exeq+" + host
			err = publishOne(client, &ecmd, queue)
			if err != nil {
				// Can't really treat this as fatal since it may have already kicked off some tasks
				log.Error().Err(err).Str("host", host).Msg("Enqueue failed during broadcast")
			}
		}

	} else {
		err = publishOne(client, &ecmd, queue)
		if err != nil {
			log.Fatal().Err(err).Msg("Enqueue failed")
		}
	}

}
