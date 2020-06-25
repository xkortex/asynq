// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	ExeqCommand = "exec:command"
)

var (
	cfgFile  string
	uri      string
	db       int
	password string
	Version  = "unset"
)

func NewExecCmd(name string, args []string) *asynq.Task {
	return asynq.NewTask(ExeqCommand+":"+name, map[string]interface{}{"name": name, "args": args})
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "exeq",
	Short: "A tool for submitting executable batches to asynq queues",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.Version = Version
	rootCmd.Long = fmt.Sprintf(` exeq %s
exeq submits executable commands to queues managed by asynq.
These commands are picked up by asynq servers and run on 
available workers`, Version)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to cmd.Execute()")
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file to set flag defaut values (default is $HOME/.asynq.yaml)")
	rootCmd.PersistentFlags().StringVarP(&uri, "uri", "u", "127.0.0.1:6379", "redis server URI")
	rootCmd.PersistentFlags().IntVarP(&db, "db", "n", 0, "redis database number (default is 0)")
	rootCmd.PersistentFlags().StringVarP(&password, "password", "p", "", "password to use when connecting to redis server")
	viper.BindPFlag("uri", rootCmd.PersistentFlags().Lookup("uri"))
	viper.BindPFlag("db", rootCmd.PersistentFlags().Lookup("db"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get home directory. Skipping config file")
		} else {
			// Search config in home directory with name ".asynq" (without extension).
			viper.AddConfigPath(home)
			viper.SetConfigName(".asynq")
		}
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Info().Str("file", viper.ConfigFileUsed()).Msg("configFileUsed")
	}
}
