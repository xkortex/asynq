# exeq CLI

`exeq` is a command line tool to submit executable tasks managed by `asynq` package.
It is designed to be a lightweight, simple, UNIX-composition-style job runner which 
can be used to run jobs across multiple hosts.

It is NOT meant to be used in production, or where more sophisticated scheduling is required.
To give you an idea, this application was born out of a desire to transcode a few thousand videos with 
`ffmpeg` on a small [Beowulf-style cluster](https://en.wikipedia.org/wiki/Beowulf_cluster),
without having to configure something like kubernetes.   

Simply configure a redis instance, then run `exeq serve -- my_command` on a host, 
then run `exeq pub -- my_command --flags args etc` to submit the job. An available
worker will pull the command from the queue and run it in its environment.

## Security

⚠️ WARNING ⚠️

The exeq server will blindly execute any jobs given to it, meaning anything that can talk to the
redis server can potentially submit arbitrary jobs. 
I make zero guarantees about the security of this application. 
It is primarily meant to be run in trusted environments inside a decent firewall. 

Limited security is provided by redis passwords, and by the whitelist functionality.
By specifying a list of allowable executables, the attack surface area is minimized.
Ideally, you'd start the exeq server with one or a handful of entry points, and
ones that are not heavily parameterized (i.e. a bash script using `${@}` is risker than 
one with finite arguments). 

## Configuration

exeq uses Cobra/Viper to read from a YAML file, environment variables, and/or CLI flags. 
By default, it looks for `~/.asynq`, so it can readily share a redis server config with `asynq`.
Run `exeq -h` or `exeq VERB -h` to see additional options.

## Server

To start the server, run `exeq serve` followed by one or more whitelisted commands. Only list the
name of the executable, e.g. `ls`, not `ls -l`. 

`exeq serve ls ffmpeg ffprobe` would start a server with the default configuration and allow running
of `ls`, `ffmpeg`, and `ffprobe`. 

Echo flag `-e/--echo` is helpful if you want to see stdout/err from the subprocess.

Specific queues can be set with `-S/--serveQueues` or in `.asynq.yml` as `serveQueues`. Queue names
are semicolon-delimited with an optional priority int, higher number has higher weight.
Default is `exeq` and default priority is 5.   

`serveQueues: exeq=5;myQueue=2;bar;spam=1`

## Publish

To publish a command, run `exeq pub -- `, followed by the command. The `--` is needed only
in the case of flags in the subcommand, to inform the shell that flag options for the `exeq` command are
finished, and only accept positional arguments after it. However, since many subcommands will have
flags, it may be a good habit to use it by default. There are no pipes currently.

`exeq pub -- ls -lah $HOME`

Commands can be published to a specific queue with `-Q/--queue` or default to the queue `exeq`.

`exeq pub -Q myQueue -- ls -lah $HOME`


## Redirection

File redirection is currently very experimental. Pipes are not implemented. 
The command syntax parser is currently very crude and will behave strangely if
it encounters `|` or `>` in unexpected ways. Redirects should be single-or-double-quoted to
avoid being parsed by your shell as redirecting the `pub` command. Only the carat needs to be quoted,
and can exist with our without a trailing space to the target file.
Redirects **must** have a space between the preceding command/argument and the redirect.
Despite these limitations, this should allow for some rudimentary logging without
having to wrap the command in a script. Also, remember the file system is relative to the
worker. You may want to redirect to NFS or similar. 

Example:
`exeq pub -- some_command '>/tmp/foo.out.log' "2>/tmp/foo.err.log"`
 
 or 
 
`exeq pub -- some_command '>' /tmp/foo.out.log '2>' /tmp/foo.err.log` 



## Interactivity

Due to the current API of `asynq` lib, there is no obvious way to get information about a published job.
This may change in the future, but for now, this means that `pub` is entirely fire-and-forget.
It is recommended to bundle your logic in a shell script if you need to do things like file redirection, 
logging, tracking, etc. 

## Caveats

This is still quite experimental and there are several dark corners. Failed commands may
get stuck in the retry queue indefinitely.

- Cannot use pipes `|`
- Cannot redirect out and error to the same file 
- Cannot redirect file descriptors e.g. '2>&1'
- Retry/error handling is very limited

