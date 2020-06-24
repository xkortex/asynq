// Copyright 2020 Michael McDermott. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package main

import "github.com/hibiken/asynq/tools/exeq/cmd"

var Version = "dev"

func main() {
	cmd.Version = Version
	cmd.Execute()
}
